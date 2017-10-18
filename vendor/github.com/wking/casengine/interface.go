// Copyright 2017 casengine contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package casengine defines common interfaces for CAS engines.
package casengine

import (
	"io"

	"github.com/opencontainers/go-digest"
	"golang.org/x/net/context"
)

// Reader represents a content-addressable storage engine reader.
type Reader interface {

	// Get returns a reader for retrieving a blob from the store.
	// Returns os.ErrNotExist if the digest is not found.
	//
	// Implementations are *not* required to verify that the returned
	// reader content matches the requested digest.  Callers that need
	// that verification are encouraged to use something like:
	//
	//   rawReader, err := engine.Get(ctx, digest)
	//   defer rawReader.Close()
	//   verifier := digest.Verifier()
	//   verifiedReader := io.TeeReader(rawReader, verifier)
	//   consume(verifiedReader)
	//   if !verifier.Verified() {
	//     dieScreaming()
	//   }
	//
	// for streaming verification.
	Get(ctx context.Context, digest digest.Digest) (reader io.ReadCloser, err error)
}

// AlgorithmCallback templates an AlgorithmLister.Algorithms callback
// used for processing algorithms.  AlgorithmLister.Algorithms for
// more details.
type AlgorithmCallback func(ctx context.Context, algorithm digest.Algorithm) (err error)

// AlgorithmLister represents a content-addressable storage engine
// algorithm lister.
type AlgorithmLister interface {

	// Algorithms returns available algorithms from the store.  The set
	// of algorithms must include those which currently have stored
	// digests, but may or may not include algorithms which may have stored
	// digests in the future.
	//
	// Results are sorted alphabetically.
	//
	// Arguments:
	//
	// * ctx: gives callers a way to gracefully cancel a long-running
	//   list.
	// * prefix: limits the result set to algorithms starting with that
	//   value.
	// * size: limits the length of the result set to the first 'size'
	//   matches.  A value of -1 means "all results".
	// * from: shifts the result set to start from the 'from'th match.
	// * callback: called for every matching algorithm.  Algorithms
	//   returns any errors returned by callback and aborts further
	//   listing.
	//
	// For example, a store with digests like:
	//
	// * sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
	// * sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	// * sha512:cf83e1357eefb8bdf1542850d66d8007d620e4050b5715dc83f4a921d36ce9ce47d0d13c5d85f2b0ff8318d2877eec2f63b931bd47417a81a538327af927da3e
	//
	// which also supports sha384 may have the following call/result pairs:
	//
	// * Algorithms(ctx, "", -1, 0, printAlgorithm) -> "sha256", "sha512"
	//   or
	//   Algorithms(ctx, "", -1, 0, printAlgorithm) -> "sha256", "sha384", "sha512"
	// * Algorithms(ctx, "", 1, 0, printAlgorithm) -> "sha256"
	// * Algorithms(ctx, "", 2, 1, printAlgorithm) -> "sha512"
	//   or
	//   Algorithms(ctx, "", 2, 1, printAlgorithm) -> "sha384", "sha512"
	// * Algorithms(ctx, "sha5", -1, 0, printAlgorithm) -> "sha512"
	Algorithms(ctx context.Context, prefix string, size int, from int, callback AlgorithmCallback) (err error)
}

// DigestCallback templates an DigestLister.Digests callback used for
// processing algorithms.  DigestLister.Digests for more details.
type DigestCallback func(ctx context.Context, digest digest.Digest) (err error)

// DigestLister represents a content-addressable storage engine digest
// lister.
type DigestLister interface {

	// Digests returns available digests from the store.
	//
	// Results are sorted alphabetically.
	//
	// Arguments:
	//
	// * ctx: gives callers a way to gracefully cancel a long-running
	//   list.
	// * algorithm: limits the result set to digests whose algorithm
	//   matches this value.  An empty-string value means "all
	//   algorithms".
	// * prefix: limits the result set to digests whose encoded part
	//   starts with that value.
	// * size: limits the length of the result set to the first 'size'
	//   matches.  A value of -1 means "all results".
	// * from: shifts the result set to start from the 'from'th match.
	// * callback: called for every matching digest.  Digests returns
	//   any errors returned by callback and aborts further listing.
	//
	// For example, a store with digests like:
	//
	// * sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f
	// * sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855
	// * sha512:374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387
	//
	// will have the following call/result pairs:
	//
	// * Digests(ctx, "", "", -1, 0, printDigest) ->
	//     "sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	//     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	//     "sha512:374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387"
	// * Digests(ctx, "sha256", "", -1, 0, printDigest) ->
	//     "sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	//     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// * Digests(ctx, "sha256", "e", -1, 0, printDigest) ->
	//     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// * Digests(ctx, "", "", 2, 0, printDigest) ->
	//     "sha256:dffd6021bb2bd5b0af676290809ec3a53191dd81c7f70a4b28688a362182986f"
	//     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	// * Digests(ctx, "", "", 2, 1, printDigest) ->
	//     "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	//     "sha512:374d794a95cdcfd8b35993185fef9ba368f160d8daf432d08ba9f1ed1e5abe6cc69291e0fa2fe0006a52570ef18c19def4e617c33ce52ef0a6e5fbe318cb0387"
	Digests(ctx context.Context, algorithm digest.Algorithm, prefix string, size int, from int, callback DigestCallback) (err error)
}

// Writer represents a content-addressable storage engine writer.
type Writer interface {

	// Put adds a new blob to the store.  The action is idempotent; a
	// nil return means "that content is stored at DIGEST" without
	// implying "because of your Put()".
	//
	// The algorithm argument allows you to require a particular digest
	// algorithm.  Set to the empty string to allow the Writer to use
	// its preferred algorithm.
	Put(ctx context.Context, algorithm digest.Algorithm, reader io.Reader) (digest digest.Digest, err error)
}

// Deleter represents a content-addressable storage engine deleter.
type Deleter interface {

	// Delete removes a blob from the store. The action is idempotent; a
	// nil return means "that content is not in the store" without
	// implying "because of your Delete()".
	Delete(ctx context.Context, digest digest.Digest) (err error)
}

// Closer represents a content-addressable storage engine closer.
type Closer interface {

	// Close releases resources held by the engine.  Subsequent engine
	// method calls will fail.
	Close(ctx context.Context) (err error)
}

// ReadCloser is the interface that groups the basic Reader and Closer
// interfaces.
type ReadCloser interface {
	Reader
	Closer
}

// ListDeleter is the interface that groups the basic AlgorithmLister,
// DigestLister, and Deleter interfaces.  This combination is useful
// for garbage collection.
type ListDeleter interface {
	AlgorithmLister
	DigestLister
	Deleter
}

// WriteCloser is the interface that groups the basic Writer and
// Closer interfaces.
type WriteCloser interface {
	Writer
	Closer
}

// Engine is the interface that groups all the basic interfaces
// defined in this package except for DigestLister.
type Engine interface {
	Reader
	AlgorithmLister
	Writer
	Deleter
	Closer
}

// DigestListerEngine is the interface that groups all the basic
// interfaces defined in this package.
type DigestListerEngine interface {
	Engine
	DigestLister
}
