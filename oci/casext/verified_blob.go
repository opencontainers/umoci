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

package casext

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/internal"
	"github.com/opencontainers/umoci/pkg/hardening"
)

var errInvalidDescriptorSize = errors.New("descriptor size must not be negative")

// GetVerifiedBlob returns a VerifiedReadCloser for retrieving a blob from the
// image, which the caller must Close() *and* read-to-EOF (checking the error
// code of both). Returns ErrNotExist if the digest is not found, and
// ErrBlobDigestMismatch on a mismatched blob digest. In addition, the reader
// is limited to the descriptor.Size.
func (e Engine) GetVerifiedBlob(ctx context.Context, descriptor ispec.Descriptor) (io.ReadCloser, error) {
	// Negative sizes are not permitted by the spec, and are a DoS vector.
	if descriptor.Size < 0 {
		return nil, fmt.Errorf("invalid descriptor: %w", errInvalidDescriptorSize)
	}
	// The empty blob descriptor only has one valid value so we should validate
	// it before allowing it to be opened.
	if descriptor.MediaType == ispec.MediaTypeEmptyJSON {
		if descriptor.Digest != ispec.DescriptorEmptyJSON.Digest ||
			descriptor.Size != ispec.DescriptorEmptyJSON.Size {
			return nil, fmt.Errorf("invalid descriptor: %w", internal.ErrInvalidEmptyJSON)
		}
		if descriptor.Data != nil &&
			!bytes.Equal(descriptor.Data, ispec.DescriptorEmptyJSON.Data) {
			return nil, fmt.Errorf("invalid descriptor: %w", internal.ErrInvalidEmptyJSON)
		}
	}
	// Embedded data.
	if descriptor.Data != nil {
		// If the digest is small enough to fit in the descriptor, we can
		// validate it immediately without deferring to VerifiedReadCloser.
		gotDigest := descriptor.Digest.Algorithm().FromBytes(descriptor.Data)
		if gotDigest != descriptor.Digest {
			return nil, fmt.Errorf("invalid embedded descriptor data: expected %s not %s: %w", descriptor.Digest, gotDigest, hardening.ErrDigestMismatch)
		}
		if int64(len(descriptor.Data)) != descriptor.Size {
			return nil, fmt.Errorf("invalid embedded descriptor data: expected %d bytes not %d bytes: %w", descriptor.Size, len(descriptor.Data), hardening.ErrSizeMismatch)
		}
		return io.NopCloser(bytes.NewBuffer(descriptor.Data)), nil
	}

	reader, err := e.GetBlob(ctx, descriptor.Digest)
	if err != nil {
		return nil, err
	}
	return &hardening.VerifiedReadCloser{
		Reader:         reader,
		ExpectedDigest: descriptor.Digest,
		ExpectedSize:   descriptor.Size,
	}, nil
}
