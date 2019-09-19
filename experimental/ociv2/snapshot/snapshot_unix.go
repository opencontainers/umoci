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

type snapshotter struct {
	fsEval fseval.FsEval
	engine casext.Engine
}

func Snapshot(ctx context.Context, engine casext.Engine, root string) (*v1.Descriptor, error) {
	s := &snapshotter{
		engine: engine,
		fsEval: fseval.DefaultFsEval,
	}

	fi, err := s.fsEval.Lstat(root)
	if err != nil {
		return nil, errors.Wrap(err, "snapshot: lstat root")
	}
	if !fi.IsDir() {
		return nil, errors.Errorf("snapshot: root is not a directory")
	}

	return s.directory(ctx, root)
}

func toBasic(st unix.Stat_t) v2.BasicInode {
	mode := st.Mode
	basic := v2.BasicInode{
		Meta: v2.InodeMeta{
			UID:  st.Uid,
			GID:  st.Gid,
			Mode: &mode,
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
	}
	if os.FileMode(mode)&os.ModeSymlink == os.ModeSymlink {
		basic.Meta.Mode = nil
	}
	return basic
}

// TODO: Switch this to be stored somewhere else.
const wholeChunkAlgorithm = digest.SHA256

func (s *snapshotter) file(ctx context.Context, path string) (*v1.Descriptor, error) {
	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrap(err, "snap-file: lstat")
	}

	fh, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "snap-file: open")
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
			return nil, errors.Wrap(err, "snap-file: next chunk")
		}
		buffer := bytes.NewBuffer(chunk.Data)

		digest, size, err := s.engine.PutBlob(ctx, buffer)
		if err != nil {
			return nil, errors.Wrap(err, "snap-file: put chunk blob")
		}
		if size != int64(chunk.Length) {
			return nil, errors.Errorf("chunk size doesn't match stored size")
		}

		chunks = append(chunks, v1.Descriptor{
			MediaType: v2.MediaTypeChunk,
			Size:      size,
			Digest:    digest,
		})
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.FileInode{
		BasicInode: toBasic(st),
		Digest:     wholeDigester.Digest(),
		Chunks:     chunks,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-file: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeFile,
		Digest:    digest,
		Size:      size,
	}, nil
}

func (s *snapshotter) directory(ctx context.Context, path string) (*v1.Descriptor, error) {
	fis, err := s.fsEval.Readdir(path)
	if err != nil {
		return nil, errors.Wrap(err, "snap-dir: readdir")
	}

	childMap := map[string]v1.Descriptor{}
	for _, fi := range fis {
		name := fi.Name()
		full := filepath.Join(path, name)

		if name == "." || name == ".." {
			continue
		}

		var childDesc *v1.Descriptor
		fimode := fi.Mode() & os.ModeType
		switch {
		case fimode&os.ModeDir == os.ModeDir:
			childDesc, err = s.directory(ctx, full)
		case fimode&os.ModeSymlink == os.ModeSymlink:
			childDesc, err = s.symlink(ctx, full)
		case fimode&os.ModeDevice == os.ModeDevice:
			childDesc, err = s.device(ctx, full)
		case fimode&os.ModeNamedPipe == os.ModeNamedPipe:
			childDesc, err = s.fifo(ctx, full)
		case fimode&os.ModeSocket == os.ModeSocket:
			childDesc, err = s.socket(ctx, full)
		case fimode == 0:
			childDesc, err = s.file(ctx, full)
		default:
			err = errors.Errorf("unknown file mode: %x", fi.Mode()&os.ModeType)
		}
		if err != nil {
			return nil, errors.Wrapf(err, "snap-dir: %q", fi.Name())
		}
		childMap[name] = *childDesc
	}

	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-dir: lstat")
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.DirectoryInode{
		BasicInode: toBasic(st),
		Children:   childMap,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-dir: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeDirectory,
		Digest:    digest,
		Size:      size,
	}, nil
}

func (s *snapshotter) symlink(ctx context.Context, path string) (*v1.Descriptor, error) {
	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-symlink: lstat")
	}

	target, err := s.fsEval.Readlink(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-symlink: readlink")
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.SymlinkInode{
		BasicInode: toBasic(st),
		Target:     target,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-symlink: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeSymlink,
		Digest:    digest,
		Size:      size,
	}, nil
	return nil, nil
}

func (s *snapshotter) device(ctx context.Context, path string) (*v1.Descriptor, error) {
	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-fifo: lstat")
	}

	deviceType := v2.BlockDevice
	if os.FileMode(st.Mode)&os.ModeCharDevice == os.ModeCharDevice {
		deviceType = v2.CharDevice
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.DeviceInode{
		BasicInode: toBasic(st),
		Type:       deviceType,
		Major:      unix.Major(st.Rdev),
		Minor:      unix.Minor(st.Rdev),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-fifo: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeDevice,
		Digest:    digest,
		Size:      size,
	}, nil
}

func (s *snapshotter) fifo(ctx context.Context, path string) (*v1.Descriptor, error) {
	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-fifo: lstat")
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.NamedPipeInode{
		BasicInode: toBasic(st),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-fifo: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeNamedPipe,
		Digest:    digest,
		Size:      size,
	}, nil
}

func (s *snapshotter) socket(ctx context.Context, path string) (*v1.Descriptor, error) {
	st, err := s.fsEval.Lstatx(path)
	if err != nil {
		return nil, errors.Wrapf(err, "snap-socket: lstat")
	}

	digest, size, err := s.engine.PutBlobJSON(ctx, v2.SocketInode{
		BasicInode: toBasic(st),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "snap-socket: put blob")
	}

	return &v1.Descriptor{
		MediaType: v2.MediaTypeInodeSocket,
		Digest:    digest,
		Size:      size,
	}, nil
}
