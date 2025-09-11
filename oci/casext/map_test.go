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
	crand "crypto/rand"
	"io"
	"math/rand"
	"testing"

	"github.com/mohae/deepcopy"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal/testhelpers"
)

func descriptorPtr(d ispec.Descriptor) *ispec.Descriptor { return &d }

func randomDescriptor(t *testing.T) ispec.Descriptor {
	var descriptor ispec.Descriptor

	// Generate a random digest and length.
	descriptor.Size = int64(rand.Intn(512 * 1024))
	digester := digest.SHA256.Digester()
	copied, _ := io.CopyN(digester.Hash(), crand.Reader, descriptor.Size)
	require.Equal(t, descriptor.Size, copied, "copy random to descriptor digest data")
	descriptor.Digest = digester.Digest()

	// Generate a random number of annotations, with random key/values.
	descriptor.Annotations = map[string]string{}
	n := rand.Intn(32)
	for range n {
		descriptor.Annotations[testhelpers.RandomString(32)] = testhelpers.RandomString(32)
	}

	return descriptor
}

// Make sure that an identity mapping doesn't change the struct, and that it
// actually does visit all of the descriptors once.
func TestMapDescriptors_Identity(t *testing.T) {
	// List of interfaces to use MapDescriptors on, as well as how many
	// *unique* descriptors they contain.
	tests := []struct {
		name string
		num  int
		obj  any
	}{
		// Make sure that "base" types work.
		{
			name: "Nil",
			num:  0,
			obj:  nil,
		},
		{
			name: "Plain",
			num:  1,
			obj:  randomDescriptor(t),
		},
		{
			name: "Ptr",
			num:  1,
			obj:  descriptorPtr(randomDescriptor(t)),
		},
		{
			name: "Slice-Plain",
			num:  3,
			obj: []ispec.Descriptor{
				randomDescriptor(t),
				randomDescriptor(t),
				randomDescriptor(t),
			},
		},
		{
			name: "Slice-Ptr",
			num:  7,
			obj: []*ispec.Descriptor{
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
			},
		},
		// Make sure official OCI structs work.
		{
			name: "Manifest-Plain",
			num:  7,
			obj: ispec.Manifest{
				MediaType: ispec.MediaTypeImageManifest,
				Config:    randomDescriptor(t),
				Layers: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			name: "Index-Plain",
			num:  2,
			obj: ispec.Index{
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		// Check that pointers also work.
		{
			name: "Manifest-Ptr",
			num:  5,
			obj: &ispec.Manifest{
				MediaType: ispec.MediaTypeImageManifest,
				Config:    randomDescriptor(t),
				Layers: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			name: "Index-Ptr",
			num:  9,
			obj: &ispec.Index{
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		// Make sure that an empty []ispec.Descriptor works properly.
		{
			name: "Slice-Empty",
			num:  0,
			obj:  []ispec.Descriptor{},
		},
		{
			name: "Manifest-NoLayers",
			num:  1,
			obj: ispec.Manifest{
				MediaType: ispec.MediaTypeImageManifest,
				Config:    randomDescriptor(t),
				Layers:    nil,
			},
		},
		{
			name: "Index-NoManifests",
			num:  0,
			obj: ispec.Index{
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{},
			},
		},
		// TODO: Add support for descending into maps.
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			// Make a copy for later comparison.
			original := deepcopy.Copy(test.obj)

			foundSet := map[digest.Digest]int{}

			require.NoError(t, MapDescriptors(test.obj, func(descriptor ispec.Descriptor) ispec.Descriptor {
				foundSet[descriptor.Digest]++
				return descriptor
			}), "MapDescriptors should not return an error")

			// Make sure that we hit everything uniquely.
			for d, n := range foundSet {
				assert.Equalf(t, 1, n, "MapDescriptors(%d) should only hit a descriptor once", d)
			}
			assert.Len(t, foundSet, test.num, "MapDescriptors hit an unexpected number of descriptors")
			assert.Equal(t, original, test.obj, "MapDescriptors with identify mapping should not change object")
		})
	}
}

// Make sure that it is possible to modify a variety of different interfaces.
func TestMapDescriptors_ModifyOCI(t *testing.T) {
	// List of interfaces to use MapDescriptors on.
	ociList := []struct {
		name string
		obj  any
	}{
		// Make sure that "base" types work.
		{
			name: "Ptr",
			obj:  descriptorPtr(randomDescriptor(t)),
		},
		{
			name: "Slice-Plain",
			obj: []ispec.Descriptor{
				randomDescriptor(t),
				randomDescriptor(t),
				randomDescriptor(t),
			},
		},
		{
			name: "Slice-Ptr",
			obj: []*ispec.Descriptor{
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
			},
		},
		// TODO: Add the ability to mutate map keys and values.
		// Make sure official OCI structs work.
		{
			name: "Manifest-Ptr",
			obj: &ispec.Manifest{
				MediaType: ispec.MediaTypeImageManifest,
				Config:    randomDescriptor(t),
				Layers: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			name: "Index-Plain",
			obj: ispec.Index{
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			name: "Index-Ptr",
			obj: &ispec.Index{
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
	}

	for _, test := range ociList {
		t.Run(test.name, func(t *testing.T) {
			// Make a copy for later comparison.
			original := deepcopy.Copy(test.obj)

			newDescriptors := map[digest.Digest]bool{}
			require.NoError(t, MapDescriptors(test.obj, func(descriptor ispec.Descriptor) ispec.Descriptor { //nolint:revive // unused-parameter doesn't make sense for this test
				// Create an entirely new descriptor.
				newDesc := randomDescriptor(t)
				newDescriptors[newDesc.Digest] = true
				return newDesc
			}), "MapDescriptors should not return an error")

			foundDescriptors := map[digest.Digest]bool{}
			require.NoError(t, MapDescriptors(test.obj, func(descriptor ispec.Descriptor) ispec.Descriptor {
				foundDescriptors[descriptor.Digest] = true
				return descriptor
			}), "MapDescriptors should not return an error")

			assert.NotEqual(t, original, test.obj, "MapDescriptors should modify the structure")
			assert.Equal(t, newDescriptors, foundDescriptors, "walking through object after modifying should yield the same set of descriptors")
		})
	}
}

// TODO: We should be able to rewrite non-OCI structs in the future.
