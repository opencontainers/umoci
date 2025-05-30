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
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci/internal/testhelpers"
)

type tarDentry struct {
	path     string
	ftype    byte
	linkname string
	xattrs   map[string]string
	contents string
}

func tarFromDentry(de tarDentry) (*tar.Header, io.Reader) {
	var r io.Reader
	var size int64
	if de.ftype == tar.TypeReg || de.ftype == tar.TypeRegA { //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
		size = int64(len(de.contents))
		r = bytes.NewBufferString(de.contents)
	}

	mode := os.FileMode(0o777)
	if de.ftype == tar.TypeDir {
		mode |= os.ModeDir
	}

	return &tar.Header{
		Name:       de.path,
		Linkname:   de.linkname,
		Typeflag:   de.ftype,
		Mode:       int64(mode),
		Size:       size,
		Xattrs:     de.xattrs, //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
		ModTime:    testhelpers.Unix(1210393, 4528036),
		AccessTime: testhelpers.Unix(7892829, 2341211),
		ChangeTime: testhelpers.Unix(8731293, 8218947),
	}, r
}

func checkLayerEntries(t *testing.T, rdr io.Reader, wantEntries []tarDentry) {
	tr := tar.NewReader(rdr)

	var sawEntries []tarDentry
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err, "read next tar header")

		contents, err := io.ReadAll(tr)
		require.NoErrorf(t, err, "read data after tar header for %q", hdr.Name)
		if hdr.Typeflag == tar.TypeReg || hdr.Typeflag == tar.TypeRegA { //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			assert.Lenf(t, contents, int(hdr.Size), "data for %q should have same size as in tar header", hdr.Name)
		} else {
			assert.Zerof(t, hdr.Size, "non-regular-file tar header for %q should have an empty size", hdr.Name)
			assert.Emptyf(t, contents, "non-regular-file tar header for %q should not have any data to read", hdr.Name)
		}

		sawEntries = append(sawEntries, tarDentry{
			path:     hdr.Name,
			ftype:    hdr.Typeflag,
			linkname: hdr.Linkname,
			xattrs:   hdr.Xattrs, //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
			contents: string(contents),
		})
	}
	assert.ElementsMatch(t, wantEntries, sawEntries, "generated archive entries")
}

func TestGenerate(t *testing.T) {
	dir := t.TempDir()

	// Create some files and other fun things.
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0o644)
	require.NoError(t, err)

	// Get initial.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err, "mtree walk")

	// Wait for a second to make sure that the the mtime of the directory gets
	// changed (in the GitHub Actions it seems the filesystem doesn't have
	// sub-second precision and so changing the directory without a delay
	// results in no diff entry being created for the directory).
	time.Sleep(1 * time.Second)

	// Modify some files.
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0o644)
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
	defer reader.Close() //nolint:errcheck

	checkLayerEntries(t, reader, []tarDentry{
		{path: "some/parents/", ftype: tar.TypeDir},
		{path: "some/parents/" + whPrefix + "deleted", ftype: tar.TypeReg},
		{path: "some/parents/filechanged", ftype: tar.TypeReg, contents: "new contents"},
	})
}

// Make sure that opencontainers/umoci#33 doesn't regress.
func TestGenerateMissingFileError(t *testing.T) {
	dir := t.TempDir()

	// Create some files and other fun things.
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0o644)
	require.NoError(t, err)

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err)

	// Modify some files.
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0o644)
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
	defer reader.Close() //nolint:errcheck

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
	err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0o644)
	require.NoError(t, err)

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	require.NoError(t, err)

	// Modify some files.
	err = os.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0o644)
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
	defer reader.Close() //nolint:errcheck

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

func TestGenerateInsertWhiteout(t *testing.T) {
	t.Run("WhiteoutPath", func(t *testing.T) {
		reader := GenerateInsertLayer("", "foo/bar/baz", false, nil)
		checkLayerEntries(t, reader, []tarDentry{
			{path: "foo/bar/" + whPrefix + "baz", ftype: tar.TypeReg},
		})
	})

	// opaque + whiteout should only result in a regular whiteout.
	t.Run("BothWhiteouts", func(t *testing.T) {
		reader := GenerateInsertLayer("", "foo/bar/baz", true, nil)
		checkLayerEntries(t, reader, []tarDentry{
			{path: "foo/bar/" + whPrefix + "baz", ftype: tar.TypeReg},
		})
	})
}
