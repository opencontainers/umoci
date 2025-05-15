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

// Package cas provides a generic interface for the content-addressable store
// that underlies OCI images. In most cases you should try to use
// "github.com/opencontainers/umoci/oci/casext" which provides more friendly
// wrappers around the content-addressible store.
package cas

import (
	"context"
	"errors"
	"io"

	// We need to include sha256 in order for go-digest to properly handle such
	// hashes, since Go's crypto library like to lazy-load cryptographic
	// libraries.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

const (
	// BlobAlgorithm is the name of the only supported digest algorithm for blobs.
	// FIXME: We can make this a list.
	BlobAlgorithm = digest.SHA256
)

// Exposed errors.
var (
	// ErrNotExist is effectively an implementation-neutral version of
	// os.ErrNotExist.
	ErrNotExist = errors.New("no such blob or index")

	// ErrInvalid is returned when an image was detected as being invalid.
	ErrInvalid = errors.New("invalid image detected")

	// ErrUnknownType is returned when an unknown (or otherwise unparseable)
	// mediatype is encountered. Callers should not ignore this error unless it
	// is in a context where ignoring it is more friendly to spec extensions.
	ErrUnknownType = errors.New("unknown mediatype encountered")

	// ErrNotImplemented is returned when a requested operation has not been
	// implementing the backing image store.
	ErrNotImplemented = errors.New("operation not implemented")

	// ErrClobber is returned when a requested operation would require clobbering a
	// reference or blob which already exists.
	ErrClobber = errors.New("operation would clobber existing object")
)

// Engine is an interface that provides methods for accessing and modifying an
// OCI image, namely allowing access to reference descriptors and blobs.
type Engine interface {
	// PutBlob adds a new blob to the image. This is idempotent; a nil error
	// means that "the content is stored at DIGEST" without implying "because
	// of this PutBlob() call".
	PutBlob(ctx context.Context, reader io.Reader) (digest digest.Digest, size int64, err error)

	// GetBlob returns a reader for retrieving a blob from the image, which the
	// caller must Close(). Returns ErrNotExist if the digest is not found.
	//
	// This function will return a VerifiedReadCloser, meaning that you must
	// call Close() and check the error returned from Close() in order to
	// ensure that the hash of the blob is verified.
	//
	// Please note that calling Close() on the returned blob will read the
	// entire from disk and hash it (even if you didn't read any bytes before
	// calling Close), so if you wish to only check if a blob exists you should
	// use StatBlob() instead.
	GetBlob(ctx context.Context, digest digest.Digest) (reader io.ReadCloser, err error)

	// StatBlob returns whether the specified blob exists in the image. Returns
	// ErrNotExist if the digest was not found.
	StatBlob(ctx context.Context, digest digest.Digest) (bool, error)

	// PutIndex sets the index of the OCI image to the given index, replacing
	// the previously existing index. This operation is atomic; any readers
	// attempting to access the OCI image while it is being modified will only
	// ever see the new or old index.
	PutIndex(ctx context.Context, index ispec.Index) (err error)

	// GetIndex returns the index of the OCI image. Return ErrNotExist if the
	// digest is not found. If the image doesn't have an index, ErrInvalid is
	// returned (a valid OCI image MUST have an image index).
	//
	// It is not recommended that users of cas.Engine use this interface
	// directly, due to the complication of properly handling references as
	// well as correctly handling nested indexes. casext.Engine provides a
	// wrapper for cas.Engine that implements various reference resolution
	// functions that should work for most users.
	GetIndex(ctx context.Context) (index ispec.Index, ierr error)

	// DeleteBlob removes a blob from the image. This is idempotent; a nil
	// error means "the content is not in the store" without implying "because
	// of this DeleteBlob() call".
	DeleteBlob(ctx context.Context, digest digest.Digest) (err error)

	// ListBlobs returns the set of blob digests stored in the image.
	ListBlobs(ctx context.Context) (digests []digest.Digest, err error)

	// Clean executes a garbage collection of any non-blob garbage in the store
	// (this includes temporary files and directories not reachable from the
	// CAS interface). This MUST NOT remove any blobs or references in the
	// store.
	Clean(ctx context.Context) (err error)

	// Close releases all references held by the engine. Subsequent operations
	// may fail.
	Close() (err error)
}
