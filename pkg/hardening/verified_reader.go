/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2018 SUSE LLC.
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

package hardening

import (
	"io"

	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

// ErrDigestMismatch indicates that VerifiedReadCloser encountered a digest
// error on-EOF.
var ErrDigestMismatch = errors.Errorf("verified reader digest mismatch")

// VerifiedReadCloser is a basic io.ReadCloser which allows for simple
// verification that a stream matches an expected hash. The entire stream is
// hashed while being passed through this reader, and on EOF it will verify
// that the hash matches the expected hash. If not, an error is returned. Note
// that this means you need to read all input to EOF in order to find
// verification errors.
//
// If Reader is a VerifiedReadCloser (with the same ExpectedDigest), all of the
// methods are just piped to the underlying methods (with no verification in
// the upper layer).
type VerifiedReadCloser struct {
	// Reader is the underlying reader.
	Reader io.ReadCloser

	// ExpectedDigest is the expected digest. When the underlying reader
	// returns an EOF, the entire stream's sum will be compared to this hash
	// and an error will be returned if they don't match.
	ExpectedDigest digest.Digest

	// digester stores the current state of the stream's hash.
	digester digest.Digester
}

func (v *VerifiedReadCloser) init() {
	// Define digester if not already set.
	if v.digester == nil {
		alg := v.ExpectedDigest.Algorithm()
		if !alg.Available() {
			log.Fatalf("verified reader: unsupported hash algorithm %s", alg)
			panic("verified reader: unreachable section") // should never be hit
		}
		v.digester = alg.Digester()
	}
}

func (v *VerifiedReadCloser) isNoop() bool {
	innerV, ok := v.Reader.(*VerifiedReadCloser)
	return ok && innerV.ExpectedDigest == v.ExpectedDigest
}

// Read is a wrapper around VerifiedReadCloser.Reader, with a digest check on
// EOF.  Make sure that you always check for EOF and read-to-the-end for all
// files.
func (v *VerifiedReadCloser) Read(p []byte) (int, error) {
	// Piped to the underling read.
	n, err := v.Reader.Read(p)
	// Are we going to be a noop?
	if v.isNoop() {
		return n, err
	}
	// Make sure we're ready.
	v.init()
	// Forward it to the digester.
	if n > 0 {
		// hash.Hash guarantees Write() never fails and is never short.
		nWrite, err := v.digester.Hash().Write(p[:n])
		if nWrite != n || err != nil {
			log.Fatalf("verified reader: short write to %s Digester (err=%v)", v.ExpectedDigest.Algorithm(), err)
			panic("verified reader: unreachable section") // should never be hit
		}
	}
	// We have finished reading -- let's check the digest!
	if errors.Cause(err) == io.EOF {
		if actualDigest := v.digester.Digest(); actualDigest != v.ExpectedDigest {
			err = errors.Wrapf(ErrDigestMismatch, "expected %s not %s", v.ExpectedDigest, actualDigest)
		}
	}
	return n, err
}

// Close is a wrapper around VerifiedReadCloser.Reader, but with a digest check
// which will return an error if the underlying Close() didn't.
func (v *VerifiedReadCloser) Close() error {
	// Piped to underlying close.
	err := v.Reader.Close()
	if err != nil {
		return err
	}
	// Are we going to be a noop?
	if v.isNoop() {
		return err
	}
	// Make sure we're ready.
	v.init()
	// Check the digest if no other errors occurred.
	if actualDigest := v.digester.Digest(); actualDigest != v.ExpectedDigest {
		err = errors.Wrapf(ErrDigestMismatch, "expected %s not %s", v.ExpectedDigest, actualDigest)
	}
	return err
}
