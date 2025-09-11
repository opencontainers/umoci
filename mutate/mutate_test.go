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

package mutate

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"path/filepath"
	"testing"

	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/oci/cas"
	casdir "github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
)

// These come from just running the code.
// TODO: Auto-generate these in a much more sane way.
const (
	expectedLayerDigest digest.Digest = "sha256:53d15a54123290a2316508a4fba65f1b568d34fcf2b88e69adcef02632e33ad8"
	expectedLayerDiffID digest.Digest = "sha256:53d15a54123290a2316508a4fba65f1b568d34fcf2b88e69adcef02632e33ad8"
	expectedLayerSize   int64         = 16778752

	expectedConfigDigest digest.Digest = "sha256:84207a85750d5d08c3489191c692ff2665e00b5c03a5730d9f2139b15d42aac2"
	expectedConfigSize   int64         = 190

	expectedManifestDigest digest.Digest = "sha256:132e9c5067776320f2dc9451f2ae74330fffe31e6cf7fd88fd2a7441a57209a7"
	expectedManifestSize   int64         = 407
)

func setup(t *testing.T) (cas.Engine, ispec.Descriptor) {
	dir := t.TempDir()
	dir = filepath.Join(dir, "image")
	err := casdir.Create(dir)
	require.NoError(t, err)

	engine, err := casdir.Open(dir)
	require.NoError(t, err)
	engineExt := casext.NewEngine(engine)

	// We need to have a large enough file (16MiB) to make sure we hit the gzip
	// buffer size (which can affect the compressed output).
	randSrc := rand.New(rand.NewSource(19970325))
	dataSize := int64(1 << 24)

	// Write a tar layer.
	var buffer bytes.Buffer
	tw := tar.NewWriter(&buffer)

	// Header.
	err = tw.WriteHeader(&tar.Header{
		Typeflag: tar.TypeReg,
		Name:     "test",
		Mode:     0o644,
		Size:     dataSize,
	})
	require.NoError(t, err, "write header")

	// File data.
	n, err := io.CopyN(tw, randSrc, dataSize)
	require.NoError(t, err, "write file data")
	require.Equal(t, dataSize, n, "written file data should match expected data size")

	_ = tw.Close()

	// Push the base layer.
	diffidDigester := cas.BlobAlgorithm.Digester()
	hashReader := io.TeeReader(&buffer, diffidDigester.Hash())
	layerDigest, layerSize, err := engine.PutBlob(t.Context(), hashReader)
	require.NoError(t, err)
	assert.Equal(t, expectedLayerDigest, layerDigest, "unexpected layer digest")
	assert.Equal(t, expectedLayerSize, layerSize, "unexpected layer size")

	layerDiffID := diffidDigester.Digest()
	assert.Equal(t, expectedLayerDiffID, layerDiffID, "unexpected layer diffid")

	// Create a config.
	config := ispec.Image{
		Config: ispec.ImageConfig{
			User: "default:user",
		},
		RootFS: ispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{layerDiffID},
		},
		History: []ispec.History{
			{EmptyLayer: false},
		},
	}

	configDigest, configSize, err := engineExt.PutBlobJSON(t.Context(), config)
	require.NoError(t, err)
	assert.Equal(t, expectedConfigDigest, configDigest, "unexpected config digest")
	assert.Equal(t, expectedConfigSize, configSize, "unexpected config size")

	// Create the manifest.
	manifest := ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageManifest,
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

	manifestDigest, manifestSize, err := engineExt.PutBlobJSON(t.Context(), manifest)
	require.NoError(t, err)
	assert.Equal(t, expectedManifestDigest, manifestDigest, "unexpected manifest digest")
	assert.Equal(t, expectedManifestSize, manifestSize, "unexpected manifest size")

	return engine, ispec.Descriptor{
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}
}

func TestMutateCache(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// Check that caching actually works.
	err = mutator.cache(t.Context())
	require.NoError(t, err, "getting cache")

	// Check manifest.
	assert.Equal(t, ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageManifest,
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageConfig,
			Digest:    expectedConfigDigest,
			Size:      expectedConfigSize,
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageLayerGzip,
				Digest:    expectedLayerDigest,
				Size:      expectedLayerSize,
			},
		},
	}, *mutator.manifest, "manifest not cached")

	// Check config.
	assert.Equal(t, ispec.Image{
		Config: ispec.ImageConfig{
			User: "default:user",
		},
		RootFS: ispec.RootFS{
			Type:    "layers",
			DiffIDs: []digest.Digest{expectedLayerDiffID},
		},
		History: []ispec.History{
			{EmptyLayer: false},
		},
	}, *mutator.config, "config not cached")
}

func TestMutateAdd(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// This isn't a valid image, but whatever.
	buffer := bytes.NewBufferString("contents")
	bufferSize := buffer.Len()

	// Add a new layer.
	annotations := map[string]string{"hello": "world"}
	newLayerHist := ispec.History{
		EmptyLayer: false,
		Comment:    "new layer",
	}
	newLayerDesc, err := mutator.Add(t.Context(), ispec.MediaTypeImageLayer, buffer, &newLayerHist, blobcompress.Gzip, annotations)
	require.NoError(t, err, "add layer")

	newDescriptor, err := mutator.Commit(t.Context())
	require.NoError(t, err, "commit changes")
	assert.NotEqual(t, fromDescriptor.Digest, newDescriptor.Descriptor().Digest, "new and old descriptors should be different")

	mutator, err = New(engine, newDescriptor)
	require.NoError(t, err)

	// Cache the data to check it.
	err = mutator.cache(t.Context())
	require.NoError(t, err, "get cache")

	// Check digests for new config and layer are different.
	assert.NotEqual(t, expectedConfigDigest, mutator.manifest.Config.Digest, "new config should have a different digest")
	assert.Equal(t, expectedLayerDigest, mutator.manifest.Layers[0].Digest, "old layer should have same digest")
	assert.NotEqual(t, expectedLayerDigest, mutator.manifest.Layers[1].Digest, "new layer should have a different digest")

	assert.Equal(t, map[string]string{
		"hello":                             "world",
		UmociUncompressedBlobSizeAnnotation: fmt.Sprintf("%d", bufferSize),
	}, mutator.manifest.Layers[1].Annotations, "new layer annotations")

	assert.Equal(t, newLayerDesc, mutator.manifest.Layers[1], "new layer descriptor should match the one returned by mutator")

	manifestFromFunction, err := mutator.Manifest(t.Context())
	require.NoError(t, err, "get manifest")
	assert.Equal(t, *mutator.manifest, manifestFromFunction, "mutator.Manifest() should return cached manifest")

	// Check layer was added.
	assert.Len(t, mutator.manifest.Layers, 2, "manifest.Layers should include new layer")
	assert.Equal(t, ispec.MediaTypeImageLayerGzip, mutator.manifest.Layers[1].MediaType, "new layer should have the right media-type")

	// Check config was also modified.
	assert.Len(t, mutator.config.RootFS.DiffIDs, 2, "config.RootFS.DiffIDs should include new layer")
	assert.NotEqual(t, expectedLayerDiffID, mutator.config.RootFS.DiffIDs[1], "new layer should have a different diffid")

	// Check history.
	assert.Len(t, mutator.config.History, 2, "config.History should include new layer")
	assert.Equal(t, newLayerHist, mutator.config.History[1], "new layer history should match specified value")
}

func testMutateAddCompression(t *testing.T, mutator *Mutator, mediaType string, addCompressAlgo, expectedCompressAlgo blobcompress.Algorithm) {
	// This test doesn't care about whether the layer is real.
	fakeLayerData := `fake tar archive`
	fakeLayerTar := bytes.NewBufferString(fakeLayerData)

	newLayerDescriptor, err := mutator.Add(
		t.Context(),
		mediaType,
		fakeLayerTar,
		&ispec.History{Comment: "fake layer"},
		addCompressAlgo,
		nil,
	)
	require.NoError(t, err)

	expectedMediaType := mediaType
	if suffix := expectedCompressAlgo.MediaTypeSuffix(); suffix != "" {
		expectedMediaType += "+" + suffix
	}

	usedCompressName := "auto"
	if addCompressAlgo != nil {
		if suffix := addCompressAlgo.MediaTypeSuffix(); suffix != "" {
			usedCompressName = suffix
		} else {
			usedCompressName = "plain"
		}
	}

	// The media-type should be what we expected.
	assert.Equalf(t, expectedMediaType, newLayerDescriptor.MediaType, "unexpected media type of new layer with compression algo %q", usedCompressName)

	// Double-check that the blob actually used the expected compression
	// algorithm.
	layerRdr, err := mutator.engine.GetVerifiedBlob(t.Context(), newLayerDescriptor)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, layerRdr.Close())
	}()

	plainLayerRdr, err := expectedCompressAlgo.Decompress(layerRdr)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, plainLayerRdr.Close())
	}()

	plainLayerData, err := io.ReadAll(plainLayerRdr)
	require.NoError(t, err)

	assert.Equal(t, fakeLayerData, string(plainLayerData), "layer data should match after round-trip")
}

func TestMutateAddCompression(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// Test that explicitly setting the compression does what you expect:
	for _, test := range []struct {
		name                  string
		useAlgo, expectedAlgo blobcompress.Algorithm
	}{
		// The default with no previous layers should be gzip.
		{"DefaultGzip", nil, blobcompress.Gzip},
		// Explicitly setting the algorithms.
		{"Noop", blobcompress.Noop, blobcompress.Noop},
		{"Gzip", blobcompress.Gzip, blobcompress.Gzip},
		{"Zstd", blobcompress.Zstd, blobcompress.Zstd},
	} {
		t.Run(test.name, func(t *testing.T) {
			testMutateAddCompression(t, mutator, "vendor/TESTING-umoci-fake-layer", test.useAlgo, test.expectedAlgo)
		})
	}

	// Check that the auto-selection of compression works properly.
	t.Run("Auto", func(t *testing.T) {
		for i, test := range []struct {
			name                  string
			useAlgo, expectedAlgo blobcompress.Algorithm
		}{
			// Basic inheritance for zstd.
			{"ExplicitZstd", blobcompress.Zstd, blobcompress.Zstd},
			{"AutoZstd", nil, blobcompress.Zstd},
			// Inheritance skips noop.
			{"ExplicitNoop", blobcompress.Noop, blobcompress.Noop},
			{"AutoZstd-SkipNoop", nil, blobcompress.Zstd},
			// Basic inheritance for gzip.
			{"ExplicitGzip", blobcompress.Gzip, blobcompress.Gzip},
			{"AutoGzip", nil, blobcompress.Gzip},
			// Inheritance skips noop.
			{"ExplicitNoop", blobcompress.Noop, blobcompress.Noop},
			{"AutoGzip-SkipNoop", nil, blobcompress.Gzip},
		} {
			t.Run(fmt.Sprintf("Step%d-%s", i, test.name), func(t *testing.T) {
				testMutateAddCompression(t, mutator, "vendor/TESTING-umoci-fake-layer", test.useAlgo, test.expectedAlgo)
			})
		}
	})
}

func TestMutateAddExisting(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// This isn't a valid image, but whatever.
	buffer := bytes.NewBufferString("contents")

	// Add a new layer.
	_, err = mutator.Add(t.Context(), ispec.MediaTypeImageLayer, buffer, &ispec.History{
		Comment: "new layer",
	}, blobcompress.Gzip, nil)
	require.NoError(t, err, "add layer")

	newDescriptor, err := mutator.Commit(t.Context())
	require.NoError(t, err, "commit change")

	mutator, err = New(engine, newDescriptor)
	require.NoError(t, err)

	// add the layer again; first loading the cache so we can use the existing one
	err = mutator.cache(t.Context())
	require.NoError(t, err, "get cache")

	diffID := mutator.config.RootFS.DiffIDs[len(mutator.config.RootFS.DiffIDs)-1]
	history := ispec.History{Comment: "hello world"}
	layerDesc := mutator.manifest.Layers[len(mutator.manifest.Layers)-1]
	err = mutator.AddExisting(t.Context(), layerDesc, &history, diffID)
	require.NoError(t, err, "add existing layer")

	_, err = mutator.Commit(t.Context())
	require.NoError(t, err, "commit change")

	manifestFromFunction, err := mutator.Manifest(t.Context())
	require.NoError(t, err, "get manifest")
	assert.Equal(t, *mutator.manifest, manifestFromFunction, "mutator.Manifest() should return cached manifest")

	require.Len(t, mutator.manifest.Layers, 3, "manifest should include new layers")
	assert.Equal(t, layerDesc, mutator.manifest.Layers[1], "new layer descriptor")
	assert.Equal(t, layerDesc, mutator.manifest.Layers[2], "re-used new layer descriptor")

	require.Len(t, mutator.config.RootFS.DiffIDs, 3, "config should include new layer diffids")
	assert.Equal(t, diffID, mutator.config.RootFS.DiffIDs[1], "new layer diffid")
	assert.Equal(t, diffID, mutator.config.RootFS.DiffIDs[2], "re-used new layer diffid")
}

func TestMutateSet(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// Change the config
	err = mutator.Set(t.Context(), ispec.ImageConfig{
		User: "changed:user",
	}, Meta{}, nil, &ispec.History{
		Comment: "another layer",
	})
	require.NoError(t, err, "set config")

	newDescriptor, err := mutator.Commit(t.Context())
	require.NoError(t, err, "commit change")
	assert.NotEqual(t, fromDescriptor.Digest, newDescriptor.Descriptor().Digest, "new manifest descriptor digest should be different")

	mutator, err = New(engine, newDescriptor)
	require.NoError(t, err)

	// Cache the data to check it.
	err = mutator.cache(t.Context())
	require.NoError(t, err, "get cache")

	// Check digests are different.
	assert.NotEqual(t, expectedConfigDigest, mutator.manifest.Config.Digest, "config digest should be different")

	// Check a layer was not added.
	assert.Len(t, mutator.manifest.Layers, 1, "config change shouldn't affect layer digests")
	assert.Len(t, mutator.config.RootFS.DiffIDs, 1, "config change shouldn't affect layer diffids")

	// Check config was also modified.
	assert.Equal(t, "changed:user", mutator.config.Config.User, "config user wasn't updated")

	// Check history.
	assert.Len(t, mutator.config.History, 2, "history should have new entry")
	assert.Equal(t, ispec.History{
		EmptyLayer: true,
		Comment:    "another layer",
	}, mutator.config.History[1], "config history entry")
}

func TestMutateSetNoHistory(t *testing.T) {
	engine, fromDescriptor := setup(t)
	defer engine.Close() //nolint:errcheck

	mutator, err := New(engine, casext.DescriptorPath{Walk: []ispec.Descriptor{fromDescriptor}})
	require.NoError(t, err)

	// Change the config
	err = mutator.Set(t.Context(), ispec.ImageConfig{
		User: "changed:user",
	}, Meta{}, nil, nil)
	require.NoError(t, err, "set config")

	newDescriptor, err := mutator.Commit(t.Context())
	require.NoError(t, err, "commit change")
	assert.NotEqual(t, fromDescriptor.Digest, newDescriptor.Descriptor().Digest, "new manifest descriptor digest should be different")

	mutator, err = New(engine, newDescriptor)
	require.NoError(t, err)

	// Cache the data to check it.
	err = mutator.cache(t.Context())
	require.NoError(t, err, "get cache")

	// Check digests are different.
	assert.NotEqual(t, expectedConfigDigest, mutator.manifest.Config.Digest, "config digest should be different")

	// Check a layer was not added.
	assert.Len(t, mutator.manifest.Layers, 1, "config change shouldn't affect layer digests")
	assert.Len(t, mutator.config.RootFS.DiffIDs, 1, "config change shouldn't affect layer diffids")

	// Check config was also modified.
	assert.Equal(t, "changed:user", mutator.config.Config.User, "config user wasn't updated")

	// Check history.
	assert.Len(t, mutator.config.History, 1, "history should not have new entry")
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
	engine, manifestDescriptor := setup(t)
	engineExt := casext.NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	// Create some additional structure.
	expectedPaths := []casext.DescriptorPath{
		{Walk: []ispec.Descriptor{manifestDescriptor}},
	}

	// Build on top of the previous blob.
	for idx := 1; idx < 32; idx++ {
		oldPath := expectedPaths[idx-1]

		// Create an Index that points to the old root.
		newRoot := ispec.Index{
			MediaType: ispec.MediaTypeImageIndex,
			Manifests: []ispec.Descriptor{
				oldPath.Root(),
			},
		}
		newRootDigest, newRootSize, err := engineExt.PutBlobJSON(t.Context(), newRoot)
		require.NoError(t, err, "failed to put new root blob json")
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
		require.NoError(t, err)

		// Change the config in some minor way.
		meta, err := mutator.Meta(t.Context())
		require.NoErrorf(t, err, "getting %d meta", idx)

		config, err := mutator.Config(t.Context())
		require.NoErrorf(t, err, "getting %d config", idx)

		// Change the label.
		label := fmt.Sprintf("TestMutateSet+%d", idx)
		if config.Config.Labels == nil {
			config.Config.Labels = map[string]string{}
		}
		config.Config.Labels["org.opensuse.testidx"] = label

		// Update it.
		err = mutator.Set(t.Context(), config.Config, meta, nil, &ispec.History{
			Comment: "change label " + label,
		})
		require.NoErrorf(t, err, "setting %d config", idx)

		// Commit.
		newPath, err := mutator.Commit(t.Context())
		require.NoErrorf(t, err, "commit change %d", idx)

		// Make sure that the paths are the same length but have different
		// digests.
		if assert.Len(t, newPath.Walk, len(path.Walk), "new path should be the same length as the old one") {
			assert.NotEqual(t, newPath, path, "new path should be different to the old one")
			for i := 0; i < len(path.Walk); i++ {
				assert.Equalf(t, path.Walk[i].MediaType, newPath.Walk[i].MediaType, "media type for entry %d in walk should be the same", i)
				assert.Equalf(t, path.Walk[i].Annotations, newPath.Walk[i].Annotations, "annotations for entry %d in walk should be the same", i)
				assert.NotEqualf(t, path.Walk[i].Digest, newPath.Walk[i].Digest, "digest for entry %d in walk should be different", i)
			}
		}

		// Emulate a reference resolution with walkDescriptorRoot.
		walkPath, err := walkDescriptorRoot(t.Context(), engineExt, newPath.Root())
		if assert.NoErrorf(t, err, "walk new path %d", idx) {
			assert.Equalf(t, newPath, walkPath, "walk of new path %d should give the same path", idx)
		}

		// Make sure the old path still exists (not necessary to be honest).
		oldWalkPath, err := walkDescriptorRoot(t.Context(), engineExt, path.Root())
		if assert.NoErrorf(t, err, "walk old path %d", idx) {
			assert.Equalf(t, path, oldWalkPath, "walk of old path %d should give the same path", idx)
		}
	}
}
