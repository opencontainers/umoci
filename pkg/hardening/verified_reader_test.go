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

package hardening

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"testing"

	// Needed for digest.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValid(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			_, err := io.CopyN(buffer, rand.Reader, int64(size))
			require.NoError(t, err, "fill buffer with random data")

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         io.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(size),
			}

			// Make sure if we copy-to-EOF we get no errors.
			_, err = io.Copy(io.Discard, verifiedReader)
			require.NoError(t, err, "digest+size should be correct on EOF")

			// And on close we shouldn't get an error either.
			err = verifiedReader.Close()
			require.NoError(t, err, "digest+size should be correct on Close")
		})
	}
}

func TestNegativeExpectedSize(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			_, err := io.CopyN(buffer, rand.Reader, int64(size))
			require.NoError(t, err, "fill buffer with random data")

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         io.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   -1,
			}

			// Make sure that negative ExpectedSize always fails.
			n, err := io.Copy(io.Discard, verifiedReader)
			assert.ErrorIs(t, err, ErrInvalidExpectedSize, "io.Copy (with ExpectedSize < 0) should fail") //nolint:testifylint // assert.*Error* makes more sense
			assert.Zero(t, n, "io.Copy (with ExpectedSize < 0) should read nothing")

			// Bad ExpectedSize should read no data.
			assert.Equal(t, size, buffer.Len(), "io.Copy (with ExpectedSize < 0) should not have read any data")

			// And on close we shouldn't get an error either.
			err = verifiedReader.Close()
			assert.ErrorIs(t, err, ErrInvalidExpectedSize, "Close (with ExpectedSize < 0) should fail") //nolint:testifylint // assert.*Error* makes more sense

			// Bad ExpectedSize should read no data.
			assert.Equal(t, size, buffer.Len(), "close (with ExpectedSize < 0) should not have read any data")
		})
	}
}

func TestValidTrailing(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			_, err := io.CopyN(buffer, rand.Reader, int64(size))
			require.NoError(t, err, "fill buffer with random data")

			// Get expected hash.
			expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
			verifiedReader := &VerifiedReadCloser{
				Reader:         io.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(size),
			}

			// Read *half* of the bytes, leaving some remaining. We should get
			// no errors.
			_, err = io.CopyN(io.Discard, verifiedReader, int64(size/2))
			require.NoError(t, err, "should get no errors when reading half of blob")

			// On close we shouldn't get an error, even though there are
			// trailing bytes still in the buffer.
			err = verifiedReader.Close()
			require.NoError(t, err, "digest should be correct on Close")
		})
	}
}

func TestInvalidDigest(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		t.Run(fmt.Sprintf("size:%d", size), func(t *testing.T) {
			// Fill buffer with random data.
			buffer := new(bytes.Buffer)
			_, err := io.CopyN(buffer, rand.Reader, int64(size))
			require.NoError(t, err, "fill buffer with random data")

			// Generate an *incorrect* hash. Note that we need to make sure the
			// appended byte is actually different to the original buffer
			// (otherwise with size=1 we could get a spurrious test failure if
			// the random byte is the same as the byte we replace it with).
			fakeBytes := append(buffer.Bytes()[1:], buffer.Bytes()[0]^0x80)
			expectedDigest := digest.SHA256.FromBytes(fakeBytes)
			verifiedReader := &VerifiedReadCloser{
				Reader:         io.NopCloser(buffer),
				ExpectedDigest: expectedDigest,
				ExpectedSize:   int64(size),
			}

			// Make sure if we copy-to-EOF we get the right error.
			_, err = io.Copy(io.Discard, verifiedReader)
			assert.ErrorIs(t, err, ErrDigestMismatch, "digest should be invalid on EOF") //nolint:testifylint // assert.*Error* makes more sense

			// And on close we should get the same error.
			err = verifiedReader.Close()
			assert.ErrorIs(t, err, ErrDigestMismatch, "digest should be invalid on Close") //nolint:testifylint // assert.*Error* makes more sense
		})
	}
}

func TestInvalidSize_LongBuffer(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				_, err := io.CopyN(buffer, rand.Reader, int64(size))
				require.NoError(t, err, "fill buffer with random data")

				// Create a fake digest and size for a subset of the buffer,
				// but get the VerifiedReadCloser to read the full buffer. This
				// will ensure that we disallow someone appending data to the
				// end of the buffer without us noticing (and that we stop
				// reading once we step over the expected length -- that we
				// don't read the entire buffer!).
				shortBuffer := buffer.Bytes()[:size-delta]
				expectedDigest := digest.SHA256.FromBytes(shortBuffer)
				verifiedReader := &VerifiedReadCloser{
					Reader:         io.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   int64(size - delta),
				}

				// Make sure if we try to copy-to-EOF we get the right error.
				read, err := io.Copy(io.Discard, verifiedReader)
				assert.ErrorIs(t, err, ErrSizeMismatch, "size should be invalid on full copy") //nolint:testifylint // assert.*Error* makes more sense

				// Make sure we don't actually read to the end of the buffer if
				// there is a known size. Copy should say that it only read
				// ExpectedSize bytes, and internally we should only read one
				// past the end of ExpectedSize.
				assert.Equal(t, verifiedReader.ExpectedSize, read, "Copy should not read past ExpectedSize")
				assert.Equal(t, verifiedReader.ExpectedSize+1, verifiedReader.currentSize, "VerifiedReadCloser.Read should internally only read one byte past the ExpectedSize")
				assert.Len(t, buffer.Bytes(), delta-1, "buffer should still have some remaining bytes after Copy")

				// On close we should get the error.
				err = verifiedReader.Close()
				assert.ErrorIs(t, err, ErrSizeMismatch, "size should be invalid on Close") //nolint:testifylint // assert.*Error* makes more sense

				// Close also shouldn't read any more bytes from the buffer.
				assert.Equal(t, verifiedReader.ExpectedSize, read, "VerifiedReadCloser.Close should not read past ExpectedSize")
				assert.Equal(t, verifiedReader.ExpectedSize+1, verifiedReader.currentSize, "VerifiedReadCloser.Close should internally only read one byte past the ExpectedSize")
				assert.Len(t, buffer.Bytes(), delta-1, "buffer should still have some remaining bytes after VerifiedReadCloser.Close")
			})
		}
	}
}

func TestInvalidSize_ShortBuffer(t *testing.T) {
	for size := 1; size <= 16384; size *= 2 {
		for delta := 1; delta-1 <= size/2; delta *= 2 {
			t.Run(fmt.Sprintf("size:%d_delta:%d", size, delta), func(t *testing.T) {
				// Fill buffer with random data.
				buffer := new(bytes.Buffer)
				_, err := io.CopyN(buffer, rand.Reader, int64(size))
				require.NoError(t, err, "fill buffer with random data")

				// Generate a correct hash, but set the size to be larger.
				expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
				verifiedReader := &VerifiedReadCloser{
					Reader:         io.NopCloser(buffer),
					ExpectedDigest: expectedDigest,
					ExpectedSize:   int64(size + delta),
				}

				// Make sure if we try to copy-to-EOF we get the right error.
				_, err = io.Copy(io.Discard, verifiedReader)
				assert.ErrorIs(t, err, ErrSizeMismatch, "size should be invalid on full copy") //nolint:testifylint // assert.*Error* makes more sense

				// On close we should get the error.
				err = verifiedReader.Close()
				assert.ErrorIs(t, err, ErrSizeMismatch, "size should be invalid on Close") //nolint:testifylint // assert.*Error* makes more sense
			})
		}
	}
}

func TestNoop(t *testing.T) {
	// Fill buffer with random data.
	buffer := new(bytes.Buffer)
	size := 256
	_, err := io.CopyN(buffer, rand.Reader, int64(size))
	require.NoError(t, err, "fill buffer with random data")

	// Get expected hash.
	expectedDigest := digest.SHA256.FromBytes(buffer.Bytes())
	verifiedReader := &VerifiedReadCloser{
		Reader:         io.NopCloser(buffer),
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
	_, _ = io.Copy(io.Discard, tripleWrappedReader)
	_ = tripleWrappedReader.Close()

	// Bottom-most wrapper should've been hit.
	assert.NotNil(t, verifiedReader.digester, "verified reader digester should be active")
	// Middle wrapper (identical to lowest) is a noop.
	assert.Nil(t, wrappedReader.digester, "wrapped reader digester should be a noop")
	// Different-digest wrapper is *not* a noop.
	assert.NotNil(t, doubleWrappedReader.digester, "wrapper reader with different digest should be active")
	// Different-size wrapper is *not* a noop.
	assert.NotNil(t, tripleWrappedReader.digester, "wrapper reader with different size should be active")
}
