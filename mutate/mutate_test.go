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

package mutate

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas"
	casdir "github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"golang.org/x/net/context"
)

// These come from just running the code.
// TODO: Auto-generate these in a much more sane way.
const (
	expectedLayerDigest    = "sha256:96338a7c847bc582c82e4962a4285afcaf568e3913b0542b8745be27a418a806"
	expectedConfigDigest   = "sha256:ddcc2a93d5b0bcdcb571431c3607d84abe3752406f7c631a898340e6e7e61ed0"
	expectedManifestDigest = "sha256:0e8b342d2b01241b3f0197d0210ed5c0012d01817881defc1464e000f5b08f4d"
)

func setup(t *testing.T, dir string) (cas.Engine, ispec.Descriptor) {
	dir = filepath.Join(dir, "image")
	if err := casdir.Create(dir); err != nil {
		t.Fatal(err)
	}

	engine, err := casdir.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	engineExt := casext.NewEngine(engine)

	// Write a tar layer.
	var buffer bytes.Buffer
	tw := tar.NewWriter(&buffer)
	data := []byte("some contents")
	tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "test",
		Mode:     0644,
		Size:     int64(len(data)),
	})
	tw.Write(data)
	tw.Close()

	// Push the base layer.
	diffidDigester := cas.BlobAlgorithm.Digester()
	hashReader := io.TeeReader(&buffer, diffidDigester.Hash())
	layerDigest, layerSize, err := engine.PutBlob(context.Background(), hashReader)
	if err != nil {
		t.Fatal(err)
	}
	if layerDigest != expectedLayerDigest {
		t.Errorf("unexpected layerDigest: got %s, expected %s", layerDigest, expectedLayerDigest)
	}

	// Create a config.
	config := ispec.Image{
		Config: ispec.ImageConfig{
			User: "default:user",
		},
		RootFS: ispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{diffidDigester.Digest()},
		},
		History: []ispec.History{
			{EmptyLayer: false},
		},
	}

	configDigest, configSize, err := engineExt.PutBlobJSON(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if configDigest != expectedConfigDigest {
		t.Errorf("unexpected configDigest: got %s, expected %s", configDigest, expectedConfigDigest)
	}

	// Create the manifest.
	manifest := ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageLayerGzip,
				Digest:    layerDigest,
				Size:      layerSize,
			},
		},
	}

	manifestDigest, manifestSize, err := engineExt.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		t.Fatal(err)
	}
	if manifestDigest != expectedManifestDigest {
		t.Errorf("unexpected manifestDigest: got %s, expected %s", manifestDigest, expectedManifestDigest)
	}

	return engine, ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}
}

func TestMutateCache(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateBasic")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, fromDescriptor := setup(t, dir)
	defer engine.Close()

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	if err != nil {
		t.Fatal(err)
	}

	// Check that caching actually works.
	if err := mutator.cache(context.Background()); err != nil {
		t.Fatalf("unexpected error getting cache: %+v", err)
	}

	// Check manifest.
	if mutator.manifest.SchemaVersion != 2 {
		t.Errorf("manifest.SchemaVersion is not cached")
	}
	if mutator.manifest.Config.MediaType != ispec.MediaTypeImageConfig {
		t.Errorf("manifest.Config.MediaType is not cached")
	}
	if mutator.manifest.Config.Digest != expectedConfigDigest {
		t.Errorf("manifest.Config.Digest is not cached")
	}
	if len(mutator.manifest.Layers) != 1 {
		t.Errorf("manifest.Layers is not cached")
	}
	if mutator.manifest.Layers[0].MediaType != ispec.MediaTypeImageLayerGzip {
		t.Errorf("manifest.Layers is not cached")
	}
	if mutator.manifest.Layers[0].Digest != expectedLayerDigest {
		t.Errorf("manifest.Layers.Digest is not cached")
	}

	// Check config.
	if mutator.config.Config.User != "default:user" {
		t.Errorf("config.Config.User is not cached")
	}
	if mutator.config.RootFS.Type != "layers" {
		t.Errorf("config.RootFS.Type is not cached")
	}
	if len(mutator.config.RootFS.DiffIDs) != 1 {
		t.Errorf("config.RootFS.DiffIDs is not cached")
	}
	// TODO: Check Config.RootFS.DiffIDs.Digest.
	if len(mutator.config.History) != 1 {
		t.Errorf("config.History is not cached")
	}
	if mutator.config.History[0].EmptyLayer != false {
		t.Errorf("config.History[0].EmptyLayer is not cached")
	}
}

func TestMutateAdd(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateAdd")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, fromDescriptor := setup(t, dir)
	defer engine.Close()

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	if err != nil {
		t.Fatal(err)
	}

	// This isn't a valid image, but whatever.
	buffer := bytes.NewBufferString("contents")

	// Add a new layer.
	newLayerDesc, err := mutator.Add(context.Background(), buffer, &ispec.History{
		Comment: "new layer",
	})
	if err != nil {
		t.Fatalf("unexpected error adding layer: %+v", err)
	}

	newDescriptor, err := mutator.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error committing changes: %+v", err)
	}

	if newDescriptor.Descriptor().Digest == fromDescriptor.Digest {
		t.Fatalf("new and old descriptors are the same!")
	}

	mutator, err = New(engine, newDescriptor)
	if err != nil {
		t.Fatal(err)
	}

	// Cache the data to check it.
	if err := mutator.cache(context.Background()); err != nil {
		t.Fatalf("unexpected error getting cache: %+v", err)
	}

	// Check digests are different.
	if mutator.manifest.Config.Digest == expectedConfigDigest {
		t.Errorf("manifest.Config.Digest is the same!")
	}
	if mutator.manifest.Layers[0].Digest != expectedLayerDigest {
		t.Errorf("manifest.Layers[0].Digest is not the same!")
	}
	if mutator.manifest.Layers[1].Digest == expectedLayerDigest {
		t.Errorf("manifest.Layers[1].Digest is not the same!")
	}

	if mutator.manifest.Layers[1].Digest != newLayerDesc.Digest {
		t.Fatalf("unexpected digest for new layer: %v %v", mutator.manifest.Layers[1].Digest, newLayerDesc.Digest)
	}

	manifestFromFunction, err := mutator.Manifest(context.Background())
	if err != nil {
		t.Fatalf("unexpected error getting manifest: %+v", err)
	}

	if !reflect.DeepEqual(manifestFromFunction, *mutator.manifest) {
		t.Fatalf("mutator.Manifest() didn't return the cached manifest")
	}

	// Check layer was added.
	if len(mutator.manifest.Layers) != 2 {
		t.Errorf("manifest.Layers was not updated")
	}
	if mutator.manifest.Layers[1].MediaType != ispec.MediaTypeImageLayerGzip {
		t.Errorf("manifest.Layers[1].MediaType is the wrong value: %s", mutator.manifest.Layers[1].MediaType)
	}

	// Check config was also modified.
	if len(mutator.config.RootFS.DiffIDs) != 2 {
		t.Errorf("config.RootFS.DiffIDs was not updated")
	}

	// Check history.
	if len(mutator.config.History) != 2 {
		t.Errorf("config.History was not updated")
	}
	if mutator.config.History[1].EmptyLayer != false {
		t.Errorf("config.History[1].EmptyLayer was not set")
	}
	if mutator.config.History[1].Comment != "new layer" {
		t.Errorf("config.History[1].Comment was not set")
	}
}

func TestMutateAddNonDistributable(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateAddNonDistributable")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, fromDescriptor := setup(t, dir)
	defer engine.Close()

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	if err != nil {
		t.Fatal(err)
	}

	// This isn't a valid image, but whatever.
	buffer := bytes.NewBufferString("contents")

	// Add a new layer.
	if err := mutator.AddNonDistributable(context.Background(), buffer, &ispec.History{
		Comment: "new layer",
	}); err != nil {
		t.Fatalf("unexpected error adding layer: %+v", err)
	}

	newDescriptor, err := mutator.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error committing changes: %+v", err)
	}

	if newDescriptor.Descriptor().Digest == fromDescriptor.Digest {
		t.Fatalf("new and old descriptors are the same!")
	}

	mutator, err = New(engine, newDescriptor)
	if err != nil {
		t.Fatal(err)
	}

	// Cache the data to check it.
	if err := mutator.cache(context.Background()); err != nil {
		t.Fatalf("unexpected error getting cache: %+v", err)
	}

	// Check digests are different.
	if mutator.manifest.Config.Digest == expectedConfigDigest {
		t.Errorf("manifest.Config.Digest is the same!")
	}
	if mutator.manifest.Layers[0].Digest != expectedLayerDigest {
		t.Errorf("manifest.Layers[0].Digest is not the same!")
	}
	if mutator.manifest.Layers[1].Digest == expectedLayerDigest {
		t.Errorf("manifest.Layers[1].Digest is not the same!")
	}

	// Check layer was added.
	if len(mutator.manifest.Layers) != 2 {
		t.Errorf("manifest.Layers was not updated")
	}
	if mutator.manifest.Layers[1].MediaType != ispec.MediaTypeImageLayerNonDistributableGzip {
		t.Errorf("manifest.Layers[1].MediaType is the wrong value: %s", mutator.manifest.Layers[1].MediaType)
	}

	// Check config was also modified.
	if len(mutator.config.RootFS.DiffIDs) != 2 {
		t.Errorf("config.RootFS.DiffIDs was not updated")
	}

	// Check history.
	if len(mutator.config.History) != 2 {
		t.Errorf("config.History was not updated")
	}
	if mutator.config.History[1].EmptyLayer != false {
		t.Errorf("config.History[1].EmptyLayer was not set")
	}
	if mutator.config.History[1].Comment != "new layer" {
		t.Errorf("config.History[1].Comment was not set")
	}
}

func TestMutateSet(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateSet")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, fromDescriptor := setup(t, dir)
	defer engine.Close()

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	if err != nil {
		t.Fatal(err)
	}

	// Change the config
	if err := mutator.Set(context.Background(), ispec.ImageConfig{
		User: "changed:user",
	}, Meta{}, nil, &ispec.History{
		Comment: "another layer",
	}); err != nil {
		t.Fatalf("unexpected error adding layer: %+v", err)
	}

	newDescriptor, err := mutator.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error committing changes: %+v", err)
	}

	if newDescriptor.Descriptor().Digest == fromDescriptor.Digest {
		t.Fatalf("new and old descriptors are the same!")
	}

	mutator, err = New(engine, newDescriptor)
	if err != nil {
		t.Fatal(err)
	}

	// Cache the data to check it.
	if err := mutator.cache(context.Background()); err != nil {
		t.Fatalf("unexpected error getting cache: %+v", err)
	}

	// Check digests are different.
	if mutator.manifest.Config.Digest == expectedConfigDigest {
		t.Errorf("manifest.Config.Digest is the same!")
	}

	// Check layer was not added.
	if len(mutator.manifest.Layers) != 1 {
		t.Errorf("manifest.Layers was updated")
	}

	// Check config was also modified.
	if len(mutator.config.RootFS.DiffIDs) != 1 {
		t.Errorf("config.RootFS.DiffIDs was updated")
	}
	if mutator.config.Config.User != "changed:user" {
		t.Errorf("config.Config.USer was not updated! expected changed:user, got %s", mutator.config.Config.User)
	}

	// Check history.
	if len(mutator.config.History) != 2 {
		t.Errorf("config.History was not updated")
	}
	if mutator.config.History[1].EmptyLayer != true {
		t.Errorf("config.History[1].EmptyLayer was not set")
	}
	if mutator.config.History[1].Comment != "another layer" {
		t.Errorf("config.History[1].Comment was not set")
	}
}

func TestMutateSetNoHistory(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateSetNoHistory")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, fromDescriptor := setup(t, dir)
	defer engine.Close()

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	if err != nil {
		t.Fatal(err)
	}

	// Change the config
	if err := mutator.Set(context.Background(), ispec.ImageConfig{
		User: "changed:user",
	}, Meta{}, nil, nil); err != nil {
		t.Fatalf("unexpected error adding layer: %+v", err)
	}

	newDescriptor, err := mutator.Commit(context.Background())
	if err != nil {
		t.Fatalf("unexpected error committing changes: %+v", err)
	}

	if newDescriptor.Descriptor().Digest == fromDescriptor.Digest {
		t.Fatalf("new and old descriptors are the same!")
	}

	mutator, err = New(engine, newDescriptor)
	if err != nil {
		t.Fatal(err)
	}

	// Cache the data to check it.
	if err := mutator.cache(context.Background()); err != nil {
		t.Fatalf("unexpected error getting cache: %+v", err)
	}

	// Check digests are different.
	if mutator.manifest.Config.Digest == expectedConfigDigest {
		t.Errorf("manifest.Config.Digest is the same!")
	}

	// Check layer was not added.
	if len(mutator.manifest.Layers) != 1 {
		t.Errorf("manifest.Layers was updated")
	}

	// Check config was also modified.
	if len(mutator.config.RootFS.DiffIDs) != 1 {
		t.Errorf("config.RootFS.DiffIDs was updated")
	}
	if mutator.config.Config.User != "changed:user" {
		t.Errorf("config.Config.USer was not updated! expected changed:user, got %s", mutator.config.Config.User)
	}

	// Check history.
	if len(mutator.config.History) == 2 {
		t.Errorf("config.History was changed")
	}
}

func walkDescriptorRoot(ctx context.Context, engine casext.Engine, root ispec.Descriptor) (casext.DescriptorPath, error) {
	var foundPath *casext.DescriptorPath

	if err := engine.Walk(ctx, root, func(descriptorPath casext.DescriptorPath) error {
		// Just find the first manifest.
		if descriptorPath.Descriptor().MediaType == ispec.MediaTypeImageManifest {
			foundPath = &descriptorPath
		}
		return nil
	}); err != nil {
		return casext.DescriptorPath{}, err
	}

	if foundPath == nil {
		return casext.DescriptorPath{}, fmt.Errorf("count not find manifest from %s", root.Digest)
	}
	return *foundPath, nil
}

func TestMutatePath(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestMutateSet")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	engine, manifestDescriptor := setup(t, dir)
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	// Create some additional structure.
	expectedPaths := []casext.DescriptorPath{
		{Walk: []ispec.Descriptor{manifestDescriptor}},
	}

	// Build on top of the previous blob.
	for idx := 1; idx < 32; idx++ {
		oldPath := expectedPaths[idx-1]

		// Create an Index that points to the old root.
		newRoot := ispec.Index{
			Manifests: []ispec.Descriptor{
				oldPath.Root(),
			},
		}
		newRootDigest, newRootSize, err := engineExt.PutBlobJSON(context.Background(), newRoot)
		if err != nil {
			t.Fatalf("failed to put blob json newroot: %+v", err)
		}
		newRootDescriptor := ispec.Descriptor{
			MediaType: ispec.MediaTypeImageIndex,
			Digest:    newRootDigest,
			Size:      newRootSize,
		}

		// Create a new path.
		var newPath casext.DescriptorPath
		newPath.Walk = append([]ispec.Descriptor{newRootDescriptor}, oldPath.Walk...)
		expectedPaths = append(expectedPaths, newPath)
	}

	// Mutate each one.
	for idx, path := range expectedPaths {
		mutator, err := New(engine, path)
		if err != nil {
			t.Fatal(err)
		}

		// Change the config in some minor way.
		meta, err := mutator.Meta(context.Background())
		if err != nil {
			t.Fatalf("%d: unexpected error getting meta: %+v", idx, err)
		}
		config, err := mutator.Config(context.Background())
		if err != nil {
			t.Fatalf("%d: unexpected error getting config: %+v", idx, err)
		}

		// Change the label.
		label := fmt.Sprintf("TestMutateSet+%d", idx)
		if config.Labels == nil {
			config.Labels = map[string]string{}
		}
		config.Labels["org.opensuse.testidx"] = label

		// Update it.
		if err := mutator.Set(context.Background(), config, meta, nil, &ispec.History{
			Comment: "change label " + label,
		}); err != nil {
			t.Fatalf("%d: unexpected error modifying config: %+v", idx, err)
		}

		// Commit.
		newPath, err := mutator.Commit(context.Background())
		if err != nil {
			t.Fatalf("%d: unexpected error committing: %+v", idx, err)
		}

		// Make sure that the paths are the same length but have different
		// digests.
		if len(newPath.Walk) != len(path.Walk) {
			t.Errorf("%d: new path was a different length than the old one: %v != %v", idx, len(newPath.Walk), len(path.Walk))
		} else if reflect.DeepEqual(newPath, path) {
			t.Errorf("%d: new path was identical to old one: %v", idx, path)
		} else {
			for i := 0; i < len(path.Walk); i++ {
				if path.Walk[i].Digest == newPath.Walk[i].Digest {
					t.Errorf("%d: path[%d].Digest = newPath[%d].Digest: %v = %v", idx, i, i, path.Walk[i].Digest, newPath.Walk[i].Digest)
				}
				if path.Walk[i].MediaType != newPath.Walk[i].MediaType {
					t.Errorf("%d: path[%d].MediaType != newPath[%d].MediaType: %v != %v", idx, i, i, path.Walk[i].MediaType, newPath.Walk[i].MediaType)
				}
				if !reflect.DeepEqual(path.Walk[i].Annotations, newPath.Walk[i].Annotations) {
					t.Errorf("%d: path[%d].Annotations != newPath[%d].Annotations: %v != %v", idx, i, i, path.Walk[i].Annotations, newPath.Walk[i].Annotations)
				}
			}
		}

		// Emulate a reference resolution with walkDescriptorRoot.
		walkPath, err := walkDescriptorRoot(context.Background(), engineExt, newPath.Root())
		if err != nil {
			t.Errorf("%d: unexpected error with walkPath %v", idx, err)
		} else if !reflect.DeepEqual(newPath, walkPath) {
			t.Errorf("%d: walkDescriptorRoot didn't give the same path: expected %v got %v", idx, newPath, walkPath)
		}

		// Make sure the old path still exists (not necessary to be honest).
		oldWalkPath, err := walkDescriptorRoot(context.Background(), engineExt, path.Root())
		if err != nil {
			t.Errorf("%d: unexpected error with oldWalkPath %v", idx, err)
		} else if !reflect.DeepEqual(oldWalkPath, path) {
			t.Errorf("%d: walkDescriptorRoot didn't give the same old path: expected %v got %v", idx, newPath, walkPath)
		}
	}
}
