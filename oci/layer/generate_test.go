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

package layer

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbatts/go-mtree"
)

func TestGenerate(t *testing.T) {
	dir := t.TempDir()

	// Create some files and other fun things.
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644)
	require.NoError(t, err)

	// Get initial.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err, "mtree walk")

	// Modify some files.
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644)
	require.NoError(t, err)
	err = os.Remove(filepath.Join(dir, "some", "parents", "deleted"))
	require.NoError(t, err)

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	require.NoError(t, err, "mtree walk")

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	require.NoError(t, err, "mtree diff generate")

	reader, err := GenerateLayer(dir, diffs, &RepackOptions{})
	require.NoError(t, err, "generate layer")
	defer reader.Close()

	var (
		gotDeleted bool
		gotChanged bool
		gotDir     bool
	)

	tr := tar.NewReader(reader)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err, "read tar entry")

		switch hdr.Name {
		case filepath.Join("some", "parents") + "/":
			assert.EqualValues(t, tar.TypeDir, hdr.Typeflag, "directory tar entry should have a dir typeflag")
			gotDir = true
		case filepath.Join("some", "parents", ".wh.deleted"):
			assert.Empty(t, hdr.Size, "whiteout tar entry should be empty")
			gotDeleted = true
		case filepath.Join("some", "parents", "filechanged"):
			contents, err := ioutil.ReadAll(tr)
			if assert.NoError(t, err, "read file tar entry") {
				assert.EqualValues(t, "new contents", contents, "modified file should contain new contents")
			}
			gotChanged = true
		case filepath.Join("some", "fileunchanged"):
			t.Errorf("got unchanged file in diff layer!")
		default:
			t.Errorf("got unexpected file: %s", hdr.Name)
		}
	}

	assert.True(t, gotDeleted, "should see the deleted file")
	assert.True(t, gotChanged, "should see the changed file")
	assert.True(t, gotDir, "should see the directory")
}

// Make sure that opencontainers/umoci#33 doesn't regress.
func TestGenerateMissingFileError(t *testing.T) {
	dir := t.TempDir()

	// Create some files and other fun things.
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644)
	require.NoError(t, err)

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err)

	// Modify some files.
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644)
	require.NoError(t, err)
	err = os.Remove(filepath.Join(dir, "some", "parents", "deleted"))
	require.NoError(t, err)

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	require.NoError(t, err, "mtree walk")

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	require.NoError(t, err, "mtree diff generate")

	// Remove the changed file after getting the diffs. This will cause an error.
	err = os.Remove(filepath.Join(dir, "some", "parents", "filechanged"))
	require.NoError(t, err)

	// Generate a layer where the changed file is missing after the diff.
	reader, err := GenerateLayer(dir, diffs, &RepackOptions{})
	require.NoError(t, err, "generate layer")
	defer reader.Close()

	tr := tar.NewReader(reader)
	// TODO: Should we use assert.Eventually?
	for {
		_, err := tr.Next()
		require.NotErrorIs(t, err, io.EOF, "should get a real error before io.EOF")
		if err != nil {
			assert.ErrorIs(t, err, os.ErrNotExist, "should get enoent from GenerateLayer stream")
			break
		}
	}
}

// Make sure that opencontainers/umoci#33 doesn't regress.
func TestGenerateWrongRootError(t *testing.T) {
	dir := t.TempDir()

	// Create some files and other fun things.
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644)
	require.NoError(t, err)
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644)
	require.NoError(t, err)

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err)

	// Modify some files.
	err = ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644)
	require.NoError(t, err)
	err = os.Remove(filepath.Join(dir, "some", "parents", "deleted"))
	require.NoError(t, err)

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	require.NoError(t, err, "mtree walk")

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	require.NoError(t, err, "mtree diff generate")

	// Generate a layer with the wrong root directory.
	reader, err := GenerateLayer(filepath.Join(dir, "some"), diffs, &RepackOptions{})
	require.NoError(t, err, "generate layer")
	defer reader.Close()

	tr := tar.NewReader(reader)
	// TODO: Should we use assert.Eventually?
	for {
		_, err := tr.Next()
		require.NotErrorIs(t, err, io.EOF, "should get a real error before io.EOF")
		if err != nil {
			assert.ErrorIs(t, err, os.ErrNotExist, "should get enoent from GenerateLayer stream")
			break
		}
	}
}
