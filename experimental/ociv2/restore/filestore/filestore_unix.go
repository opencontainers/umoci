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

package filestore

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

type store struct {
	root    string
	tmp     string
	tmpFile *os.File
}

const (
	inodeDir    = "blobs"
	tmpInodeDir = ".tmp"
)

func blobPath(digest digest.Digest) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", errors.Wrapf(err, "invalid digest: %q", digest)
	}

	alg := digest.Algorithm().String()
	hex := digest.Encoded()

	// Don't make the same mistake as OCI -- shard the blob store!
	hex1, hex2, hex3 := hex[:2], hex[2:4], hex[4:6]
	return filepath.Join(inodeDir, alg, hex1, hex2, hex3, hex), nil
}

func (s *store) ensureTempDir() error {
	if s.tmp == "" {
		tmpDir, err := ioutil.TempDir(s.root, ".umoci2-")
		if err != nil {
			return errors.Wrap(err, "create tmpdir")
		}
		s.tmp = tmpDir
	}
	return nil
}

// Put inserts the given data as a new file-inode into the store.
func (s *store) Put(ctx context.Context, reader io.Reader) (digest.Digest, int64, error) {
	if err := s.ensureTempDir(); err != nil {
		return "", -1, errors.Wrap(err, "ensure tmpdir")
	}

	digester := StoreAlgorithm.Digester()

	// We copy this into a temporary file because we need to get the blob hash,
	// but also to avoid half-writing an invalid blob.
	fh, err := ioutil.TempFile(s.tmp, "blob-")
	if err != nil {
		return "", -1, errors.Wrap(err, "create temporary blob")
	}
	tmpPath := fh.Name()
	defer fh.Close()

	writer := io.MultiWriter(fh, digester.Hash())
	size, err := io.Copy(writer, reader)
	if err != nil {
		return "", -1, errors.Wrap(err, "copy to temporary blob")
	}
	if err := fh.Close(); err != nil {
		return "", -1, errors.Wrap(err, "close temporary blob")
	}

	// Get the digest.
	path, err := blobPath(digester.Digest())
	if err != nil {
		return "", -1, errors.Wrap(err, "compute blob name")
	}
	path = filepath.Join(s.root, path)

	// Move the blob to its correct path.
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", -1, errors.Wrapf(err, "mkdir parent shardpath %s", path)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return "", -1, errors.Wrap(err, "rename temporary blob")
	}

	return digester.Digest(), int64(size), nil
}

// Get a read-only handle to a file-inode inside the store. If there is no
// file-inode in the store matching the contents, (nil, nil) is returned.
func (s *store) Get(ctx context.Context, digest digest.Digest) (*os.File, error) {
	path, err := blobPath(digest)
	if err != nil {
		return nil, errors.Wrap(err, "compute blob path")
	}
	fh, err := os.Open(filepath.Join(s.root, path))
	if os.IsNotExist(err) {
		err = nil
	}
	return fh, err
}

// Close and clean up the handle.
func (s *store) Close() error {
	var err1, err2 error
	if s.tmpFile != nil {
		err1 = s.tmpFile.Close()
	}
	if s.tmp != "" {
		err2 = os.RemoveAll(s.tmp)
	}

	s.tmp = ""
	s.tmpFile = nil

	if err2 != nil {
		return err2
	}
	if err1 != nil {
		return err1
	}
	return nil
}
