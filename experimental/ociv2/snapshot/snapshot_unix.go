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

package snapshot

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/openSUSE/umoci/experimental/ociv2/spec/v2"
	"github.com/opencontainers/image-spec/specs-go/v1"

	// We need to include sha256 in order for go-digest to properly handle such
	// hashes, since Go's crypto library like to lazy-load cryptographic
	// libraries.
	_ "crypto/sha256"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"github.com/restic/chunker"
	"golang.org/x/sys/unix"
)

func toBaseInode(st unix.Stat_t) (v2.Inode, error) {
	var inodeType v2.InodeType
	switch st.Mode & unix.S_IFMT {
	case unix.S_IFREG:
		inodeType = v2.InodeTypeFile
	case unix.S_IFDIR:
		inodeType = v2.InodeTypeDirectory
	case unix.S_IFLNK:
		inodeType = v2.InodeTypeSymlink
	case unix.S_IFBLK:
		inodeType = v2.InodeTypeBlockDevice
	case unix.S_IFCHR:
		inodeType = v2.InodeTypeCharDevice
	case unix.S_IFIFO:
		inodeType = v2.InodeTypeNamedPipe
	case unix.S_IFSOCK:
		inodeType = v2.InodeTypeSocket
	default:
		return v2.Inode{}, errors.Errorf("unknown st.Mode: 0x%x", st.Mode&unix.S_IFMT)
	}

	// Handle mode for symlinks.
	mode := st.Mode &^ unix.S_IFMT
	modePtr := &mode
	if st.Mode&unix.S_IFMT == unix.S_IFLNK {
		modePtr = nil
	}

	return v2.Inode{
		Type: inodeType,
		Meta: v2.InodeMeta{
			UID:  st.Uid,
			GID:  st.Gid,
			Mode: modePtr,
			/*
				AccessTime: v2.Timespec{
					Sec:  st.Atim.Sec,
					Nsec: uint32(st.Atim.Nsec),
				},
				ChangeTime: v2.Timespec{
					Sec:  st.Ctim.Sec,
					Nsec: uint32(st.Ctim.Nsec),
				},
				ModifyTime: v2.Timespec{
					Sec:  st.Mtim.Sec,
					Nsec: uint32(st.Mtim.Nsec),
				},
			*/
		},
	}, nil
}

func Snapshot(ctx context.Context, engine casext.Engine, rootPath string) (*v1.Descriptor, error) {
	fsEval := fseval.DefaultFsEval

	fi, err := fsEval.Lstat(rootPath)
	if err != nil {
		return nil, errors.Wrap(err, "snapshot: lstat root")
	}
	if !fi.IsDir() {
		return nil, errors.Errorf("snapshot: root is not a directory")
	}

	root := v2.Root{
		Inodes: map[string]v2.Inode{},
	}

	err = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "walking into %s", path)
		}
		relPath, err := filepath.Rel(rootPath, path)
		if err != nil {
			return errors.Wrapf(err, "finding relpath of %s (root is %s)", path, rootPath)
		}

		st, err := fsEval.Lstatx(path)
		if err != nil {
			return errors.Wrapf(err, "snapshot path")
		}

		ino, err := toBaseInode(st)
		if err != nil {
			return errors.Wrap(err, "convert stat to base-inode")
		}

		switch ino.Type {
		case v2.InodeTypeFile:
			wholeDigest, chunks, err := chunkFile(ctx, engine, path)
			if err != nil {
				return errors.Wrapf(err, "snap-file: chunks")
			}
			ino.InlineData = map[string]string{
				"digest": wholeDigest.Encoded(),
			}
			ino.IndirectData = chunks
		case v2.InodeTypeCharDevice, v2.InodeTypeBlockDevice:
			ino.InlineData = map[string]string{
				"major": string(unix.Major(st.Rdev)),
				"minor": string(unix.Minor(st.Rdev)),
			}
		case v2.InodeTypeSymlink:
			target, err := fsEval.Readlink(path)
			if err != nil {
				return errors.Wrapf(err, "snap-symlink: readlink")
			}
			ino.InlineData = map[string]string{
				"target": target,
			}
		}

		if _, ok := root.Inodes[relPath]; ok {
			return errors.Errorf("snapshot: hit duplicate path in walk: %s", relPath)
		}
		root.Inodes[relPath] = ino
		return nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "snapshot: walk root")
	}

	digest, size, err := engine.PutBlobJSON(ctx, root)
	if err != nil {
		return nil, errors.Wrapf(err, "snapshot: put root blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeRoot,
		Digest:    digest,
		Size:      size,
	}, nil
}

// TODO: Switch this to be stored somewhere else.
const wholeChunkAlgorithm = digest.SHA256

func chunkFile(ctx context.Context, engine casext.Engine, path string) (digest.Digest, []v1.Descriptor, error) {
	fh, err := os.Open(path)
	if err != nil {
		return "", nil, errors.Wrap(err, "snap-file: open")
	}
	defer fh.Close()

	// Digest the file while chunking it.
	wholeDigester := wholeChunkAlgorithm.Digester()
	rdr := io.TeeReader(fh, wholeDigester.Hash())
	chunker := chunker.NewWithBoundaries(rdr, v2.ChunkPolynomial, v2.ChunkMin, v2.ChunkMax)

	var chunks []v1.Descriptor
	for {
		chunk, err := chunker.Next(nil)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, errors.Wrap(err, "snap-file: next chunk")
		}
		buffer := bytes.NewBuffer(chunk.Data)

		digest, size, err := engine.PutBlob(ctx, buffer)
		if err != nil {
			return "", nil, errors.Wrap(err, "snap-file: put chunk blob")
		}
		if size != int64(chunk.Length) {
			return "", nil, errors.Errorf("chunk size doesn't match stored size")
		}

		chunks = append(chunks, v1.Descriptor{
			MediaType: v2.MediaTypeChunk,
			Size:      size,
			Digest:    digest,
		})
	}
	return wholeDigester.Digest(), chunks, nil
}
