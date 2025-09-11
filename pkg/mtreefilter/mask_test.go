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

package mtreefilter

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbatts/go-mtree"
)

func isParent(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)

	for a != b && b != filepath.Dir(b) {
		b = filepath.Dir(b)
	}
	return a == b
}

func TestMaskDeltas(t *testing.T) {
	dir := t.TempDir()

	mtreeKeywords := append(mtree.DefaultKeywords, "sha256digest")

	// Create some files.
	err := os.WriteFile(filepath.Join(dir, "file1"), []byte("contents"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "file2"), []byte("another content"), 0o644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(dir, "dir", "child"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(dir, "dir", "child2"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "dir", "file 3"), []byte("more content"), 0o644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "dir", "child2", "4 files"), []byte("very content"), 0o644)
	require.NoError(t, err)

	// Generate a diff.
	initDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	require.NoError(t, err, "mtree walk")

	// Modify the root.
	err = os.RemoveAll(filepath.Join(dir, "file2"))
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "dir", "new"), []byte("more content"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "file1"), []byte("different contents"), 0o666)
	require.NoError(t, err)

	// Generate the set of diffs.
	postDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	require.NoError(t, err, "mtree walk")

	diff, err := mtree.Compare(initDh, postDh, mtreeKeywords)
	require.NoError(t, err, "mtree diff generate")

	for _, test := range []struct {
		name  string
		paths []string
	}{
		{"NilFilter", nil},
		{"EmptyFilter", []string{}},
		{"Root", []string{"/"}},
		{"Dir", []string{"dir"}},
		{"UntouchedSubpath", []string{filepath.Join("dir", "child2")}},
		{"File", []string{"file2"}},
		{"Overlapping", []string{"/", "file2"}},
		{"Multiple", []string{"file2", filepath.Join("dir", "child2")}},
	} {
		t.Run(test.name, func(t *testing.T) {
			simpleDiff := FilterDeltas(diff, MaskFilter(test.paths))
			for _, delta := range simpleDiff {
				if len(test.paths) == 0 {
					assert.Equal(t, diff, simpleDiff, "noop filter should not modify diff")
				} else {
					for _, path := range test.paths {
						assert.Falsef(t, isParent(path, delta.Path()), "delta entry %q should not have a parent path in the filter list but %q is in the list", delta.Path(), path)
					}
				}
			}
		})
	}
}

func TestSimplifyFilter(t *testing.T) {
	dir := t.TempDir()

	mtreeKeywords := append(mtree.DefaultKeywords, "sha256digest")

	// Create some nested directories we can remove.
	err := os.MkdirAll(filepath.Join(dir, "some", "path", "to", "remove"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "path", "to", "remove", "child"), []byte("very content"), 0o644)
	require.NoError(t, err)

	// Generate a diff.
	initDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	require.NoError(t, err, "mtree walk")

	// Modify the root.
	err = os.RemoveAll(filepath.Join(dir, "some"))
	require.NoError(t, err)

	// Generate the set of diffs.
	postDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	require.NoError(t, err, "mtree walk")

	diff, err := mtree.Compare(initDh, postDh, mtreeKeywords)
	require.NoError(t, err, "mtree diff generate")

	// We expect to see a deletion for each entry.
	var sawDeletions int
	for _, delta := range diff {
		if delta.Type() == mtree.Missing {
			sawDeletions++
		}
	}
	assert.Equal(t, 5, sawDeletions, "should see 5 deletions with stock Compare")

	// Simplify the diffs.
	simpleDiff := FilterDeltas(diff, SimplifyFilter(diff))
	require.Less(t, len(simpleDiff), len(diff), "SimplifyFilter diff should be smaller than original diff")
	var sawSimpleDeletions int
	for _, delta := range simpleDiff {
		if delta.Type() == mtree.Missing {
			sawSimpleDeletions++
		}
	}
	assert.Equal(t, 1, sawSimpleDeletions, "should only see 1 deletion with SimplifyFilter")
}
