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

package mtreefilter

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

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
	dir, err := ioutil.TempDir("", "TestMaskDeltas-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mtreeKeywords := append(mtree.DefaultKeywords, "sha256digest")

	// Create some files.
	if err != ioutil.WriteFile(filepath.Join(dir, "file1"), []byte("contents"), 0644) {
		t.Fatal(err)
	}
	if err != ioutil.WriteFile(filepath.Join(dir, "file2"), []byte("another content"), 0644) {
		t.Fatal(err)
	}
	if err != os.MkdirAll(filepath.Join(dir, "dir", "child"), 0755) {
		t.Fatal(err)
	}
	if err != os.MkdirAll(filepath.Join(dir, "dir", "child2"), 0755) {
		t.Fatal(err)
	}
	if err != ioutil.WriteFile(filepath.Join(dir, "dir", "file 3"), []byte("more content"), 0644) {
		t.Fatal(err)
	}
	if err != ioutil.WriteFile(filepath.Join(dir, "dir", "child2", "4 files"), []byte("very content"), 0644) {
		t.Fatal(err)
	}

	// Generate a diff.
	originalDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the root.
	if err := os.RemoveAll(filepath.Join(dir, "file2")); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "dir", "new"), []byte("more content"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "file1"), []byte("different contents"), 0666); err != nil {
		t.Fatal(err)
	}

	// Generate the set of diffs.
	newDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := mtree.Compare(originalDh, newDh, mtreeKeywords)
	if err != nil {
		t.Fatal(err)
	}

	for _, test := range []struct {
		paths []string
	}{
		{nil},
		{[]string{"/"}},
		{[]string{"dir"}},
		{[]string{filepath.Join("dir", "child2")}},
		{[]string{"file2"}},
		{[]string{"/", "file2"}},
		{[]string{"file2", filepath.Join("dir", "child2")}},
	} {
		simpleDiff := FilterDeltas(diff, MaskFilter(test.paths))
		for _, delta := range simpleDiff {
			if len(test.paths) == 0 {
				if len(simpleDiff) != len(diff) {
					t.Errorf("expected diff={} to give %d got %d", len(diff), len(simpleDiff))
				}
			} else {
				found := false
				for _, path := range test.paths {
					if !isParent(path, delta.Path()) {
						found = true
					}
				}
				if !found {
					t.Errorf("expected one of %v to not be a parent of %q", test.paths, delta.Path())
				}
			}
		}
	}
}

func TestSimplifyFilter(t *testing.T) {
	dir, err := ioutil.TempDir("", "TestSimplifyFilter-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	mtreeKeywords := append(mtree.DefaultKeywords, "sha256digest")

	// Create some nested directories we can remove.
	if err != os.MkdirAll(filepath.Join(dir, "some", "path", "to", "remove"), 0755) {
		t.Fatal(err)
	}
	if err != ioutil.WriteFile(filepath.Join(dir, "some", "path", "to", "remove", "child"), []byte("very content"), 0644) {
		t.Fatal(err)
	}

	// Generate a diff.
	originalDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Modify the root.
	if err := os.RemoveAll(filepath.Join(dir, "some")); err != nil {
		t.Fatal(err)
	}

	// Generate the set of diffs.
	newDh, err := mtree.Walk(dir, nil, mtreeKeywords, nil)
	if err != nil {
		t.Fatal(err)
	}
	diff, err := mtree.Compare(originalDh, newDh, mtreeKeywords)
	if err != nil {
		t.Fatal(err)
	}

	// We expect to see a deletion for each entry.
	var sawDeletions int
	for _, delta := range diff {
		if delta.Type() == mtree.Missing {
			sawDeletions++
		}
	}
	if sawDeletions != 5 {
		t.Errorf("expected to see 5 deletions with stock Compare, saw %v", sawDeletions)
	}

	// Simplify the diffs.
	simpleDiff := FilterDeltas(diff, SimplifyFilter(diff))
	if len(simpleDiff) >= len(diff) {
		t.Errorf("expected simplified diff to be shorter (%v >= %v)", len(simpleDiff), len(diff))
	}
	var sawSimpleDeletions int
	for _, delta := range simpleDiff {
		if delta.Type() == mtree.Missing {
			sawSimpleDeletions++
		}
	}
	if sawSimpleDeletions != 1 {
		t.Errorf("expected to see 1 deletion with simplified filter, saw %v", sawSimpleDeletions)
	}
}
