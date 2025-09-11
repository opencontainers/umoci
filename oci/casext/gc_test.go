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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/oci/cas/dir"
)

func TestGCWithEmptyIndex(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	// creates an empty index.json and several orphan blobs which should be pruned
	descMap := fakeSetupEngine(t, engineExt)
	require.NotEmpty(t, descMap, "fakeSetupEngine descriptor map")

	b, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs before gc")
	assert.NotEmpty(t, b, "fakeSetupEngine'd image should contain blobs")

	err = engineExt.GC(t.Context())
	require.NoError(t, err, "gc")

	b, err = engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs after gc")
	assert.Empty(t, b, "empty image should contain no blobs")
}

func TestGCWithNonEmptyIndex(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	// creates an empty index.json and several orphan blobs which should be pruned
	descMap := fakeSetupEngine(t, engineExt)
	require.NotEmpty(t, descMap, "fakeSetupEngine descriptor map")

	b, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs")
	assert.NotEmpty(t, b, "fakeSetupEngine'd image should contain blobs")
	initalBlobNumber := len(b)

	// build a blob, manifest, index that will survive GC
	content := "this is a test blob"
	br := strings.NewReader(content)
	digest, size, err := engine.PutBlob(t.Context(), br)
	require.NoError(t, err, "put blob")
	assert.EqualValues(t, len(content), size, "put blob should write entire blob")

	m := ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageManifest,
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageLayer,
			Digest:    digest,
			Size:      size,
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageLayer,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	data, err := json.Marshal(&m)
	require.NoError(t, err, "marshal manifest json")
	mr := bytes.NewReader(data)
	digest, size, err = engine.PutBlob(t.Context(), mr)
	require.NoError(t, err, "put marshal json blob")
	assert.EqualValues(t, len(data), size, "put blob should write entire blob")

	idx := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageIndex,
		Manifests: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageManifest,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	err = engine.PutIndex(t.Context(), idx)
	require.NoError(t, err, "put index")

	b, err = engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs before gc")
	assert.Len(t, b, initalBlobNumber+2, "before gc we should have two more blobs")

	err = engineExt.GC(t.Context())
	require.NoError(t, err, "gc")

	b, err = engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs after gc")
	assert.Len(t, b, 2, "after gc only two pinned blobs should remain")
}

func gcOkFunc(t *testing.T, expectedDigest digest.Digest) GCPolicy {
	return func(ctx context.Context, digest digest.Digest) (bool, error) { //nolint:revive // unused-parameter doesn't make sense for this test
		assert.Equal(t, expectedDigest, digest, "unexpected digest with gc callback")
		return true, nil
	}
}

func gcSkipFunc(t *testing.T, expectedDigest digest.Digest) GCPolicy {
	return func(ctx context.Context, digest digest.Digest) (bool, error) { //nolint:revive // unused-parameter doesn't make sense for this test
		assert.Equal(t, expectedDigest, digest, "unexpected digest with gc callback")
		return false, nil
	}
}

func errFunc(ctx context.Context, digest digest.Digest) (bool, error) { //nolint:revive // unused-parameter doesn't make sense for this test
	return false, errors.New("err policy")
}

func TestGCWithPolicy(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	// build a orphan blob that should be GC'ed
	content := "this is a orphan blob"
	br := strings.NewReader(content)
	odigest, size, err := engine.PutBlob(t.Context(), br)
	require.NoError(t, err, "put blob")
	assert.EqualValues(t, len(content), size, "put blob should write entire blob")

	// build a blob, manifest, index that will survive GC
	content = "this is a test blob"
	br = strings.NewReader(content)
	digest, size, err := engine.PutBlob(t.Context(), br)
	require.NoError(t, err, "put blob")
	assert.EqualValues(t, len(content), size, "put blob should write entire blob")

	digest, size, err = engineExt.PutBlobJSON(t.Context(),
		ispec.Manifest{
			Versioned: imeta.Versioned{
				SchemaVersion: 2,
			},
			MediaType: ispec.MediaTypeImageManifest,
			Config: ispec.Descriptor{
				MediaType: ispec.MediaTypeImageLayer,
				Digest:    digest,
				Size:      size,
			},
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayer,
					Digest:    digest,
					Size:      size,
				},
			},
		})
	require.NoError(t, err, "put manifest json blob")

	idx := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageIndex,
		Manifests: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageManifest,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	err = engine.PutIndex(t.Context(), idx)
	require.NoError(t, err, "put index")

	err = engineExt.GC(t.Context(), errFunc)
	require.Error(t, err, "gc with err policy should fail")

	err = engineExt.GC(t.Context(), gcSkipFunc(t, odigest))
	require.NoError(t, err, "gc with skip policy")

	b, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs after gc with skip policy")
	assert.Len(t, b, 3, "after gc with skip policy all blobs should remain")

	err = engineExt.GC(t.Context(), gcOkFunc(t, odigest))
	require.NoError(t, err, "gc with ok policy")

	b, err = engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs after gc with skip policy")
	assert.Len(t, b, 2, "after gc with ok policy only pinned blobs should remain")
}
