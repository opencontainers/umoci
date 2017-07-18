/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2017 SUSE LLC.
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
	"reflect"
	"testing"

	"github.com/mohae/deepcopy"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func descriptorPtr(d ispec.Descriptor) *ispec.Descriptor { return &d }

func randomDescriptor(t *testing.T) ispec.Descriptor {
	var descriptor ispec.Descriptor

	// Generate a random digest and length.
	descriptor.Size = int64(rand.Intn(512 * 1024))
	digester := digest.SHA256.Digester()
	io.CopyN(digester.Hash(), crand.Reader, descriptor.Size)
	descriptor.Digest = digester.Digest()

	// Generate a random number of annotations, with random key/values.
	descriptor.Annotations = map[string]string{}
	n := rand.Intn(32)
	for i := 0; i < n; i++ {
		descriptor.Annotations[randomString(32)] = randomString(32)
	}

	return descriptor
}

// Make sure that an identity mapping doesn't change the struct, and that it
// actually does visit all of the descriptors once.
func TestMapDescriptors_Identity(t *testing.T) {
	// List of interfaces to use MapDescriptors on, as well as how many
	// *unique* descriptors they contain.
	ociList := []struct {
		num int
		obj interface{}
	}{
		// Make sure that "base" types work.
		{
			num: 0,
			obj: nil,
		},
		{
			num: 1,
			obj: randomDescriptor(t),
		},
		{
			num: 1,
			obj: descriptorPtr(randomDescriptor(t)),
		},
		{
			num: 3,
			obj: []ispec.Descriptor{
				randomDescriptor(t),
				randomDescriptor(t),
				randomDescriptor(t),
			},
		},
		{
			num: 7,
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
			num: 7,
			obj: ispec.Manifest{
				Config: randomDescriptor(t),
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
			num: 2,
			obj: ispec.Index{
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		// Check that pointers also work.
		{
			num: 5,
			obj: &ispec.Manifest{
				Config: randomDescriptor(t),
				Layers: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			num: 9,
			obj: &ispec.Index{
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
			num: 0,
			obj: []ispec.Descriptor{},
		},
		{
			num: 1,
			obj: ispec.Manifest{
				Config: randomDescriptor(t),
				Layers: nil,
			},
		},
		{
			num: 0,
			obj: ispec.Index{
				Manifests: []ispec.Descriptor{},
			},
		},
		// TODO: Add support for descending into maps.
	}

	for idx, test := range ociList {
		// Make a copy for later comparison.
		original := deepcopy.Copy(test.obj)

		foundSet := map[digest.Digest]int{}

		if err := MapDescriptors(test.obj, func(descriptor ispec.Descriptor) ispec.Descriptor {
			foundSet[descriptor.Digest]++
			return descriptor
		}); err != nil {
			t.Errorf("MapDescriptors(%d) unexpected error: %v", idx, err)
			continue
		}

		// Make sure that we hit everything uniquely.
		found := 0
		for d, n := range foundSet {
			found++
			if n != 1 {
				t.Errorf("MapDescriptors(%d) hit a descriptor more than once: %#v hit %d times", idx, d, n)
			}
		}
		if found != test.num {
			t.Errorf("MapDescriptors(%d) didn't hit the right number, expected %d got %d", idx, test.num, found)
		}

		if !reflect.DeepEqual(original, test.obj) {
			t.Errorf("MapDescriptors(%d) descriptors were modified with identity mapping, expected %#v got %#v", idx, original, test.obj)
		}
	}
}

// Make sure that it is possible to modify a variety of different interfaces.
func TestMapDescriptors_ModifyOCI(t *testing.T) {
	// List of interfaces to use MapDescriptors on.
	ociList := []struct {
		obj interface{}
	}{
		// Make sure that "base" types work.
		{
			obj: descriptorPtr(randomDescriptor(t)),
		},
		{
			obj: []ispec.Descriptor{
				randomDescriptor(t),
				randomDescriptor(t),
				randomDescriptor(t),
			},
		},
		{
			obj: []*ispec.Descriptor{
				descriptorPtr(randomDescriptor(t)),
				descriptorPtr(randomDescriptor(t)),
			},
		},
		// TODO: Add the ability to mutate map keys and values.
		// Make sure official OCI structs work.
		{
			obj: &ispec.Manifest{
				Config: randomDescriptor(t),
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
			obj: ispec.Index{
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
		{
			obj: &ispec.Index{
				Manifests: []ispec.Descriptor{
					randomDescriptor(t),
					randomDescriptor(t),
				},
			},
		},
	}

	for idx, test := range ociList {
		// Make a copy for later comparison.
		original := deepcopy.Copy(test.obj)

		if err := MapDescriptors(&test.obj, func(descriptor ispec.Descriptor) ispec.Descriptor {
			// Create an entirely new descriptor.
			return randomDescriptor(t)
		}); err != nil {
			t.Errorf("MapDescriptors(%d) unexpected error: %v", idx, err)
			continue
		}

		if reflect.DeepEqual(original, test.obj) {
			t.Errorf("MapDescriptors(%d) descriptor was unmodified when replacing with a random descriptor!", idx)
		}
	}
}

// TODO: We should be able to rewrite non-OCI structs in the future.
