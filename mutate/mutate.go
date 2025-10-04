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

// Package mutate implements various functionality to allow for the
// modification of container images in a much higher-level fashion than
// available from github.com/opencontainers/umoci/oci/cas. In particular, this library
// should be viewed as a wrapper around github.com/opencontainers/umoci/oci/cas that
// provides many convenience functions.
package mutate

import (
	"context"
	"fmt"
	"io"
	"maps"
	"time"

	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/iohelpers"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
	"github.com/opencontainers/umoci/oci/casext/mediatype"
)

// UmociUncompressedBlobSizeAnnotation is an umoci-specific annotation to
// provide information in descriptors to compressed blobs about the size of the
// underlying uncompressed blob for users that need that information. Note that
// this annotation value should be treated as a hint -- an attacker could
// create an image that has a dummy UmociUncompressedBlobSizeAnnotation value
// for a zip-bomb blob.
const UmociUncompressedBlobSizeAnnotation = "ci.umo.uncompressed_blob_size"

func configPtr(c ispec.Image) *ispec.Image         { return &c }
func manifestPtr(m ispec.Manifest) *ispec.Manifest { return &m }
func timePtr(t time.Time) *time.Time               { return &t }

// XXX: Currently this package is very entangled in modifying of a given
//      Manifest and their associated Config + Layers. While this works fine,
//      really mutate/ should be a far more generic library that allows you to
//      apply a delta for a particular OCI structure and then regenerate the
//      necessary blobs. Something like changing annotations for intermediate
//      manifests is not really possible at the moment, and it's not clear how
//      to make it possible without forcing users to interactively make all the
//      necessary changes.

// Mutator is a wrapper around a cas.Engine instance, and is used to mutate a
// given image (described by a manifest) in a high-level fashion. It handles
// creating all necessary blobs and modfying other blobs. In order for changes
// to be committed you must call .Commit().
//
// TODO: Implement manifest list support.
type Mutator struct {
	// These are the arguments we got in New().
	engine casext.Engine
	source casext.DescriptorPath

	// Cached values of the configuration and manifest.
	manifest *ispec.Manifest
	config   *ispec.Image
}

// Meta is a wrapper around the "safe" fields in ispec.Image, which can be
// modified by users and have no effect on a Mutator or the validity of an
// image.
type Meta struct {
	// Created defines an ISO-8601 formatted combined date and time at which
	// the image was created.
	Created time.Time `json:"created,omitzero"`

	// Author defines the name and/or email address of the person or entity
	// which created and is responsible for maintaining the image.
	Author string `json:"author,omitzero"`

	// Architecture is the CPU architecture which the binaries in this image
	// are built to run on.
	Architecture string `json:"architecture"`

	// Variant is the variant of the CPU architecture which the binaries in
	// this image are built to run on.
	Variant string `json:"variant"`

	// OS is the name of the operating system which the image is built to run
	// on.
	OS string `json:"os"`

	// TODO: Should we embed ispec.Platform?
}

// cache ensures that the cached versions of the related configurations have
// been loaded. Calling this function more than once will do nothing, unless
// you've explicitly cleared the cache.
func (m *Mutator) cache(ctx context.Context) (Err error) {
	// We need the manifest
	if m.manifest == nil {
		blob, err := m.engine.FromDescriptor(ctx, m.source.Descriptor())
		if err != nil {
			return fmt.Errorf("cache source manifest: %w", err)
		}
		defer funchelpers.VerifyClose(&Err, blob)

		manifest, ok := blob.Data.(ispec.Manifest)
		if !ok {
			// Should _never_ be reached.
			return fmt.Errorf("[internal error] unknown manifest blob type: %s", blob.Descriptor.MediaType)
		}

		// Make a copy of the manifest.
		m.manifest = manifestPtr(manifest)
	}

	if m.config == nil {
		blob, err := m.engine.FromDescriptor(ctx, m.manifest.Config)
		if err != nil {
			return fmt.Errorf("cache source config: %w", err)
		}
		defer funchelpers.VerifyClose(&Err, blob)

		config, ok := blob.Data.(ispec.Image)
		if !ok {
			// Should _never_ be reached.
			return fmt.Errorf("[internal error] unknown config blob type: %s", blob.Descriptor.MediaType)
		}

		// Make a copy of the config and configDescriptor.
		m.config = configPtr(config)
	}

	return nil
}

// New creates a new Mutator for the given descriptor (which _must_ have a
// MediaType of ispec.MediaTypeImageManifest.
func New(engine cas.Engine, src casext.DescriptorPath) (*Mutator, error) {
	// We currently only support changing a given manifest through a walk.
	if mt := src.Descriptor().MediaType; mt != ispec.MediaTypeImageManifest {
		return nil, fmt.Errorf("unsupported source type: %s", mt)
	}

	return &Mutator{
		engine: casext.NewEngine(engine),
		source: src,
	}, nil
}

// Config returns the current (cached) image configuration, which should be
// used as the source for any modifications of the configuration using
// Set.
func (m *Mutator) Config(ctx context.Context) (ispec.Image, error) {
	if err := m.cache(ctx); err != nil {
		return ispec.Image{}, fmt.Errorf("getting cache failed: %w", err)
	}

	return *m.config, nil
}

// Manifest returns the current (cached) image manifest. This is what will be
// appended to when any additional Add() calls are made, and what will be
// Commit()ed if no further changes are made.
func (m *Mutator) Manifest(ctx context.Context) (ispec.Manifest, error) {
	if err := m.cache(ctx); err != nil {
		return ispec.Manifest{}, fmt.Errorf("getting cache failed: %w", err)
	}

	return *m.manifest, nil
}

// Meta returns the current (cached) image metadata, which should be used as
// the source for any modifications of the configuration using Set.
func (m *Mutator) Meta(ctx context.Context) (Meta, error) {
	if err := m.cache(ctx); err != nil {
		return Meta{}, fmt.Errorf("getting cache failed: %w", err)
	}

	var created time.Time
	if m.config.Created != nil {
		created = *m.config.Created
	}
	return Meta{
		Created:      created,
		Author:       m.config.Author,
		Architecture: m.config.Architecture,
		OS:           m.config.OS,
	}, nil
}

// Annotations returns the set of annotations in the current manifest. This
// does not include the annotations set in ispec.ImageConfig.Labels. This
// should be used as the source for any modifications of the annotations using
// Set.
func (m *Mutator) Annotations(ctx context.Context) (map[string]string, error) {
	if err := m.cache(ctx); err != nil {
		return nil, fmt.Errorf("getting cache failed: %w", err)
	}

	annotations := map[string]string{}
	maps.Copy(annotations, m.manifest.Annotations)
	return annotations, nil
}

// Set sets the image configuration and metadata to the given values. The
// provided ispec.History entry is appended to the image's history and should
// correspond to what operations were made to the configuration.
func (m *Mutator) Set(ctx context.Context, config ispec.ImageConfig, meta Meta, annotations map[string]string, history *ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return fmt.Errorf("getting cache failed: %w", err)
	}

	// Ensure the mediatype is correct.
	m.manifest.MediaType = ispec.MediaTypeImageManifest

	// Set annotations.
	m.manifest.Annotations = annotations

	// Set configuration.
	m.config.Config = config

	// Set metadata.
	m.config.Created = timePtr(meta.Created)
	m.config.Author = meta.Author
	m.config.Architecture = meta.Architecture
	m.config.Variant = meta.Variant
	m.config.OS = meta.OS

	// Append history.
	if history != nil {
		history.EmptyLayer = true
		m.config.History = append(m.config.History, *history)
	}
	return nil
}

func (m *Mutator) appendToConfig(history *ispec.History, layerDiffID digest.Digest) {
	m.config.RootFS.DiffIDs = append(m.config.RootFS.DiffIDs, layerDiffID)

	// Append history.
	if history != nil {
		history.EmptyLayer = false
		m.config.History = append(m.config.History, *history)
	} else {
		// Some tools get confused if there are layers with no history entry.
		// Especially if you have later layers have history entries (which will
		// result in the history entries not matching up and everyone getting
		// quite confused).
		log.Warnf("new layer has no history entry -- this will confuse many tools!")
	}
}

// PickDefaultCompressAlgorithm returns the best option for the compression
// algorithm for new layers. The main preference is to use re-use whatever the
// most recent layer's compression algorithm is (for those we support). As a
// final fallback, we use blobcompress.Default.
func (m *Mutator) PickDefaultCompressAlgorithm(ctx context.Context) (Compressor, error) {
	if err := m.cache(ctx); err != nil {
		return nil, fmt.Errorf("getting cache failed: %w", err)
	}
	layers := m.manifest.Layers
	for i := len(layers) - 1; i >= 0; i-- {
		_, compressType := mediatype.SplitMediaTypeSuffix(layers[i].MediaType)
		// Don't generate an uncompressed layer even if the previous one is --
		// there is no reason to automatically generate uncompressed blobs.
		if compressType != "" {
			candidate := blobcompress.GetAlgorithm(compressType)
			if candidate != nil {
				return candidate, nil
			}
		}
	}
	// No supported, non-plain algorithm found. Just use the default.
	return blobcompress.Default, nil
}

// Add adds a layer to the image, by reading the layer changeset blob from the
// provided reader. The stream must not be compressed, as it is used to
// generate the DiffIDs for the image metatadata. The provided history entry is
// appended to the image's history and should correspond to what operations
// were made to the configuration.
func (m *Mutator) Add(ctx context.Context, mediaType string, r io.Reader, history *ispec.History, compressor Compressor, annotations map[string]string) (_ ispec.Descriptor, Err error) {
	var desc ispec.Descriptor
	if err := m.cache(ctx); err != nil {
		return desc, fmt.Errorf("getting cache failed: %w", err)
	}

	countReader := iohelpers.CountReader(r)

	diffidDigester := cas.BlobAlgorithm.Digester()
	hashReader := io.TeeReader(countReader, diffidDigester.Hash())

	if compressor == nil {
		bestCompressor, err := m.PickDefaultCompressAlgorithm(ctx)
		if err != nil {
			return desc, fmt.Errorf("find best default layer compression algorithm: %w", err)
		}
		compressor = bestCompressor
	}

	compressed, err := compressor.Compress(hashReader)
	if err != nil {
		return desc, fmt.Errorf("couldn't create compression for blob: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, compressed)

	layerDigest, layerSize, err := m.engine.PutBlob(ctx, compressed)
	if err != nil {
		return desc, fmt.Errorf("put layer blob: %w", err)
	}

	// Add DiffID to configuration.
	m.appendToConfig(history, diffidDigester.Digest())

	// Build the descriptor.
	if suffix := compressor.MediaTypeSuffix(); suffix != "" {
		mediaType += "+" + suffix
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}
	if plainSize := countReader.BytesRead(); plainSize != layerSize {
		annotations[UmociUncompressedBlobSizeAnnotation] = fmt.Sprintf("%d", plainSize)
	}

	// Append to layers.
	desc = ispec.Descriptor{
		MediaType:   mediaType,
		Digest:      layerDigest,
		Size:        layerSize,
		Annotations: annotations,
	}
	m.manifest.Layers = append(m.manifest.Layers, desc)
	return desc, nil
}

// AddExisting adds a blob that already exists to the layer, using the user
// specified DiffID. It currently checks that the layer exists, but does not
// validate the DiffID.
func (m *Mutator) AddExisting(ctx context.Context, desc ispec.Descriptor, history *ispec.History, diffID digest.Digest) error {
	if err := m.cache(ctx); err != nil {
		return fmt.Errorf("getting cache failed: %w", err)
	}

	m.appendToConfig(history, diffID)
	m.manifest.Layers = append(m.manifest.Layers, desc)
	return nil
}

// Commit writes all of the temporary changes made to the configuration,
// metadata and manifest to the engine. It then returns a new manifest
// descriptor (which can be used in place of the source descriptor provided to
// New).
func (m *Mutator) Commit(ctx context.Context) (_ casext.DescriptorPath, Err error) {
	if err := m.cache(ctx); err != nil {
		return casext.DescriptorPath{}, fmt.Errorf("getting cache failed: %w", err)
	}

	// We first have to commit the configuration blob.
	configDigest, configSize, err := m.engine.PutBlobJSON(ctx, m.config)
	if err != nil {
		return casext.DescriptorPath{}, fmt.Errorf("commit mutated config blob: %w", err)
	}

	m.manifest.Config = ispec.Descriptor{
		MediaType: m.manifest.Config.MediaType,
		Digest:    configDigest,
		Size:      configSize,
	}

	// Now commit the manifest.
	manifestDigest, manifestSize, err := m.engine.PutBlobJSON(ctx, m.manifest)
	if err != nil {
		return casext.DescriptorPath{}, fmt.Errorf("commit mutated manifest blob: %w", err)
	}

	// We now have to create a new DescriptorPath that replaces the one we were
	// given. Note that we have to walk *up* the path rather than down it
	// because we have to replace each blob in order to replace its references.
	pathLength := len(m.source.Walk)
	newPath := casext.DescriptorPath{
		Walk: make([]ispec.Descriptor, pathLength),
	}
	copy(newPath.Walk, m.source.Walk)

	// Replace the end of the path.
	end := &newPath.Walk[pathLength-1]
	end.Digest = manifestDigest
	end.Size = manifestSize

	// Walk up the path, mutating the parent reference of each descriptor.
	for idx := pathLength - 1; idx >= 1; idx-- {
		// Get the blob of the parent.
		parentBlob, err := m.engine.FromDescriptor(ctx, newPath.Walk[idx-1])
		if err != nil {
			return casext.DescriptorPath{}, fmt.Errorf("get parent-%d blob: %w", idx, err)
		}
		defer funchelpers.VerifyClose(&Err, parentBlob)

		// Replace all references to the child blob with the new one.
		oldDesc := m.source.Walk[idx]
		newDesc := newPath.Walk[idx]
		if err := casext.MapDescriptors(parentBlob.Data, func(d ispec.Descriptor) ispec.Descriptor {
			if d.Digest == oldDesc.Digest {
				// In principle you should never be in a situation where two
				// descriptors reference the same data with different
				// media-types. This lead to CVE-2021-41190.
				if d.MediaType != oldDesc.MediaType {
					log.Warnf("mutate: found inconsistent media-type usage during DescriptorPath rewriting (found descriptor for blob %s with both %s and %s media-types)",
						oldDesc.Digest, oldDesc.MediaType, d.MediaType)
				}
				// Replace the digest+size with the new blob.
				d.Digest = newDesc.Digest
				d.Size = newDesc.Size
				// Copy the embedded data for the new descriptor (if any).
				d.Data = newDesc.Data
				// Do not touch any other bits in case the same blob is being
				// referenced with different annotations or platform
				// configurations.
			}
			return d
		}); err != nil {
			return casext.DescriptorPath{}, fmt.Errorf("rewrite parent-%d blob: %w", idx, err)
		}

		// Re-commit the blob.
		// TODO: This won't handle foreign blobs correctly, we need to make it
		//       possible to write a modified blob through the blob API.
		blobDigest, blobSize, err := m.engine.PutBlobJSON(ctx, parentBlob.Data)
		if err != nil {
			return casext.DescriptorPath{}, fmt.Errorf("put json parent-%d blob: %w", idx, err)
		}

		// Update the key parts of the descriptor.
		newPath.Walk[idx-1].Digest = blobDigest
		newPath.Walk[idx-1].Size = blobSize
		// Clear the embedded data (if present).
		// TODO: Auto-embed data if it is reasonably small.
		newPath.Walk[idx-1].Data = nil
	}

	return newPath, nil
}
