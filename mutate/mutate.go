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

package mutate

import (
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"

	"github.com/openSUSE/umoci/oci/cas"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func configPtr(c ispec.Image) *ispec.Image         { return &c }
func manifestPtr(m ispec.Manifest) *ispec.Manifest { return &m }

// Mutator is a wrapper around a cas.Engine instance, and is used to mutate a
// given image (described by a manifest) in a high-level fashion. It handles
// creating all necessary blobs and modfying other blobs. In order for changes
// to be comitted you must call .Commit().
//
// TODO: Implement manifest list support.
type Mutator struct {
	// These are the arguments we got in New().
	engine cas.Engine
	source ispec.Descriptor

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
	Created string `json:"created,omitempty"`

	// Author defines the name and/or email address of the person or entity
	// which created and is responsible for maintaining the image.
	Author string `json:"author,omitempty"`

	// Architecture is the CPU architecture which the binaries in this image
	// are built to run on.
	Architecture string `json:"architecture"`

	// OS is the name of the operating system which the image is built to run
	// on.
	OS string `json:"os"`
}

// cache ensures that the cached versions of the related configurations have
// been loaded. Calling this function more than once will do nothing, unless
// you've explicitly cleared the cache.
func (m *Mutator) cache(ctx context.Context) error {
	// We need the manifest
	if m.manifest == nil {
		blob, err := cas.FromDescriptor(ctx, m.engine, &m.source)
		if err != nil {
			return errors.Wrap(err, "cache source manifest")
		}
		defer blob.Close()

		manifest, ok := blob.Data.(*ispec.Manifest)
		if !ok {
			// Should never be reached.
			return errors.Errorf("unknown manifest blob type: %s", blob.MediaType)
		}

		// Make a copy of the manifest.
		m.manifest = manifestPtr(*manifest)
	}

	if m.config == nil {
		blob, err := cas.FromDescriptor(ctx, m.engine, &m.manifest.Config)
		if err != nil {
			return errors.Wrap(err, "cache source config")
		}
		defer blob.Close()

		config, ok := blob.Data.(*ispec.Image)
		if !ok {
			// Should never be reached.
			return errors.Errorf("unknown config blob type: %s", blob.MediaType)
		}

		// Make a copy of the config and configDescriptor.
		m.config = configPtr(*config)
	}

	return nil
}

// New creates a new Mutator for the given descriptor (which _must_ have a
// MediaType of ispec.MediaTypeImageManifest.
func New(engine cas.Engine, src ispec.Descriptor) (*Mutator, error) {
	// TODO: Implement manifest list support.
	if src.MediaType != ispec.MediaTypeImageManifest {
		return nil, errors.Errorf("unsupported source type: %s", src.MediaType)
	}

	return &Mutator{
		engine: engine,
		source: src,
	}, nil
}

// Config returns the current (cached) image configuration, which should be
// used as the source for any modifications of the configuration using
// Set.
func (m *Mutator) Config(ctx context.Context) (ispec.ImageConfig, error) {
	if err := m.cache(ctx); err != nil {
		return ispec.ImageConfig{}, errors.Wrap(err, "getting cache failed")
	}

	return m.config.Config, nil
}

// Meta returns the current (cached) image metadata, which should be used as
// the source for any modifications of the configuration using Set.
func (m *Mutator) Meta(ctx context.Context) (Meta, error) {
	if err := m.cache(ctx); err != nil {
		return Meta{}, errors.Wrap(err, "getting cache failed")
	}

	return Meta{
		Created:      m.config.Created,
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
		return nil, errors.Wrap(err, "getting cache failed")
	}

	annotations := map[string]string{}
	for k, v := range m.manifest.Annotations {
		annotations[k] = v
	}
	return annotations, nil
}

// Set sets the image configuration and metadata to the given values. The
// provided ispec.History entry is appended to the image's history and should
// correspond to what operations were made to the configuration.
func (m *Mutator) Set(ctx context.Context, config ispec.ImageConfig, meta Meta, annotations map[string]string, history ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return errors.Wrap(err, "getting cache failed")
	}

	// Set annotations.
	m.manifest.Annotations = annotations

	// Set configuration.
	m.config.Config = config

	// Set metadata.
	m.config.Created = meta.Created
	m.config.Author = meta.Author
	m.config.Architecture = meta.Architecture
	m.config.OS = meta.OS

	// Append history.
	history.EmptyLayer = true
	m.config.History = append(m.config.History, history)

	return nil
}

//

// add adds the given layer to the CAS, and mutates the configuration to
// include the diffID. The returned string is the digest of the *compressed*
// layer (which is compressed by us).
func (m *Mutator) add(ctx context.Context, reader io.Reader) (string, int64, error) {
	if err := m.cache(ctx); err != nil {
		return "", -1, errors.Wrap(err, "getting cache failed")
	}

	// XXX: We should not have to do this check here.
	if cas.BlobAlgorithm != "sha256" {
		return "", -1, errors.Errorf("unknown blob algorithm: %s", cas.BlobAlgorithm)
	}
	diffIDHash := sha256.New()
	hashReader := io.TeeReader(reader, diffIDHash)

	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	gzw := gzip.NewWriter(pipeWriter)
	defer gzw.Close()
	go func() {
		_, err := io.Copy(gzw, hashReader)
		if err != nil {
			pipeWriter.CloseWithError(errors.Wrap(err, "compressing layer"))
			return
		}
		gzw.Close()
		pipeWriter.Close()
	}()

	layerDigest, layerSize, err := m.engine.PutBlob(ctx, pipeReader)
	if err != nil {
		return "", -1, errors.Wrap(err, "put layer blob")
	}

	// Add DiffID to configuration.
	layerDiffID := fmt.Sprintf("%s:%x", cas.BlobAlgorithm, diffIDHash.Sum(nil))
	m.config.RootFS.DiffIDs = append(m.config.RootFS.DiffIDs, layerDiffID)

	return layerDigest, layerSize, nil
}

// Add adds a layer to the image, by reading the layer changeset blob from the
// provided reader. The stream must not be compressed, as it is used to
// generate the DiffIDs for the image metatadata. The provided history entry is
// appended to the image's history and should correspond to what operations
// were made to the configuration.
func (m *Mutator) Add(ctx context.Context, r io.Reader, history ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return errors.Wrap(err, "getting cache failed")
	}

	digest, size, err := m.add(ctx, r)
	if err != nil {
		return errors.Wrap(err, "add layer")
	}

	// Append to layers.
	m.manifest.Layers = append(m.manifest.Layers, ispec.Descriptor{
		MediaType: ispec.MediaTypeImageLayer,
		Digest:    digest,
		Size:      size,
	})

	// Append history.
	history.EmptyLayer = false
	m.config.History = append(m.config.History, history)
	return nil
}

// AddNonDistributable is the same as Add, except it adds a non-distributable
// layer to the image.
func (m *Mutator) AddNonDistributable(ctx context.Context, r io.Reader, history ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return errors.Wrap(err, "getting cache failed")
	}

	digest, size, err := m.add(ctx, r)
	if err != nil {
		return errors.Wrap(err, "add non-distributable layer")
	}

	// Append to layers.
	m.manifest.Layers = append(m.manifest.Layers, ispec.Descriptor{
		MediaType: ispec.MediaTypeImageLayerNonDistributable,
		Digest:    digest,
		Size:      size,
	})

	// Append history.
	history.EmptyLayer = false
	m.config.History = append(m.config.History, history)
	return nil
}

// Commit writes all of the temporary changes made to the configuration,
// metadata and manifest to the engine. It then returns a new manifest
// descriptor (which can be used in place of the source descriptor provided to
// New).
func (m *Mutator) Commit(ctx context.Context) (ispec.Descriptor, error) {
	if err := m.cache(ctx); err != nil {
		return ispec.Descriptor{}, errors.Wrap(err, "getting cache failed")
	}

	// We first have to commit the configuration blob.
	configDigest, configSize, err := m.engine.PutBlobJSON(ctx, m.config)
	if err != nil {
		return ispec.Descriptor{}, errors.Wrap(err, "commit mutated config blob")
	}

	m.manifest.Config = ispec.Descriptor{
		MediaType: m.manifest.Config.MediaType,
		Digest:    configDigest,
		Size:      configSize,
	}

	// Now commit the manifest.
	manifestDigest, manifestSize, err := m.engine.PutBlobJSON(ctx, m.manifest)
	if err != nil {
		return ispec.Descriptor{}, errors.Wrap(err, "commit mutated manifest blob")
	}

	// Generate a new descriptor.
	return ispec.Descriptor{
		MediaType: m.source.MediaType,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}, nil
}
