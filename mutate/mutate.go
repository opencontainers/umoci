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

// Package mutate implements various functionality to allow for the
// modification of container images in a much higher-level fashion than
// available from github.com/opencontainers/umoci/oci/cas. In particular, this library
// should be viewed as a wrapper around github.com/opencontainers/umoci/oci/cas that
// provides many convenience functions.
package mutate

import (
	"io"
	"reflect"
	"runtime"
	"time"

	"github.com/apex/log"
	gzip "github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

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
	Created time.Time `json:"created,omitempty"`

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
		blob, err := m.engine.FromDescriptor(ctx, m.source.Descriptor())
		if err != nil {
			return errors.Wrap(err, "cache source manifest")
		}
		defer blob.Close()

		manifest, ok := blob.Data.(ispec.Manifest)
		if !ok {
			// Should _never_ be reached.
			return errors.Errorf("[internal error] unknown manifest blob type: %s", blob.Descriptor.MediaType)
		}

		// Make a copy of the manifest.
		m.manifest = manifestPtr(manifest)
	}

	if m.config == nil {
		blob, err := m.engine.FromDescriptor(ctx, m.manifest.Config)
		if err != nil {
			return errors.Wrap(err, "cache source config")
		}
		defer blob.Close()

		config, ok := blob.Data.(ispec.Image)
		if !ok {
			// Should _never_ be reached.
			return errors.Errorf("[internal error] unknown config blob type: %s", blob.Descriptor.MediaType)
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
		return nil, errors.Errorf("unsupported source type: %s", mt)
	}

	return &Mutator{
		engine: casext.NewEngine(engine),
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

// Manifest returns the current (cached) image manifest. This is what will be
// appended to when any additional Add() calls are made, and what will be
// Commit()ed if no further changes are made.
func (m *Mutator) Manifest(ctx context.Context) (ispec.Manifest, error) {
	if err := m.cache(ctx); err != nil {
		return ispec.Manifest{}, errors.Wrap(err, "getting cache failed")
	}

	return *m.manifest, nil
}

// Meta returns the current (cached) image metadata, which should be used as
// the source for any modifications of the configuration using Set.
func (m *Mutator) Meta(ctx context.Context) (Meta, error) {
	if err := m.cache(ctx); err != nil {
		return Meta{}, errors.Wrap(err, "getting cache failed")
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
func (m *Mutator) Set(ctx context.Context, config ispec.ImageConfig, meta Meta, annotations map[string]string, history *ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return errors.Wrap(err, "getting cache failed")
	}

	// Set annotations.
	m.manifest.Annotations = annotations

	// Set configuration.
	m.config.Config = config

	// Set metadata.
	m.config.Created = timePtr(meta.Created)
	m.config.Author = meta.Author
	m.config.Architecture = meta.Architecture
	m.config.OS = meta.OS

	// Append history.
	if history != nil {
		history.EmptyLayer = true
		m.config.History = append(m.config.History, *history)
	}
	return nil
}

// add adds the given layer to the CAS, and mutates the configuration to
// include the diffID. The returned string is the digest of the *compressed*
// layer (which is compressed by us).
func (m *Mutator) add(ctx context.Context, reader io.Reader, history *ispec.History) (digest.Digest, int64, error) {
	if err := m.cache(ctx); err != nil {
		return "", -1, errors.Wrap(err, "getting cache failed")
	}

	diffidDigester := cas.BlobAlgorithm.Digester()
	hashReader := io.TeeReader(reader, diffidDigester.Hash())

	pipeReader, pipeWriter := io.Pipe()
	defer pipeReader.Close()

	gzw := gzip.NewWriter(pipeWriter)
	defer gzw.Close()
	if err := gzw.SetConcurrency(256<<10, 2*runtime.NumCPU()); err != nil {
		return "", -1, errors.Wrapf(err, "set concurrency level to %v blocks", 2*runtime.NumCPU())
	}
	go func() {
		if _, err := io.Copy(gzw, hashReader); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "compressing layer"))
		}
		if err := gzw.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close gzip writer"))
		}
		if err := pipeWriter.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close pipe writer"))
		}
	}()

	layerDigest, layerSize, err := m.engine.PutBlob(ctx, pipeReader)
	if err != nil {
		return "", -1, errors.Wrap(err, "put layer blob")
	}

	// Add DiffID to configuration.
	layerDiffID := diffidDigester.Digest()
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
	return layerDigest, layerSize, nil
}

// Add adds a layer to the image, by reading the layer changeset blob from the
// provided reader. The stream must not be compressed, as it is used to
// generate the DiffIDs for the image metatadata. The provided history entry is
// appended to the image's history and should correspond to what operations
// were made to the configuration.
func (m *Mutator) Add(ctx context.Context, r io.Reader, history *ispec.History) (ispec.Descriptor, error) {
	desc := ispec.Descriptor{}
	if err := m.cache(ctx); err != nil {
		return desc, errors.Wrap(err, "getting cache failed")
	}

	digest, size, err := m.add(ctx, r, history)
	if err != nil {
		return desc, errors.Wrap(err, "add layer")
	}

	// Append to layers.
	desc = ispec.Descriptor{
		// TODO: Detect whether the layer is gzip'd or not...
		MediaType: ispec.MediaTypeImageLayerGzip,
		Digest:    digest,
		Size:      size,
	}
	m.manifest.Layers = append(m.manifest.Layers, desc)
	return desc, nil
}

// AddNonDistributable is the same as Add, except it adds a non-distributable
// layer to the image.
func (m *Mutator) AddNonDistributable(ctx context.Context, r io.Reader, history *ispec.History) error {
	if err := m.cache(ctx); err != nil {
		return errors.Wrap(err, "getting cache failed")
	}

	digest, size, err := m.add(ctx, r, history)
	if err != nil {
		return errors.Wrap(err, "add non-distributable layer")
	}

	// Append to layers.
	m.manifest.Layers = append(m.manifest.Layers, ispec.Descriptor{
		// TODO: Detect whether the layer is gzip'd or not...
		MediaType: ispec.MediaTypeImageLayerNonDistributableGzip,
		Digest:    digest,
		Size:      size,
	})
	return nil
}

// Commit writes all of the temporary changes made to the configuration,
// metadata and manifest to the engine. It then returns a new manifest
// descriptor (which can be used in place of the source descriptor provided to
// New).
func (m *Mutator) Commit(ctx context.Context) (casext.DescriptorPath, error) {
	if err := m.cache(ctx); err != nil {
		return casext.DescriptorPath{}, errors.Wrap(err, "getting cache failed")
	}

	// We first have to commit the configuration blob.
	configDigest, configSize, err := m.engine.PutBlobJSON(ctx, m.config)
	if err != nil {
		return casext.DescriptorPath{}, errors.Wrap(err, "commit mutated config blob")
	}

	m.manifest.Config = ispec.Descriptor{
		MediaType: m.manifest.Config.MediaType,
		Digest:    configDigest,
		Size:      configSize,
	}

	// Now commit the manifest.
	manifestDigest, manifestSize, err := m.engine.PutBlobJSON(ctx, m.manifest)
	if err != nil {
		return casext.DescriptorPath{}, errors.Wrap(err, "commit mutated manifest blob")
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
			return casext.DescriptorPath{}, errors.Wrapf(err, "get parent-%d blob", idx)
		}
		defer parentBlob.Close()

		// Replace all references to the child blob with the new one.
		old := m.source.Walk[idx]
		new := newPath.Walk[idx]
		if err := casext.MapDescriptors(parentBlob.Data, func(d ispec.Descriptor) ispec.Descriptor {
			// XXX: Maybe we should just be comparing the Digest?
			if reflect.DeepEqual(d, old) {
				d = new
			}
			return d
		}); err != nil {
			return casext.DescriptorPath{}, errors.Wrapf(err, "rewrite parent-%d blob", idx)
		}

		// Re-commit the blob.
		// TODO: This won't handle foreign blobs correctly, we need to make it
		//       possible to write a modified blob through the blob API.
		blobDigest, blobSize, err := m.engine.PutBlobJSON(ctx, parentBlob.Data)
		if err != nil {
			return casext.DescriptorPath{}, errors.Wrapf(err, "put json parent-%d blob", idx)
		}

		// Update the key parts of the descriptor.
		newPath.Walk[idx-1].Digest = blobDigest
		newPath.Walk[idx-1].Size = blobSize
	}

	return newPath, nil
}
