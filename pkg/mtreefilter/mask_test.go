/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2017 SUSE LLC.
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

func TestIsParent(t *testing.T) {
	for _, test := range []struct {
		parent, path string
		expected     bool
	}{
		{"/", "/a", true},
		{"/", "/a/b/c", true},
		{"/", "/", true},
		{"/a path/", "/a path", true},
		{"/a nother path", "/a nother path/test", true},
		{"/a nother path", "/a nother path/test/1   2/  33 /", true},
		{"/path1", "/path2", false},
		{"/pathA", "/PATHA", false},
		{"/pathC", "/path", false},
		{"/path9", "/", false},
		// Make sure it's not the same as filepath.HasPrefix.
		{"/a/b/c", "/a/b/c/d", true},
		{"/a/b/c", "/a/b/cd", false},
		{"/a/b/c", "/a/bcd", false},
		{"/a/bc", "/a/bcd", false},
		{"/abc", "/abcd", false},
	} {
		got := isParent(test.parent, test.path)
		if got != test.expected {
			t.Errorf("isParent(%q, %q) got %v expected %v", test.parent, test.path, got, test.expected)
		}
	}
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
		newDiff := FilterDeltas(diff, MaskFilter(test.paths))
		for _, delta := range newDiff {
			if len(test.paths) == 0 {
				if len(newDiff) != len(diff) {
					t.Errorf("expected diff={} to give %d got %d", len(diff), len(newDiff))
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
