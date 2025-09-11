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
	"archive/tar"
	"bytes"
	crand "crypto/rand"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/opencontainers/go-digest"
	ispecs "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal/testhelpers"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext/mediatype"
)

const (
	customMediaType       = "org.opensuse.our-new-type"
	customTargetMediaType = "org.opensuse.our-new-TARGET-type"
	unknownMediaType      = "org.opensuse.fake-manifest"
)

type fakeManifest struct {
	Descriptor ispec.Descriptor `json:"descriptor"`
	Data       []byte           `json:"data"`
}

func init() {
	fakeManifestParser := mediatype.JSONParser[fakeManifest]

	mediatype.RegisterParser(customMediaType, fakeManifestParser)
	mediatype.RegisterTarget(customTargetMediaType)
	mediatype.RegisterParser(customTargetMediaType, fakeManifestParser)
}

type descriptorMap struct {
	index  ispec.Descriptor
	result ispec.Descriptor
}

func randomTarData(tw *tar.Writer) error {
	// Add some files with random contents and random names.
	for n := range 32 {
		size := rand.Intn(512 * 1024)

		if err := tw.WriteHeader(&tar.Header{
			Name:     testhelpers.RandomString(16),
			Mode:     0o755,
			Uid:      rand.Intn(1337),
			Gid:      rand.Intn(1337),
			Size:     int64(size),
			Typeflag: tar.TypeReg,
		}); err != nil {
			return fmt.Errorf("randomTarData WriteHeader %d", n)
		}
		if _, err := io.CopyN(tw, crand.Reader, int64(size)); err != nil {
			return fmt.Errorf("randomTarData Write %d", n)
		}
	}
	return nil
}

// fakeSetupEngine injects a variety of "fake" blobs which may not include a
// full blob tree to test whether Walk and ResolveReference act sanely in the
// face of unknown media types as well as arbitrary nesting of known media
// types. The returned mapping is for a given index -> descriptor you would
// expect to get from ResolveReference.
func fakeSetupEngine(t *testing.T, engineExt Engine) []descriptorMap {
	mapping := []descriptorMap{}

	// Add some "normal" images that contain some layers and also have some
	// index indirects. The multiple layers makes sure that we don't break the
	// multi-level resolution.
	// XXX: In future we'll have to make tests for platform matching.
	for k := range 5 {
		n := 3
		name := fmt.Sprintf("normal_img_%d", k)

		layerData := make([]bytes.Buffer, n)

		// Generate layer data.
		for idx := range layerData {
			tw := tar.NewWriter(&layerData[idx])
			err := randomTarData(tw)
			require.NoErrorf(t, err, "%s: generate layer%d data", name, idx)
			_ = tw.Close()
		}

		// Insert all of the layers.
		layerDescriptors := make([]ispec.Descriptor, n)
		for idx, layer := range layerData {
			digest, size, err := engineExt.PutBlob(t.Context(), &layer)
			require.NoErrorf(t, err, "%s: putting layer%d blob", name, idx)
			layerDescriptors[idx] = ispec.Descriptor{
				MediaType: ispec.MediaTypeImageLayer,
				Digest:    digest,
				Size:      size,
			}
		}

		// Create our config and insert it.
		created := time.Now()
		configDigest, configSize, err := engineExt.PutBlobJSON(t.Context(), ispec.Image{
			Created:      &created,
			Author:       "Jane Author <janesmith@example.com>",
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
			RootFS: ispec.RootFS{
				Type: "unknown",
			},
		})
		require.NoError(t, err, "put config blob json")
		configDescriptor := ispec.Descriptor{
			MediaType: ispec.MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      configSize,
		}

		// Create our manifest and insert it.
		manifest := ispec.Manifest{
			Versioned: ispecs.Versioned{
				SchemaVersion: 2,
			},
			MediaType: ispec.MediaTypeImageManifest,
			Config:    configDescriptor,
		}
		manifest.Layers = append(manifest.Layers, layerDescriptors...)

		manifestDigest, manifestSize, err := engineExt.PutBlobJSON(t.Context(), manifest)
		require.NoError(t, err, "put manifest blob json")
		manifestDescriptor := ispec.Descriptor{
			MediaType: ispec.MediaTypeImageManifest,
			Digest:    manifestDigest,
			Size:      manifestSize,
			Annotations: map[string]string{
				"name": name,
			},
		}

		// Add extra index layers.
		indexDescriptor := manifestDescriptor
		for i := range k {
			newIndex := ispec.Index{
				Versioned: ispecs.Versioned{
					SchemaVersion: 2,
				},
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{indexDescriptor},
			}
			indexDigest, indexSize, err := engineExt.PutBlobJSON(t.Context(), newIndex)
			require.NoErrorf(t, err, "%s: put index-%d blob", name, i)
			indexDescriptor = ispec.Descriptor{
				MediaType: ispec.MediaTypeImageIndex,
				Digest:    indexDigest,
				Size:      indexSize,
			}
		}

		mapping = append(mapping, descriptorMap{
			index:  indexDescriptor,
			result: manifestDescriptor,
		})
	}

	// Add some blobs that have custom mediaTypes. This is loosely based on
	// the previous section.
	for k := range 5 {
		name := fmt.Sprintf("custom_img_%d", k)

		// Create a fake customTargetMediaType (will be masked by a different
		// target media-type above).
		notTargetDigest, notTargetSize, err := engineExt.PutBlobJSON(t.Context(), fakeManifest{
			Data: []byte("Hello, world!"),
		})
		require.NoErrorf(t, err, "%s: put custom manifest blob", name)
		notTargetDescriptor := ispec.Descriptor{
			MediaType: customTargetMediaType,
			Digest:    notTargetDigest,
			Size:      notTargetSize,
			Annotations: map[string]string{
				"name": name,
			},
		}

		// Add extra custom non-target layers.
		currentDescriptor := notTargetDescriptor
		for i := range k {
			newDigest, newSize, err := engineExt.PutBlobJSON(t.Context(), fakeManifest{
				Descriptor: currentDescriptor,
				Data:       []byte("intermediate non-target"),
			})
			require.NoErrorf(t, err, "%s: putting custom-(non)target-%d blob", name, i)
			currentDescriptor = ispec.Descriptor{
				MediaType: customMediaType,
				Digest:    newDigest,
				Size:      newSize,
			}
		}

		// Add the *real* customTargetMediaType.
		targetDigest, targetSize, err := engineExt.PutBlobJSON(t.Context(), fakeManifest{
			Descriptor: currentDescriptor,
			Data:       []byte("I am the real target!"),
		})
		require.NoErrorf(t, err, "%s: putting custom-manifest blob", name)
		targetDescriptor := ispec.Descriptor{
			MediaType: customTargetMediaType,
			Digest:    targetDigest,
			Size:      targetSize,
			Annotations: map[string]string{
				"name": name,
			},
		}

		// Add extra custom non-target layers.
		currentDescriptor = targetDescriptor
		for i := range k {
			newDigest, newSize, err := engineExt.PutBlobJSON(t.Context(), fakeManifest{
				Descriptor: currentDescriptor,
				Data:       []byte("intermediate non-target"),
			})
			require.NoErrorf(t, err, "%s: putting custom-(non)target-%d blob", name, i)
			currentDescriptor = ispec.Descriptor{
				MediaType: customMediaType,
				Digest:    newDigest,
				Size:      newSize,
			}
		}

		// Add extra index layers.
		indexDescriptor := currentDescriptor
		for i := range k {
			newIndex := ispec.Index{
				Versioned: ispecs.Versioned{
					SchemaVersion: 2,
				},
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{indexDescriptor},
			}
			indexDigest, indexSize, err := engineExt.PutBlobJSON(t.Context(), newIndex)
			require.NoErrorf(t, err, "%s: putting index-%d blob", name, i)
			indexDescriptor = ispec.Descriptor{
				MediaType: ispec.MediaTypeImageIndex,
				Digest:    indexDigest,
				Size:      indexSize,
			}
		}

		mapping = append(mapping, descriptorMap{
			index:  indexDescriptor,
			result: targetDescriptor,
		})
	}

	// Add some blobs that have unknown mediaTypes. This is loosely based on
	// the previous section.
	for k := range 5 {
		name := fmt.Sprintf("unknown_img_%d", k)

		manifestDigest, manifestSize, err := engineExt.PutBlobJSON(t.Context(), fakeManifest{
			Descriptor: ispec.Descriptor{
				MediaType: "org.opensuse.fake-data",
				Digest:    digest.SHA256.FromString("Hello, world!"),
				Size:      0,
			},
			Data: []byte("Hello, world!"),
		})
		require.NoErrorf(t, err, "%s: put manifest blob", name)
		manifestDescriptor := ispec.Descriptor{
			MediaType: unknownMediaType,
			Digest:    manifestDigest,
			Size:      manifestSize,
			Annotations: map[string]string{
				"name": name,
			},
		}

		// Add extra index layers.
		indexDescriptor := manifestDescriptor
		for i := range k {
			newIndex := ispec.Index{
				Versioned: ispecs.Versioned{
					SchemaVersion: 2,
				},
				MediaType: ispec.MediaTypeImageIndex,
				Manifests: []ispec.Descriptor{indexDescriptor},
			}
			indexDigest, indexSize, err := engineExt.PutBlobJSON(t.Context(), newIndex)
			require.NoErrorf(t, err, "%s: put index-%d blob", name, i)
			indexDescriptor = ispec.Descriptor{
				MediaType: ispec.MediaTypeImageIndex,
				Digest:    indexDigest,
				Size:      indexSize,
			}
		}

		mapping = append(mapping, descriptorMap{
			index:  indexDescriptor,
			result: manifestDescriptor,
		})
	}

	return mapping
}

func TestEngineReference(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	descMap := fakeSetupEngine(t, engineExt)
	assert.NotEmpty(t, descMap, "fakeSetupEngine descriptor map")

	for idx, test := range descMap {
		t.Run(fmt.Sprintf("Descriptor%.2d", idx+1), func(t *testing.T) {
			name := fmt.Sprintf("new_tag_%d", idx)

			err := engineExt.UpdateReference(t.Context(), name, test.index)
			require.NoErrorf(t, err, "update reference %s", name)

			gotDescriptorPaths, err := engineExt.ResolveReference(t.Context(), name)
			require.NoErrorf(t, err, "resolve reference %s", name)
			assert.Len(t, gotDescriptorPaths, 1, "unexpected number of descriptors")
			gotDescriptor := gotDescriptorPaths[0].Descriptor()

			assert.Equal(t, test.result, gotDescriptor, "resolve reference should get same descriptor as original")

			err = engineExt.DeleteReference(t.Context(), name)
			require.NoErrorf(t, err, "delete reference %s", name)

			gotDescriptorPaths, err = engineExt.ResolveReference(t.Context(), name)
			require.NoErrorf(t, err, "resolve reference %s", name)
			assert.Empty(t, gotDescriptorPaths, "resolve reference after deleting should find no references")

			// DeleteReference is idempotent. It shouldn't cause an error.
			err = engineExt.DeleteReference(t.Context(), name)
			require.NoErrorf(t, err, "delete non-existent reference %s", name)
		})
	}
}

func TestEngineReferenceReadonly(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err, "open read-write engine")
	engineExt := NewEngine(engine)

	descMap := fakeSetupEngine(t, engineExt)
	assert.NotEmpty(t, descMap, "fakeSetupEngine descriptor map")

	err = engine.Close()
	require.NoError(t, err, "close read-write engine")

	for idx, test := range descMap {
		t.Run(fmt.Sprintf("Descriptor%.2d", idx+1), func(t *testing.T) {
			name := fmt.Sprintf("new_tag_%d", idx)

			engine, err := dir.Open(image)
			require.NoError(t, err, "open read-write engine")
			engineExt := NewEngine(engine)

			err = engineExt.UpdateReference(t.Context(), name, test.index)
			require.NoErrorf(t, err, "update reference %s", name)

			err = engine.Close()
			require.NoError(t, err, "close read-write engine")

			// make it readonly
			testhelpers.MakeReadOnly(t, image)
			defer testhelpers.MakeReadWrite(t, image)

			newEngine, err := dir.Open(image)
			require.NoError(t, err, "open read-only engine")
			newEngineExt := NewEngine(newEngine)

			gotDescriptorPaths, err := engineExt.ResolveReference(t.Context(), name)
			require.NoErrorf(t, err, "resolve reference %s", name)
			assert.Len(t, gotDescriptorPaths, 1, "unexpected number of descriptors")
			gotDescriptor := gotDescriptorPaths[0].Descriptor()

			assert.Equal(t, test.result, gotDescriptor, "resolve reference should get same descriptor as original")

			// Make sure that writing will FAIL.
			err = newEngineExt.UpdateReference(t.Context(), name+"new", test.index)
			assert.Errorf(t, err, "update reference %s for read-only image should fail", name) //nolint:testifylint // assert.*Error* makes more sense

			err = newEngine.Close()
			require.NoError(t, err, "close read-only engine")
		})
	}
}
