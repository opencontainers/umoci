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
	"io"
	"testing"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/pkg/hardening"
)

func TestDescriptorEmbeddedData(t *testing.T) {
	for _, test := range []struct {
		name         string
		descriptor   ispec.Descriptor
		expectedErr  error
		expectedData any
	}{
		{
			name: "Empty",
			descriptor: ispec.Descriptor{
				MediaType: "application/octet-stream; charset=binary",
				Digest:    "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				Size:      0,
				Data:      []byte{},
			},
		},
		{
			name:         "EmptyJSON",
			descriptor:   ispec.DescriptorEmptyJSON,
			expectedData: struct{}{},
		},
		{
			name: "BadDigest",
			descriptor: ispec.Descriptor{
				MediaType: "application/text",
				Digest:    "sha256:088d9fb7a4966acfd030fa54f1a096e7e33482e2a3ee3bd9a2f2b97cf4d50ce3",
				Size:      45,
				Data:      []byte("The quick brown fox jumps over the lazy dog.\n"),
			},
			expectedErr: hardening.ErrDigestMismatch,
		},
		{
			name: "BadSize",
			descriptor: ispec.Descriptor{
				MediaType: "application/text",
				Digest:    "sha256:b47cc0f104b62d4c7c30bcd68fd8e67613e287dc4ad8c310ef10cbadea9c4380",
				Size:      44,
				Data:      []byte("The quick brown fox jumps over the lazy dog.\n"),
			},
			expectedErr: hardening.ErrSizeMismatch,
		},
		{
			name: "Text",
			descriptor: ispec.Descriptor{
				MediaType: "application/text",
				Digest:    "sha256:b47cc0f104b62d4c7c30bcd68fd8e67613e287dc4ad8c310ef10cbadea9c4380",
				Size:      45,
				Data:      []byte("The quick brown fox jumps over the lazy dog.\n"),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			engine := NewEngine(nil) // will panic if we operate on the underlying CAS
			t.Run("FromDescriptor", func(t *testing.T) {
				blob, err := engine.FromDescriptor(t.Context(), test.descriptor)
				require.ErrorIs(t, err, test.expectedErr, "FromDescriptor(%#v)", test.descriptor)
				if test.expectedErr == nil {
					assert.Equal(t, test.descriptor.Data, blob.RawData, "blob RawData should match embedded data")
				} else {
					assert.Nil(t, blob, "blob should be nil in error path")
				}
				if test.expectedData != nil {
					assert.Equal(t, test.expectedData, blob.Data, "parsed blob data should match")
				}
			})
			t.Run("GetVerifiedBlob", func(t *testing.T) {
				rdr, err := engine.GetVerifiedBlob(t.Context(), test.descriptor)
				require.ErrorIs(t, err, test.expectedErr, "GetVerifiedBlob(%#v)", test.descriptor)
				if test.expectedErr == nil {
					data, err := io.ReadAll(rdr)
					require.NoError(t, err, "read all embedded data should succeed")
					assert.Equal(t, test.descriptor.Data, data, "GetVerifiedBlob should match embedded data")
				} else {
					assert.Nil(t, rdr, "reader should be nil in error path")
				}
			})
		})
	}
}
