// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
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

// Package dir implements the basic local directory-backed layout backend for
// the OCI content-addressible store.
package dir

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/pkg/hardening"
)

const (
	// ImageLayoutVersion is the version of the image layout we support. This
	// value is *not* the same as imagespec.Version, and the meaning of this
	// field is still under discussion in the spec. For now we'll just hardcode
	// the value and hope for the best.
	ImageLayoutVersion = ispec.ImageLayoutVersion // "1.0.0"

	// blobDirectory is the directory inside an OCI image that contains blobs.
	blobDirectory = ispec.ImageBlobsDir // "blobs"

	// indexFile is the file inside an OCI image that contains the top-level
	// index.
	indexFile = ispec.ImageIndexFile // "index.json"

	// layoutFile is the file in side an OCI image the indicates what version
	// of the OCI spec the image is.
	layoutFile = ispec.ImageLayoutFile // "oci-layout"
)

// blobPath returns the path to a blob given its digest, relative to the root
// of the OCI image. The digest must be of the form algorithm:hex.
func blobPath(digest digest.Digest) (string, error) {
	if err := digest.Validate(); err != nil {
		return "", fmt.Errorf("invalid digest: %q: %w", digest, err)
	}

	algo := digest.Algorithm()
	hash := digest.Hex()

	if algo != cas.BlobAlgorithm {
		return "", fmt.Errorf("unsupported algorithm: %q", algo)
	}

	return filepath.Join(blobDirectory, algo.String(), hash), nil
}

type uidgid struct {
	uid, gid int
}

type dirEngine struct {
	path     string
	temp     string
	tempFile *os.File
	owner    *uidgid
}

func (e *dirEngine) ensureTempDir() error {
	if e.temp == "" {
		tempDir, err := os.MkdirTemp(e.path, ".umoci-")
		if err != nil {
			return fmt.Errorf("create tempdir: %w", err)
		}

		// We get an advisory lock to ensure that GC() won't delete our
		// temporary directory here. Once we get the lock we know it won't do
		// anything until we unlock it or exit.

		e.tempFile, err = os.Open(tempDir)
		if err != nil {
			return fmt.Errorf("open tempdir for lock: %w", err)
		}
		if err := unix.Flock(int(e.tempFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return fmt.Errorf("lock tempdir: %w", err)
		}

		e.temp = tempDir
	}
	return nil
}

// chown changes the ownership of the provided file to match the owner of the
// cas directory itself (in order to avoid a root "umoci repack" creating
// inaccessible files for the original user).
func (e *dirEngine) fchown(file *os.File) error {
	if os.Geteuid() != 0 {
		return nil // skip chown if not running as root
	}
	if e.owner == nil {
		var stat unix.Stat_t
		if err := unix.Stat(e.path, &stat); err != nil {
			return fmt.Errorf("stat cas dir: %w", err)
		}
		e.owner = &uidgid{
			uid: int(stat.Uid),
			gid: int(stat.Gid),
		}
	}
	return file.Chown(e.owner.uid, e.owner.gid)
}

// verify ensures that the image is valid.
func (e *dirEngine) validate() error {
	content, err := os.ReadFile(filepath.Join(e.path, layoutFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = cas.ErrInvalid
		}
		return fmt.Errorf("read oci-layout: %w", err)
	}

	var ociLayout ispec.ImageLayout
	if err := json.Unmarshal(content, &ociLayout); err != nil {
		return fmt.Errorf("parse oci-layout: %w", err)
	}

	// XXX: Currently the meaning of this field is not adequately defined by
	//      the spec, nor is the "official" value determined by the spec.
	if ociLayout.Version != ImageLayoutVersion {
		return fmt.Errorf("layout version is not supported: %w", cas.ErrInvalid)
	}

	// Check that "blobs" and "index.json" exist in the image.
	// FIXME: We also should check that blobs *only* contains a cas.BlobAlgorithm
	//        directory (with no subdirectories) and that refs *only* contains
	//        files (optionally also making sure they're all JSON descriptors).
	if fi, err := os.Stat(filepath.Join(e.path, blobDirectory)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = cas.ErrInvalid
		}
		return fmt.Errorf("check blobdir: %w", err)
	} else if !fi.IsDir() {
		return fmt.Errorf("blobdir is not a directory: %w", cas.ErrInvalid)
	}

	if fi, err := os.Stat(filepath.Join(e.path, indexFile)); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = cas.ErrInvalid
		}
		return fmt.Errorf("check index: %w", err)
	} else if fi.IsDir() {
		return fmt.Errorf("index is a directory: %w", cas.ErrInvalid)
	}

	return nil
}

// PutBlob adds a new blob to the image. This is idempotent; a nil error
// means that "the content is stored at DIGEST" without implying "because
// of this PutBlob() call".
func (e *dirEngine) PutBlob(_ context.Context, reader io.Reader) (_ digest.Digest, _ int64, Err error) {
	if err := e.ensureTempDir(); err != nil {
		return "", -1, fmt.Errorf("ensure tempdir: %w", err)
	}

	digester := cas.BlobAlgorithm.Digester()

	// We copy this into a temporary file because we need to get the blob hash,
	// but also to avoid half-writing an invalid blob.
	fh, err := os.CreateTemp(e.temp, "blob-")
	if err != nil {
		return "", -1, fmt.Errorf("create temporary blob: %w", err)
	}
	tempPath := fh.Name()
	defer funchelpers.VerifyClose(&Err, fh)

	writer := io.MultiWriter(fh, digester.Hash())
	size, err := system.Copy(writer, reader)
	if err != nil {
		return "", -1, fmt.Errorf("copy to temporary blob: %w", err)
	}
	if err := e.fchown(fh); err != nil {
		return "", -1, fmt.Errorf("fix ownership of new blob: %w", err)
	}
	if err := fh.Sync(); err != nil {
		return "", -1, fmt.Errorf("sync temporary blob: %w", err)
	}

	// Get the digest.
	path, err := blobPath(digester.Digest())
	if err != nil {
		return "", -1, fmt.Errorf("compute blob name: %w", err)
	}

	// Move the blob to its correct path.
	path = filepath.Join(e.path, path)
	if err := os.Rename(tempPath, path); err != nil {
		return "", -1, fmt.Errorf("rename temporary blob: %w", err)
	}

	return digester.Digest(), size, nil
}

// GetBlob returns a reader for retrieving a blob from the image, which the
// caller must Close(). Returns ErrNotExist if the digest is not found.
//
// This function will return a VerifiedReadCloser, meaning that you must call
// Close() and check the error returned from Close() in order to ensure that
// the hash of the blob is verified.
//
// Please note that calling Close() on the returned blob will read the entire
// from disk and hash it (even if you didn't read any bytes before calling
// Close), so if you wish to only check if a blob exists you should use
// StatBlob() instead.
func (e *dirEngine) GetBlob(_ context.Context, digest digest.Digest) (io.ReadCloser, error) {
	path, err := blobPath(digest)
	if err != nil {
		return nil, fmt.Errorf("compute blob path: %w", err)
	}
	fh, err := os.Open(filepath.Join(e.path, path))
	if err != nil {
		return nil, fmt.Errorf("open blob: %w", err)
	}
	st, err := fh.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat blob: %w", err)
	}
	return &hardening.VerifiedReadCloser{
		Reader:         fh,
		ExpectedDigest: digest,
		// Assume the file size is the blob size. This is almost certainly true
		// in general, and if an attacker is modifying the blobs underneath us
		// then snapshotting the size makes sure we don't read endlessly.
		ExpectedSize: st.Size(),
	}, nil
}

// StatBlob returns whether the specified blob exists in the image. Returns
// false if the blob doesn't exist, true if it does, or an error if any error
// occurred.
//
// NOTE: In future this API may change to return additional information.
func (e *dirEngine) StatBlob(_ context.Context, digest digest.Digest) (bool, error) {
	path, err := blobPath(digest)
	if err != nil {
		return false, fmt.Errorf("compute blob path: %w", err)
	}
	_, err = os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("stat blob path: %w", err)
	}
	return true, nil
}

// PutIndex sets the index of the OCI image to the given index, replacing the
// previously existing index. This operation is atomic; any readers attempting
// to access the OCI image while it is being modified will only ever see the
// new or old index.
func (e *dirEngine) PutIndex(_ context.Context, index ispec.Index) (Err error) {
	if err := e.ensureTempDir(); err != nil {
		return fmt.Errorf("ensure tempdir: %w", err)
	}

	// Make sure the index has the mediatype field set.
	index.MediaType = ispec.MediaTypeImageIndex

	// We copy this into a temporary index to ensure the atomicity of this
	// operation.
	fh, err := os.CreateTemp(e.temp, "index-")
	if err != nil {
		return fmt.Errorf("create temporary index: %w", err)
	}
	tempPath := fh.Name()
	defer funchelpers.VerifyClose(&Err, fh)

	// Encode the index.
	if err := json.NewEncoder(fh).Encode(index); err != nil {
		return fmt.Errorf("write temporary index: %w", err)
	}
	if err := e.fchown(fh); err != nil {
		return fmt.Errorf("fix ownership of new index: %w", err)
	}
	if err := fh.Sync(); err != nil {
		return fmt.Errorf("sync temporary index: %w", err)
	}

	// Move the blob to its correct path.
	path := filepath.Join(e.path, indexFile)
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename temporary index: %w", err)
	}
	return nil
}

// GetIndex returns the index of the OCI image. Return ErrNotExist if the
// digest is not found. If the image doesn't have an index, ErrInvalid is
// returned (a valid OCI image MUST have an image index).
//
// It is not recommended that users of cas.Engine use this interface directly,
// due to the complication of properly handling references as well as correctly
// handling nested indexes. casext.Engine provides a wrapper for cas.Engine
// that implements various reference resolution functions that should work for
// most users.
func (e *dirEngine) GetIndex(_ context.Context) (ispec.Index, error) {
	content, err := os.ReadFile(filepath.Join(e.path, indexFile))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			err = cas.ErrInvalid
		}
		return ispec.Index{}, fmt.Errorf("read index: %w", err)
	}

	var index ispec.Index
	if err := json.Unmarshal(content, &index); err != nil {
		return ispec.Index{}, fmt.Errorf("parse index: %w", err)
	}

	return index, nil
}

// DeleteBlob removes a blob from the image. This is idempotent; a nil
// error means "the content is not in the store" without implying "because
// of this DeleteBlob() call".
func (e *dirEngine) DeleteBlob(_ context.Context, digest digest.Digest) error {
	path, err := blobPath(digest)
	if err != nil {
		return fmt.Errorf("compute blob path: %w", err)
	}

	err = os.Remove(filepath.Join(e.path, path))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove blob: %w", err)
	}
	return nil
}

// ListBlobs returns the set of blob digests stored in the image.
func (e *dirEngine) ListBlobs(_ context.Context) ([]digest.Digest, error) {
	digests := []digest.Digest{}
	blobDir := filepath.Join(e.path, blobDirectory, cas.BlobAlgorithm.String())

	if err := filepath.Walk(blobDir, func(path string, _ os.FileInfo, _ error) error {
		// Skip the actual directory.
		if path == blobDir {
			return nil
		}
		// XXX: Do we need to handle multiple-directory-deep cases?
		digest := digest.NewDigestFromHex(cas.BlobAlgorithm.String(), filepath.Base(path))
		digests = append(digests, digest)
		return nil
	}); err != nil {
		return nil, fmt.Errorf("walk blobdir: %w", err)
	}

	return digests, nil
}

// Clean executes a garbage collection of any non-blob garbage in the store
// (this includes temporary files and directories not reachable from the CAS
// interface). This MUST NOT remove any blobs or references in the store.
func (e *dirEngine) Clean(ctx context.Context) error {
	// Remove every .umoci directory that isn't flocked.
	matches, err := filepath.Glob(filepath.Join(e.path, ".umoci-*"))
	if err != nil {
		return fmt.Errorf("glob .umoci-*: %w", err)
	}
	for _, path := range matches {
		err = e.cleanPath(ctx, path)
		if err != nil && !errors.Is(err, filepath.SkipDir) {
			return err
		}
	}

	return nil
}

func (e *dirEngine) cleanPath(_ context.Context, path string) (Err error) {
	cfh, err := os.Open(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("open for locking: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, cfh)

	if err := unix.Flock(int(cfh.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		// If we fail to get a flock(2) then it's probably already locked,
		// so we shouldn't touch it.
		return filepath.SkipDir
	}
	defer funchelpers.VerifyError(&Err, func() error {
		return unix.Flock(int(cfh.Fd()), unix.LOCK_UN)
	})

	if err := os.RemoveAll(path); errors.Is(err, os.ErrNotExist) {
		return nil // somebody else beat us to it
	} else if err != nil {
		log.Warnf("failed to remove %s: %v", path, err)
		return filepath.SkipDir
	}
	log.Debugf("cleaned %s", path)

	return nil
}

// Close releases all references held by the e. Subsequent operations may
// fail.
func (e *dirEngine) Close() error {
	if e.temp != "" {
		if err := unix.Flock(int(e.tempFile.Fd()), unix.LOCK_UN); err != nil {
			return fmt.Errorf("unlock tempdir: %w", err)
		}
		if err := e.tempFile.Close(); err != nil {
			return fmt.Errorf("close tempdir: %w", err)
		}
		if err := os.RemoveAll(e.temp); err != nil {
			return fmt.Errorf("remove tempdir: %w", err)
		}
	}
	return nil
}

// Open opens a new reference to the directory-backed OCI image referenced by
// the provided path.
func Open(path string) (cas.Engine, error) {
	engine := &dirEngine{
		path: path,
		temp: "",
	}

	if err := engine.validate(); err != nil {
		return nil, fmt.Errorf("validate: %w", err)
	}

	return engine, nil
}

// Create creates a new OCI image layout at the given path. If the path already
// exists, os.ErrExist is returned. However, all of the parent components of
// the path will be created if necessary.
func Create(path string) (Err error) {
	// We need to fail if path already exists, but we first create all of the
	// parent paths.
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}
	}
	if err := os.Mkdir(path, 0o755); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}

	// Create the necessary directories and "oci-layout" file.
	if err := os.Mkdir(filepath.Join(path, blobDirectory), 0o755); err != nil {
		return fmt.Errorf("mkdir blobdir: %w", err)
	}
	if err := os.Mkdir(filepath.Join(path, blobDirectory, cas.BlobAlgorithm.String()), 0o755); err != nil {
		return fmt.Errorf("mkdir algorithm: %w", err)
	}

	indexFh, err := os.Create(filepath.Join(path, indexFile))
	if err != nil {
		return fmt.Errorf("create index.json: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, indexFh)

	defaultIndex := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2, // FIXME: This is hardcoded at the moment.
		},
		MediaType: ispec.MediaTypeImageIndex,
		Manifests: []ispec.Descriptor{},
	}
	if err := json.NewEncoder(indexFh).Encode(defaultIndex); err != nil {
		return fmt.Errorf("encode index.json: %w", err)
	}

	layoutFh, err := os.Create(filepath.Join(path, layoutFile))
	if err != nil {
		return fmt.Errorf("create oci-layout: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, layoutFh)

	ociLayout := ispec.ImageLayout{
		Version: ImageLayoutVersion,
	}
	if err := json.NewEncoder(layoutFh).Encode(ociLayout); err != nil {
		return fmt.Errorf("encode oci-layout: %w", err)
	}

	// Everything is now set up.
	return nil
}
