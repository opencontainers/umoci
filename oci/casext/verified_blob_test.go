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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/oci/cas/dir"
)

func TestGetVerifiedBlob(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	descMap := fakeSetupEngine(t, engineExt)
	assert.NotEmpty(t, descMap, "fakeSetupEngine descriptor map")

	t.Run("NonExist", func(t *testing.T) {
		for idx, test := range descMap {
			t.Run(fmt.Sprintf("Descriptor%.2d", idx+1), func(t *testing.T) {
				const badDigest = digest.Digest("sha256:000111222333444555666777888999aaabbbcccdddeeefff0123456789abcdef")
				desc := test.result

				badDescriptor := ispec.Descriptor{
					MediaType: desc.MediaType,
					Digest:    badDigest,
					Size:      desc.Size,
				}

				blob, err := engineExt.GetVerifiedBlob(t.Context(), badDescriptor)
				assert.ErrorIs(t, err, os.ErrNotExist, "get non-existent verified blob (negative descriptor size)") //nolint:testifylint // assert.*Error* makes more sense
				if !assert.Nil(t, blob, "get verified blob (negative descriptor size)") {
					_ = blob.Close()
				}
			})
		}
	})

	t.Run("InvalidSize", func(t *testing.T) {
		for idx, test := range descMap {
			t.Run(fmt.Sprintf("Descriptor%.2d", idx+1), func(t *testing.T) {
				desc := test.result

				blob, err := engineExt.GetVerifiedBlob(t.Context(), desc)
				assert.NoError(t, err, "get verified blob (regular descriptor)") //nolint:testifylint // assert.*Error* makes more sense
				if assert.NotNil(t, blob, "get verified blob (regular descriptor)") {
					// Avoid "trailing data" log warnings on Close.
					_, _ = io.Copy(io.Discard, blob)
					_ = blob.Close()
				}

				badDescriptor := ispec.Descriptor{
					MediaType: desc.MediaType,
					Digest:    desc.Digest,
					Size:      -1, // invalid!
				}

				blob, err = engineExt.GetVerifiedBlob(t.Context(), badDescriptor)
				assert.ErrorIs(t, err, errInvalidDescriptorSize, "get verified blob (negative descriptor size)") //nolint:testifylint // assert.*Error* makes more sense
				if !assert.Nil(t, blob, "get verified blob (negative descriptor size)") {
					_ = blob.Close()
				}
			})
		}
	})
}
