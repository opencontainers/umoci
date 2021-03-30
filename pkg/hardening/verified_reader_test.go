/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	// Needed for digest.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
)

func TestValid(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
				t.Fatalf("getting random data for buffer failed: %v", err)
			}

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         ioutil.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(size),
			}

			// Make sure everything if we copy-to-EOF we get no errors.
			if _, err := io.Copy(ioutil.Discard, verifiedReader); err != nil {
				t.Errorf("expected digest+size to be correct on EOF: got an error: %v", err)
			}

			// And on close we shouldn't get an error either.
			if err := verifiedReader.Close(); err != nil {
				t.Errorf("expected digest+size to be correct on Close: got an error: %v", err)
			}
		})
	}
}

func TestValidIgnoreLength(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
				t.Fatalf("getting random data for buffer failed: %v", err)
			}

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         ioutil.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(-1),
			}

			// Make sure everything if we copy-to-EOF we get no errors.
			if _, err := io.Copy(ioutil.Discard, verifiedReader); err != nil {
				t.Errorf("expected digest+size to be correct on EOF: got an error: %v", err)
			}

			// And on close we shouldn't get an error either.
			if err := verifiedReader.Close(); err != nil {
				t.Errorf("expected digest+size to be correct on Close: got an error: %v", err)
			}
		})
	}
}

func TestValidTrailing(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
				t.Fatalf("getting random data for buffer failed: %v", err)
			}

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         ioutil.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(-1),
			}

			// Read *half* of the bytes, leaving some remaining. We should get
			// no errors.
			if _, err := io.CopyN(ioutil.Discard, verifiedReader, int64(size/2)); err != nil {
				t.Errorf("expected no error after reading only %d bytes: got an error: %v", size/2, err)
			}

			// And on close we shouldn't get an error either.
			if err := verifiedReader.Close(); err != nil {
				t.Errorf("expected digest+size to be correct on Close: got an error: %v", err)
			}
		})
	}
}

func TestInvalidDigest(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
				t.Fatalf("getting random data for buffer failed: %v", err)
			}

			// Generate an *incorrect* hash.
			fakeBytes := append(buffer.Bytes()[1:], 0x80)
			expectedDigest := digest.SHA256.FromBytes(fakeBytes)
			verifiedReader := &VerifiedReadCloser{
				Reader:         ioutil.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(size),
			}

			// Make sure everything if we copy-to-EOF we get the right error.
			if _, err := io.Copy(ioutil.Discard, verifiedReader); errors.Cause(err) != ErrDigestMismatch {
				t.Errorf("expected digest to be invalid on EOF: got wrong error: %v", err)
			}

			// And on close we should get the error.
			if err := verifiedReader.Close(); errors.Cause(err) != ErrDigestMismatch {
				t.Errorf("expected digest to be invalid on Close: got wrong error: %v", err)
			}
		})
	}
}

func TestInvalidDigest_Trailing(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
					t.Fatalf("getting random data for buffer failed: %v", err)
				}

				// Generate a correct hash (for a shorter buffer), but don't
				// verify the size -- this is to make sure that we actually
				// read all the bytes.
				shortBuffer := buffer.Bytes()[:size-delta]
				expectedDigest := digest.SHA256.FromBytes(shortBuffer)
				verifiedReader := &VerifiedReadCloser{
					Reader:         ioutil.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   -1,
				}

				// Make sure everything if we copy-to-EOF we get the right error.
				if _, err := io.CopyN(ioutil.Discard, verifiedReader, int64(size-delta)); err != nil {
					t.Errorf("expected no errors after reading N bytes: got error: %v", err)
				}

				// Check that the digest does actually match right now.
				verifiedReader.init()
				if err := verifiedReader.verify(nil); err != nil {
					t.Errorf("expected no errors in verify before Close: got error: %v", err)
				}

				// And on close we should get the error.
				if err := verifiedReader.Close(); errors.Cause(err) != ErrDigestMismatch {
					t.Errorf("expected digest to be invalid on Close: got wrong error: %v", err)
				}
			})

		}
	}
}

func TestInvalidSize_Short(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
					t.Fatalf("getting random data for buffer failed: %v", err)
				}

				// Generate a correct hash (for a shorter buffer), but limit the
				// size to be smaller.
				shortBuffer := buffer.Bytes()[:buffer.Len()-delta]
				expectedDigest := digest.SHA256.FromBytes(shortBuffer)
				verifiedReader := &VerifiedReadCloser{
					Reader:         ioutil.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   int64(size - delta),
				}

				// Make sure everything if we copy-to-EOF we get the right error.
				if _, err := io.Copy(ioutil.Discard, verifiedReader); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on EOF: got wrong error: %v", err)
				}

				// And on close we should get the error.
				if err := verifiedReader.Close(); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on Close: got wrong error: %v", err)
				}
			})

		}
	}
}

func TestInvalidSize_LongBuffer(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
					t.Fatalf("getting random data for buffer failed: %v", err)
				}

				// Generate a correct hash (for the full buffer), but limit the
				// size to be smaller (so that we ensure we don't allow such
				// actions).
				shortBuffer := buffer.Bytes()[:size-delta]
				expectedDigest := digest.SHA256.FromBytes(shortBuffer)
				verifiedReader := &VerifiedReadCloser{
					Reader:         ioutil.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   int64(size - delta),
				}

				// Make sure everything if we copy-to-EOF we get the right error.
				if _, err := io.Copy(ioutil.Discard, verifiedReader); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on EOF: got wrong error: %v", err)
				}

				// And on close we should get the error.
				if err := verifiedReader.Close(); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on Close: got wrong error: %v", err)
				}
			})
		}
	}
}

func TestInvalidSize_Long(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
					t.Fatalf("getting random data for buffer failed: %v", err)
				}

				// Generate a correct hash, but set the size to be larger.
				expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
				verifiedReader := &VerifiedReadCloser{
					Reader:         ioutil.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   int64(size + delta),
				}

				// Make sure everything if we copy-to-EOF we get the right error.
				if _, err := io.Copy(ioutil.Discard, verifiedReader); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on EOF: got wrong error: %v", err)
				}

				// And on close we should get the error.
				if err := verifiedReader.Close(); errors.Cause(err) != ErrSizeMismatch {
					t.Errorf("expected size to be invalid on Close: got wrong error: %v", err)
				}
			})
		}
	}
}

func TestNoop(t *testing.T) {
	// Fill buffer with random data.
	buffer := new(bytes.Buffer)
	size := 256
	if _, err := io.CopyN(buffer, rand.Reader, int64(size)); err != nil {
		t.Fatalf("getting random data for buffer failed: %v", err)
	}

	// Get expected hash.
	expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
	verifiedReader := &VerifiedReadCloser{
		Reader:         ioutil.NopCloser(buffer),
		ExpectedDigest: expectedDigest,
		ExpectedSize:   int64(size),
	}

	// And make an additional wrapper with the same digest+size ...
	wrappedReader := &VerifiedReadCloser{
		Reader:         verifiedReader,
		ExpectedDigest: verifiedReader.ExpectedDigest,
		ExpectedSize:   verifiedReader.ExpectedSize,
	}

	// ... and a different digest.
	doubleWrappedReader := &VerifiedReadCloser{
		Reader:         wrappedReader,
		ExpectedDigest: digest.SHA256.FromString("foo"),
		ExpectedSize:   wrappedReader.ExpectedSize,
	}

	// ... and a different size.
	tripleWrappedReader := &VerifiedReadCloser{
		Reader:         doubleWrappedReader,
		ExpectedDigest: doubleWrappedReader.ExpectedDigest,
		ExpectedSize:   doubleWrappedReader.ExpectedSize - 1,
	}

	// Read from the uppermost wrapper, ignoring all errors.
	_, _ = io.Copy(ioutil.Discard, tripleWrappedReader)
	_ = tripleWrappedReader.Close()

	// Bottom-most wrapper should've been hit.
	if verifiedReader.digester == nil {
		t.Errorf("verifiedReader didn't digest input")
	}
	// Middle wrapper (identical to lowest) is a noop.
	if wrappedReader.digester != nil {
		t.Errorf("wrappedReader wasn't noop'd out")
	}
	// Different-digest wrapper is *not* a noop.
	if doubleWrappedReader.digester == nil {
		t.Errorf("doubleWrappedReader was incorrectly noop'd out")
	}
	// Different-size wrapper is *not* a noop.
	if tripleWrappedReader.digester == nil {
		t.Errorf("tripleWrappedReader was incorrectly noop'd out")
	}
}
