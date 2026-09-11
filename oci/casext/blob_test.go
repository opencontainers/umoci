// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
 * Copyright (C) 2026 Aleksa Sarai <cyphar@cyphar.com>
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
	"path/filepath"
	"testing"

	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal"
	cas "github.com/opencontainers/umoci/oci/cas/dir"
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
			name: "EmptyJSON-BadData",
			descriptor: ispec.Descriptor{
				MediaType: ispec.MediaTypeEmptyJSON,
				Digest:    "sha256:74234e98afe7498fb5daf1f36ac2d78acc339464f950703b8c019892f982b90b",
				Size:      4,
				Data:      []byte("null"),
			},
			expectedErr: internal.ErrInvalidEmptyJSON,
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

// descriptor.Size comes out of index.json or a manifest, so a descriptor that
// declares more bytes than the blob actually holds is bad image data rather
// than a programmer error. The verified reader reports it as a size mismatch;
// FromDescriptor must return that error rather than panicking on its way out.
func TestFromDescriptorOversizedDescriptor(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	require.NoError(t, cas.Create(image), "create image")

	engine, err := cas.Open(image)
	require.NoError(t, err, "open engine")
	defer engine.Close() //nolint:errcheck
	engineExt := NewEngine(engine)

	manifest := ispec.Manifest{
		Versioned: imeta.Versioned{SchemaVersion: 2},
		MediaType: ispec.MediaTypeImageManifest,
		Config:    ispec.Descriptor{MediaType: ispec.MediaTypeImageConfig},
	}

	digest, size, err := engineExt.PutBlobJSON(t.Context(), manifest)
	require.NoError(t, err, "put manifest blob")

	good := ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    digest,
		Size:      size,
	}

	// Positive control: the honest descriptor must still parse.
	blob, err := engineExt.FromDescriptor(t.Context(), good)
	require.NoError(t, err, "honest descriptor should parse")
	require.NotNil(t, blob)

	oversized := good
	oversized.Size = size + 1000

	_, err = engineExt.FromDescriptor(t.Context(), oversized)
	assert.Error(t, err, "a descriptor that overstates its size should error, not panic") //nolint:testifylint
}
