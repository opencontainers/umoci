/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/opencontainers/go-digest"
	ispecs "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context"
)

type descriptorMap struct {
	index  ispec.Descriptor
	result ispec.Descriptor
}

func randomTarData(t *testing.T, tw *tar.Writer) error {
	// Add some files with random contents and random names.
	for n := 0; n < 32; n++ {
		size := rand.Intn(512 * 1024)

		if err := tw.WriteHeader(&tar.Header{
			Name:     randomString(16),
			Mode:     0755,
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
// types. The returned
func fakeSetupEngine(t *testing.T, engineExt Engine) ([]descriptorMap, error) {
	ctx := context.Background()
	mapping := []descriptorMap{}

	// Add some "normal" images that contain some layers and also have some
	// index indirects. The multiple layers makes sure that we don't break the
	// multi-level resolution.
	// XXX: In future we'll have to make tests for platform matching.
	for k := 0; k < 5; k++ {
		n := 3
		name := fmt.Sprintf("img_%d", k)

		layerData := make([]bytes.Buffer, n)

		// Generate layer data.
		for idx := range layerData {
			tw := tar.NewWriter(&layerData[idx])
			if err := randomTarData(t, tw); err != nil {
				t.Fatalf("%s: error generating layer%d data: %+v", name, idx, err)
			}
			tw.Close()
		}

		// Insert all of the layers.
		layerDescriptors := make([]ispec.Descriptor, n)
		for idx, layer := range layerData {
			digest, size, err := engineExt.PutBlob(ctx, &layer)
			if err != nil {
				t.Fatalf("%s: error putting layer%d blob: %+v", name, idx, err)
			}
			layerDescriptors[idx] = ispec.Descriptor{
				MediaType: ispec.MediaTypeImageLayer,
				Digest:    digest,
				Size:      size,
			}
		}

		// Create our config and insert it.
		created := time.Now()
		configDigest, configSize, err := engineExt.PutBlobJSON(ctx, ispec.Image{
			Created:      &created,
			Author:       "Jane Author <janesmith@example.com>",
			Architecture: runtime.GOARCH,
			OS:           runtime.GOOS,
			RootFS: ispec.RootFS{
				Type: "unknown",
			},
		})
		if err != nil {
			t.Fatalf("%s: error putting config blob: %+v", name, err)
		}
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
			Config: configDescriptor,
		}
		for _, layer := range layerDescriptors {
			manifest.Layers = append(manifest.Layers, layer)
		}

		manifestDigest, manifestSize, err := engineExt.PutBlobJSON(ctx, manifest)
		if err != nil {
			t.Fatalf("%s: error putting manifest blob: %+v", name, err)
		}
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
		for i := 0; i < k; i++ {
			newIndex := ispec.Index{
				Versioned: ispecs.Versioned{
					SchemaVersion: 2,
				},
				Manifests: []ispec.Descriptor{indexDescriptor},
			}
			indexDigest, indexSize, err := engineExt.PutBlobJSON(ctx, newIndex)
			if err != nil {
				t.Fatalf("%s: error putting index-%d blob: %+v", name, i, err)
			}
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

	// Add some blobs that have unknown mediaTypes. This is loosely based on
	// the previous section.
	for k := 0; k < 5; k++ {
		name := fmt.Sprintf("img_%d", k)

		type fakeManifest struct {
			Descriptor ispec.Descriptor `json:"descriptor"`
			Data       []byte           `json:"data"`
		}

		manifestDigest, manifestSize, err := engineExt.PutBlobJSON(ctx, fakeManifest{
			Descriptor: ispec.Descriptor{
				MediaType: "org.opensuse.fake.data",
				Digest:    digest.SHA256.FromString("Hello, world!"),
				Size:      0,
			},
			Data: []byte("Hello, world!"),
		})
		if err != nil {
			t.Fatalf("%s: error putting manifest blob: %+v", name, err)
		}
		manifestDescriptor := ispec.Descriptor{
			MediaType: "org.opensuse.fake.manifest",
			Digest:    manifestDigest,
			Size:      manifestSize,
			Annotations: map[string]string{
				"name": name,
			},
		}

		// Add extra index layers.
		indexDescriptor := manifestDescriptor
		for i := 0; i < k; i++ {
			newIndex := ispec.Index{
				Versioned: ispecs.Versioned{
					SchemaVersion: 2,
				},
				Manifests: []ispec.Descriptor{indexDescriptor},
			}
			indexDigest, indexSize, err := engineExt.PutBlobJSON(ctx, newIndex)
			if err != nil {
				t.Fatalf("%s: error putting index-%d blob: %+v", name, i, err)
			}
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

	return mapping, nil
}

func TestEngineReference(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReference")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)
	defer engine.Close()

	descMap, err := fakeSetupEngine(t, engineExt)
	if err != nil {
		t.Fatalf("unexpected error doing fakeSetupEngine: %+v", err)
	}

	for idx, test := range descMap {
		name := fmt.Sprintf("new_tag_%d", idx)

		if err := engineExt.UpdateReference(ctx, name, test.index); err != nil {
			t.Errorf("UpdateReference: unexpected error: %+v", err)
		}

		gotDescriptorPaths, err := engineExt.ResolveReference(ctx, name)
		if err != nil {
			t.Errorf("ResolveReference: unexpected error: %+v", err)
		}
		if len(gotDescriptorPaths) != 1 {
			t.Errorf("ResolveReference: expected %q to get %d descriptors, got %d: %+v", name, 1, len(gotDescriptorPaths), gotDescriptorPaths)
			continue
		}
		gotDescriptor := gotDescriptorPaths[0].Descriptor()

		if !reflect.DeepEqual(test.result, gotDescriptor) {
			t.Errorf("ResolveReference: got different descriptor to original: expected=%v got=%v", test.result, gotDescriptor)
		}

		if err := engineExt.DeleteReference(ctx, name); err != nil {
			t.Errorf("DeleteReference: unexpected error: %+v", err)
		}

		if gotDescriptorPaths, err := engineExt.ResolveReference(ctx, name); err != nil {
			t.Errorf("ResolveReference: unexpected error: %+v", err)
		} else if len(gotDescriptorPaths) > 0 {
			t.Errorf("ResolveReference: still got reference descriptors after DeleteReference!")
		}

		// DeleteBlob is idempotent. It shouldn't cause an error.
		if err := engineExt.DeleteReference(ctx, name); err != nil {
			t.Errorf("DeleteReference: unexpected error on double-delete: %+v", err)
		}
	}
}

func TestEngineReferenceReadonly(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReferenceReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)

	descMap, err := fakeSetupEngine(t, engineExt)
	if err != nil {
		t.Fatalf("unexpected error doing fakeSetupEngine: %+v", err)
	}

	if err := engine.Close(); err != nil {
		t.Fatalf("unexpected error closing image: %+v", err)
	}

	for idx, test := range descMap {
		name := fmt.Sprintf("new_tag_%d", idx)

		engine, err := dir.Open(image)
		if err != nil {
			t.Fatalf("unexpected error opening image: %+v", err)
		}
		engineExt := NewEngine(engine)

		if err := engineExt.UpdateReference(ctx, name, test.index); err != nil {
			t.Errorf("UpdateReference: unexpected error: %+v", err)
		}

		if err := engine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered: %+v", err)
		}

		// make it readonly
		readonly(t, image)

		newEngine, err := dir.Open(image)
		if err != nil {
			t.Errorf("unexpected error opening ro image: %+v", err)
		}
		newEngineExt := NewEngine(newEngine)

		gotDescriptorPaths, err := newEngineExt.ResolveReference(ctx, name)
		if err != nil {
			t.Errorf("ResolveReference: unexpected error: %+v", err)
		}
		if len(gotDescriptorPaths) != 1 {
			t.Errorf("ResolveReference: expected to get %d descriptors, got %d: %+v", 1, len(gotDescriptorPaths), gotDescriptorPaths)
		}
		gotDescriptor := gotDescriptorPaths[0].Descriptor()

		if !reflect.DeepEqual(test.result, gotDescriptor) {
			t.Errorf("ResolveReference: got different descriptor to original: expected=%v got=%v", test.result, gotDescriptor)
		}

		// Make sure that writing will FAIL.
		if err := newEngineExt.UpdateReference(ctx, name+"new", test.index); err == nil {
			t.Errorf("UpdateReference: expected error on ro image!")
		}

		if err := newEngine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered on ro: %+v", err)
		}

		// make it readwrite again.
		readwrite(t, image)
	}
}
