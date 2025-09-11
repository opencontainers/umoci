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
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal/testhelpers"
	"github.com/opencontainers/umoci/oci/cas/dir"
)

func TestEngineBlobJSON(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	engine, err := dir.Open(image)
	require.NoError(t, err)
	engineExt := NewEngine(engine)
	defer engine.Close() //nolint:errcheck

	type object struct {
		A string `json:"A"`
		B int64  `json:"B,omitempty"`
	}

	for _, test := range []struct {
		object object
	}{
		{object{}},
		{object{"a value", 100}},
		{object{"another value", 200}},
	} {
		digest, _, err := engineExt.PutBlobJSON(t.Context(), test.object)
		require.NoError(t, err, "put blob json")

		blobReader, err := engine.GetBlob(t.Context(), digest)
		require.NoError(t, err, "get blob")
		defer blobReader.Close() //nolint:errcheck

		gotBytes, err := io.ReadAll(blobReader)
		require.NoError(t, err, "read entire blob")

		var gotObject object
		err = json.Unmarshal(gotBytes, &gotObject)
		require.NoError(t, err, "unmarshal blob")
		assert.Equal(t, test.object, gotObject, "parsed json blob should match original data")

		err = engine.DeleteBlob(t.Context(), digest)
		require.NoError(t, err, "delete blob")

		br, err := engine.GetBlob(t.Context(), digest)
		assert.ErrorIs(t, err, os.ErrNotExist, "get blob after deleting should fail") //nolint:testifylint // assert.*Error* makes more sense
		assert.Nil(t, br, "get blob after deleting should fail")

		// DeleteBlob is idempotent. It shouldn't cause an error.
		err = engine.DeleteBlob(t.Context(), digest)
		require.NoError(t, err, "delete non-existent blob")
	}

	// Should be no blobs left.
	blobs, err := engine.ListBlobs(t.Context())
	require.NoError(t, err, "list blobs at end of test")
	assert.Empty(t, blobs, "no blobs should remain at end of test")
}

func TestEngineBlobJSONReadonly(t *testing.T) {
	image := filepath.Join(t.TempDir(), "image")
	err := dir.Create(image)
	require.NoError(t, err)

	type object struct {
		A string `json:"A"`
		B int64  `json:"B,omitempty"`
	}

	for _, test := range []struct {
		object object
	}{
		{object{}},
		{object{"a value", 100}},
		{object{"another value", 200}},
	} {
		engine, err := dir.Open(image)
		require.NoError(t, err, "open read-write engine")
		engineExt := NewEngine(engine)

		digest, _, err := engineExt.PutBlobJSON(t.Context(), test.object)
		require.NoError(t, err, "put blob json")

		err = engine.Close()
		require.NoError(t, err, "close engine")

		// make it readonly
		testhelpers.MakeReadOnly(t, image)

		newEngine, err := dir.Open(image)
		require.NoError(t, err, "open read-only engine")
		newEngineExt := NewEngine(newEngine)

		blobReader, err := newEngine.GetBlob(t.Context(), digest)
		require.NoError(t, err, "get blob")
		defer blobReader.Close() //nolint:errcheck

		gotBytes, err := io.ReadAll(blobReader)
		require.NoError(t, err, "read entire blob")

		var gotObject object
		err = json.Unmarshal(gotBytes, &gotObject)
		require.NoError(t, err, "unmarshal blob")
		assert.Equal(t, test.object, gotObject, "parsed json blob should match original data")

		// Make sure that writing again will FAIL.
		_, _, err = newEngineExt.PutBlobJSON(t.Context(), test.object)
		assert.Error(t, err, "put blob with read-only engine should fail") //nolint:testifylint // assert.*Error* makes more sense

		err = newEngine.Close()
		require.NoError(t, err, "close read-only engine")

		// make it readwrite again.
		testhelpers.MakeReadWrite(t, image)
	}
}
