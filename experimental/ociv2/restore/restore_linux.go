/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2019 SUSE LLC.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// TODO: All of this needs to be reworked to be lookup-safe.

package restore

import (
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/openSUSE/umoci/experimental/ociv2/restore/filestore"
	"github.com/openSUSE/umoci/experimental/ociv2/spec/v2"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

type Restorer struct {
	Engine casext.Engine
	Store  filestore.Store
}

type stitchedReader struct {
	idx   int
	parts []io.ReadCloser
}

func (r *stitchedReader) Read(p []byte) (int, error) {
	if r.idx >= len(r.parts) {
		return 0, io.EOF
	}

	part := r.parts[r.idx]
	n, err := part.Read(p)
	if err == io.EOF {
		// Move on to the next blob.
		if err := part.Close(); err != nil {
			return -1, err
		}
		r.idx++
		if r.idx < len(r.parts) {
			err = nil
		}
	}
	return n, err
}

func (r *stitchedReader) Close() error {
	var err error
	for i := r.idx; i < len(r.parts); i++ {
		err2 := r.parts[i].Close()
		if err == nil {
			err = err2
		}
	}
	return err
}

func (r *Restorer) stitch(ctx context.Context, chunks []v1.Descriptor) (io.ReadCloser, int64, error) {
	var totalSize int64
	stitched := &stitchedReader{}
	for idx, chunk := range chunks {
		chunkRdr, err := r.Engine.GetVerifiedBlob(ctx, chunk)
		if err != nil {
			return nil, -1, errors.Wrapf(err, "get chunk %d", idx)
		}
		totalSize += chunk.Size
		stitched.parts = append(stitched.parts, chunkRdr)
	}
	return stitched, totalSize, nil
}

func (r *Restorer) reflinkFile(dst *os.File, src *os.File) error {
	const FiClone = 0x40049409
	err := unix.IoctlSetInt(int(dst.Fd()), FiClone, int(src.Fd()))
	return errors.Wrapf(err, "ioctl FICLONE")
}

func (r *Restorer) copyFile(dst io.Writer, src io.Reader) error {
	_, err := io.Copy(dst, src)
	return err
}

func applyMetadata(path string, meta v2.InodeMeta) error {
	if err := unix.Lchown(path, int(meta.UID), int(meta.GID)); err != nil {
		return &os.PathError{Op: "lchown", Path: path, Err: err}
	}
	if meta.Mode != nil {
		if err := unix.Chmod(path, *meta.Mode); err != nil {
			return &os.PathError{Op: "chmod", Path: path, Err: err}
		}
	}
	utimes := []unix.Timespec{
		{Sec: meta.AccessTime.Sec, Nsec: int64(meta.AccessTime.Nsec)},
		{Sec: meta.ModifyTime.Sec, Nsec: int64(meta.ModifyTime.Nsec)},
	}
	if err := unix.UtimesNanoAt(unix.AT_FDCWD, path, utimes, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return &os.PathError{Op: "utimensat", Path: path, Err: err}
	}
	// TODO: the rest
	return nil
}

func (r *Restorer) Restore(ctx context.Context, descriptor v1.Descriptor, rootPath string) error {
	rootBlob, err := r.Engine.FromDescriptor(ctx, descriptor)
	if err != nil {
		return errors.Wrap(err, "get root blob")
	}
	if rootBlob.Descriptor.MediaType != v2.MediaTypeRoot {
		return errors.Errorf("invalid mediatype for root: %s", rootBlob.Descriptor.MediaType)
	}
	root, ok := rootBlob.Data.(v2.Root)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown root blob type: %s", rootBlob.Descriptor.MediaType)
	}

	// Apply all inodes to the target system.
	directories := map[string]v2.Inode{}
	for path, ino := range root.Inodes {
		// Store away directories so the metadata can be re-applied as a
		// post-processing step (to make it easier to get reproducible
		// metadata).
		if ino.Type == v2.InodeTypeDirectory {
			directories[path] = ino
		}
		fullPath := filepath.Join(rootPath, path)
		parentPath := filepath.Dir(fullPath)

		if err := os.MkdirAll(parentPath, 0700); err != nil {
			return errors.Wrapf(err, "mkdirall parent path %s", parentPath)
		}

		switch ino.Type {
		case v2.InodeTypeFile:
			wholeDigestStr, ok := ino.InlineData["digest"]
			if !ok {
				return errors.Errorf("restore-file: required inline data 'digest' missing")
			}
			wholeDigest := digest.Digest(wholeDigestStr)
			if err := wholeDigest.Validate(); err != nil {
				return errors.Wrapf(err, "restore-file: invalid digest: %s", wholeDigestStr)
			}

			newFh, err := os.Create(fullPath)
			if err != nil {
				return errors.Wrap(err, "restore-file: create target")
			}
			defer newFh.Close()

			var old io.ReadCloser
			defer func() {
				if old != nil {
					old.Close()
				}
			}()
			if r.Store != nil {
				oldFh, err := r.Store.Get(ctx, wholeDigest)
				if err != nil {
					return errors.Wrap(err, "restore-file: get file-inode from store")
				}
				if oldFh == nil {
					whole, wholeSize, err := r.stitch(ctx, ino.IndirectData)
					if err != nil {
						return errors.Wrap(err, "restore-file: stitch parts")
					}

					actualDigest, actualSize, err := r.Store.Put(ctx, whole)
					if err != nil {
						return errors.Wrap(err, "restore-file: put stitched file")
					}
					if actualSize != wholeSize {
						return errors.Errorf("restore-file: wrong size (%d != %d) when putting into store", actualSize, wholeSize)
					}
					if actualDigest != wholeDigest {
						return errors.Errorf("restore-file: wrong digest (%s != %s) when putting into store", actualDigest, wholeDigest)
					}

					oldFh, err = r.Store.Get(ctx, wholeDigest)
					if err != nil {
						return errors.Wrap(err, "restore-file: get file-inode from store")
					}
					if oldFh == nil {
						return errors.Errorf("[internal error] failed to get file-inode after put")
					}
				}
				old = oldFh
			} else {
				whole, _, err := r.stitch(ctx, ino.IndirectData)
				if err != nil {
					return errors.Errorf("restore-file: stitch parts")
				}
				old = whole
			}

			done := false
			if oldFh, ok := old.(*os.File); ok {
				if err := r.reflinkFile(newFh, oldFh); err == nil {
					done = true
				}
			}
			if !done {
				if err := r.copyFile(newFh, old); err != nil {
					return errors.Wrap(err, "restore-file: copy file")
				}
			}

			newFh.Close()
			old.Close()

		case v2.InodeTypeDirectory:
			if err := os.MkdirAll(fullPath, 0700); err != nil {
				return errors.Wrapf(err, "restore-dir")
			}
		case v2.InodeTypeSymlink:
			target, ok := ino.InlineData["target"]
			if !ok {
				return errors.Errorf("restore-symlink: required inline data 'target' missing")
			}
			if err := unix.Symlink(target, fullPath); err != nil {
				return errors.Wrap(err, "restore-symlink")
			}
		case v2.InodeTypeCharDevice, v2.InodeTypeBlockDevice:
			majorStr, ok := ino.InlineData["major"]
			if !ok {
				return errors.Errorf("restore-device: required inline data 'major' missing")
			}
			major, err := strconv.ParseUint(majorStr, 10, 32)
			if err != nil {
				return errors.Wrapf(err, "restore-device: invalid major: %s", majorStr)
			}

			minorStr, ok := ino.InlineData["minor"]
			if !ok {
				return errors.Errorf("restore-device: required inline data 'minor' missing")
			}
			minor, err := strconv.ParseUint(minorStr, 10, 32)
			if err != nil {
				return errors.Wrapf(err, "restore-device: invalid minor: %s", minorStr)
			}

			var mode uint32
			switch ino.Type {
			case v2.InodeTypeCharDevice:
				mode |= unix.S_IFCHR
			case v2.InodeTypeBlockDevice:
				mode |= unix.S_IFBLK
			}
			dev := unix.Mkdev(uint32(major), uint32(minor))
			if err := unix.Mknod(path, mode, int(dev)); err != nil {
				return errors.Wrap(err, "restore-device")
			}
		case v2.InodeTypeNamedPipe:
			if err := unix.Mknod(path, unix.S_IFIFO, 0); err != nil {
				return errors.Wrap(err, "restore-fifo")
			}
		case v2.InodeTypeSocket:
			if err := unix.Mknod(path, unix.S_IFSOCK, 0); err != nil {
				return errors.Wrap(err, "restore-socket")
			}
		}
		if err := applyMetadata(fullPath, ino.Meta); err != nil {
			return errors.Wrapf(err, "apply metadata for %s", fullPath)
		}
	}
	// Re-apply directory metadata after all inodes are in place.
	for path, ino := range directories {
		fullPath := filepath.Join(rootPath, path)
		if err := applyMetadata(fullPath, ino.Meta); err != nil {
			return errors.Wrapf(err, "apply metadata for %s", fullPath)
		}
	}
	return nil
}
