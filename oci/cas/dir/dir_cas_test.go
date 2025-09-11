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

package dir

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal/testhelpers"
	"github.com/opencontainers/umoci/oci/cas"
)

// NOTE: These tests aren't really testing OCI-style manifests. It's all just
//       example structures to make sure that the CAS acts properly.

func TestCreateLayout(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := Create(image)
	require.NoError(t, err)

	engine, err := Open(image)
	require.NoError(t, err)
	defer engine.Close() //nolint:errcheck

	// We should have an empty index and no blobs.
	index, err := engine.GetIndex(t.Context())
	require.NoError(t, err, "get index")
	assert.Empty(t, index.Manifests, "new image should have no manifests")

	blobs, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs")
	assert.Empty(t, blobs, "new image should have no blobs")

	// We should get an error if we try to create a new image atop an old one.
	err = Create(image)
	assert.Error(t, err, "trying to clobber existing image should fail") //nolint:testifylint // assert.*Error* makes more sense
}

func TestEngineBlob(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := Create(image)
	require.NoError(t, err)

	engine, err := Open(image)
	require.NoError(t, err)
	defer engine.Close() //nolint:errcheck

	for _, test := range []struct {
		bytes []byte
	}{
		{[]byte("")},
		{[]byte("some blob")},
		{[]byte("another blob")},
	} {
		digester := cas.BlobAlgorithm.Digester()
		expectedSize, err := io.Copy(digester.Hash(), bytes.NewReader(test.bytes))
		require.NoError(t, err)
		assert.EqualValues(t, len(test.bytes), expectedSize, "whole blob should be written to hasher") //nolint:testifylint // we are testing expectedSize
		expectedDigest := digester.Digest()

		digest, size, err := engine.PutBlob(t.Context(), bytes.NewReader(test.bytes))
		require.NoError(t, err, "put blob")
		assert.Equal(t, expectedDigest, digest, "put blob digest should match actual digest")
		assert.Equal(t, expectedSize, size, "put blob size should match actual size")

		blobReader, err := engine.GetBlob(t.Context(), digest)
		require.NoError(t, err, "get blob")
		defer blobReader.Close() //nolint:errcheck

		gotBytes, err := io.ReadAll(blobReader)
		require.NoError(t, err)
		assert.Equal(t, test.bytes, gotBytes, "get blob should give same contents")

		err = engine.DeleteBlob(t.Context(), digest)
		require.NoError(t, err, "delete blob")

		blobReader2, err := engine.GetBlob(t.Context(), digest)
		assert.ErrorIs(t, err, os.ErrNotExist, "get blob should fail for non-existent blob") //nolint:testifylint // assert.*Error* makes more sense
		assert.Nil(t, blobReader2, "get blob should return nil blob reader for non-existent blob")

		// DeleteBlob is idempotent. It shouldn't cause an error.
		err = engine.DeleteBlob(t.Context(), digest)
		require.NoError(t, err, "delete non-existent blob should still succeed")
	}

	// Should be no blobs left.
	blobs, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs")
	assert.Empty(t, blobs, "image should contain no blobs after all deletions")
}

func TestEngineValidate(t *testing.T) {
	// Empty directory.
	t.Run("EmptyDir", func(t *testing.T) {
		image := t.TempDir()

		engine, err := Open(image)
		require.Error(t, err, "empty directory is not a valid image")
		assert.Nil(t, engine)
	})

	// Invalid oci-layout.
	t.Run("InvalidLayoutJSON-NonJSON", func(t *testing.T) {
		image := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(image, layoutFile), []byte("invalid JSON"), 0o644))

		engine, err := Open(image)
		require.Error(t, err, "non-json oci-layout is not a valid image")
		assert.Nil(t, engine)
	})

	// Invalid oci-layout.
	t.Run("InvalidLayoutJSON-Empty", func(t *testing.T) {
		image := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(image, layoutFile), []byte("{}"), 0o644))

		engine, err := Open(image)
		require.Error(t, err, "empty oci-layout is not a valid image")
		assert.Nil(t, engine)
	})

	// Missing blobdir.
	t.Run("BlobDir-Missing", func(t *testing.T) {
		image := filepath.Join(t.TempDir(), "image")
		err := Create(image)
		require.NoError(t, err, "create image")
		require.NoError(t, os.RemoveAll(filepath.Join(image, blobDirectory)))

		engine, err := Open(image)
		require.Error(t, err, "missing blob directory is not a valid image")
		assert.Nil(t, engine)
	})

	// blobdir is not a directory.
	t.Run("BlobDir-File", func(t *testing.T) {
		image := filepath.Join(t.TempDir(), "image")
		err := Create(image)
		require.NoError(t, err, "create image")
		require.NoError(t, os.RemoveAll(filepath.Join(image, blobDirectory)))
		require.NoError(t, os.WriteFile(filepath.Join(image, blobDirectory), []byte(""), 0o755))

		engine, err := Open(image)
		require.Error(t, err, "blob directory as file is not a valid image")
		assert.Nil(t, engine)
	})

	// Missing index.json.
	t.Run("IndexJSON-Missing", func(t *testing.T) {
		image := filepath.Join(t.TempDir(), "image")
		err := Create(image)
		require.NoError(t, err, "create image")
		require.NoError(t, os.RemoveAll(filepath.Join(image, indexFile)))

		engine, err := Open(image)
		require.Error(t, err, "missing index.json is not a valid image")
		assert.Nil(t, engine)
	})

	// index is not a valid file.
	t.Run("IndexJSON-Dir", func(t *testing.T) {
		image := filepath.Join(t.TempDir(), "image")
		err := Create(image)
		require.NoError(t, err, "create image")
		require.NoError(t, os.RemoveAll(filepath.Join(image, indexFile)))
		require.NoError(t, os.Mkdir(filepath.Join(image, indexFile), 0o755))

		engine, err := Open(image)
		require.Error(t, err, "index.json as directory is not a valid image")
		assert.Nil(t, engine)
	})

	// No such directory.
	t.Run("IndexJSON-Dir", func(t *testing.T) {
		image := filepath.Join(t.TempDir(), "non-exist")

		engine, err := Open(image)
		require.Error(t, err, "non-existent path is not a valid image")
		assert.Nil(t, engine)
	})
}

// Make sure that opencontainers/umoci#63 doesn't have a regression. We
// shouldn't GC any blobs which are currently locked.
func TestEngineGCLocking(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := Create(image)
	require.NoError(t, err)

	engine, err := Open(image)
	require.NoError(t, err)
	defer engine.Close() //nolint:errcheck

	// Open a reference to the CAS, and make sure that it has a .temp set up.
	content := []byte("here's some sample content")

	digester := cas.BlobAlgorithm.Digester()
	expectedSize, err := io.Copy(digester.Hash(), bytes.NewReader(content))
	require.NoError(t, err)
	assert.EqualValues(t, len(content), expectedSize, "whole blob should be written to hasher") //nolint:testifylint // we are testing expectedSize
	expectedDigest := digester.Digest()

	digest, size, err := engine.PutBlob(t.Context(), bytes.NewReader(content))
	require.NoError(t, err, "put blob")
	assert.Equal(t, expectedDigest, digest, "put blob should return same digest as content")
	assert.Equal(t, expectedSize, size, "put blob should return same size as content")

	// We need a live tmpdir that has an advisory lock set.
	engineTempDir := engine.(*dirEngine).temp //nolint:forcetypeassert
	require.NotEmpty(t, engineTempDir, "engine should have a tmpdir after adding a blob")

	// Create subpaths to make sure our GC will only clean things that we can
	// be sure can be removed.
	umociTestDir, err := os.MkdirTemp(image, ".umoci-dead-") //nolint:usetesting // we are intentionally creating a tempdir inside an image dir
	require.NoError(t, err)
	otherTestDir, err := os.MkdirTemp(image, "other-") //nolint:usetesting // we are intentionally creating a tempdir inside an image dir
	require.NoError(t, err)

	// Open a new reference and GC it.
	gcEngine, err := Open(image)
	require.NoError(t, err)

	// TODO: This should be done with casext.GC...
	err = gcEngine.Clean(t.Context())
	require.NoError(t, err, "engine clean")

	for _, path := range []string{
		engineTempDir,
		otherTestDir,
	} {
		_, err := os.Lstat(path)
		require.NoErrorf(t, err, "image subpath %q should exist after GC", path)
	}

	for _, path := range []string{
		umociTestDir,
	} {
		_, err := os.Lstat(path)
		require.ErrorIsf(t, err, os.ErrNotExist, "image subpath %q should not exist after GC", path)
	}
}

func TestCreateLayoutReadonly(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := Create(image)
	require.NoError(t, err)

	// make it readonly
	testhelpers.MakeReadOnly(t, image)
	defer testhelpers.MakeReadWrite(t, image)

	engine, err := Open(image)
	require.NoError(t, err)
	defer engine.Close() //nolint:errcheck

	// We should have an empty index and no blobs.
	index, err := engine.GetIndex(t.Context())
	require.NoError(t, err, "get index")
	assert.Empty(t, index.Manifests, "new image should have no manifests")

	blobs, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs")
	assert.Empty(t, blobs, "new image should have no blobs")
}

func TestEngineBlobReadonly(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := Create(image)
	require.NoError(t, err)

	for _, test := range []struct {
		bytes []byte
	}{
		{[]byte("")},
		{[]byte("some blob")},
		{[]byte("another blob")},
	} {
		engine, err := Open(image)
		require.NoError(t, err, "open read-write image")

		digester := cas.BlobAlgorithm.Digester()
		expectedSize, err := io.Copy(digester.Hash(), bytes.NewReader(test.bytes))
		require.NoError(t, err)
		assert.EqualValues(t, len(test.bytes), expectedSize, "whole blob should be written to hasher") //nolint:testifylint // we are testing expectedSize
		expectedDigest := digester.Digest()

		digest, size, err := engine.PutBlob(t.Context(), bytes.NewReader(test.bytes))
		require.NoError(t, err, "put blob")
		assert.Equal(t, expectedDigest, digest, "put blob digest should match actual digest")
		assert.Equal(t, expectedSize, size, "put blob size should match actual size")

		require.NoError(t, engine.Close(), "close read-write engine")

		// make it readonly
		testhelpers.MakeReadOnly(t, image)

		newEngine, err := Open(image)
		require.NoError(t, err, "open read-only image")

		blobReader, err := engine.GetBlob(t.Context(), digest)
		require.NoError(t, err, "get blob")
		defer blobReader.Close() //nolint:errcheck

		gotBytes, err := io.ReadAll(blobReader)
		require.NoError(t, err)
		assert.Equal(t, test.bytes, gotBytes, "get blob should give same contents")

		// Make sure that writing again will FAIL.
		_, _, err = newEngine.PutBlob(t.Context(), bytes.NewReader(test.bytes))
		require.Error(t, err, "put blob on read-only image should fail")
		err = newEngine.DeleteBlob(t.Context(), digest)
		require.Error(t, err, "delete blob on read-only image should fail")

		require.NoError(t, newEngine.Close(), "close read-only engine")

		// make it readwrite again.
		testhelpers.MakeReadWrite(t, image)
	}
}
