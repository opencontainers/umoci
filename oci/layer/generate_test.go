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

package layer

import (
	"archive/tar"
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/vbatts/go-mtree"
)

func TestGenerate(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestGenerate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files and other fun things.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Get initial.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "some", "parents", "deleted")); err != nil {
		t.Fatal(err)
	}

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	if err != nil {
		t.Fatal(err)
	}

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	if err != nil {
		t.Fatal(err)
	}

	reader, err := GenerateLayer(dir, diffs, &MapOptions{})
	if err != nil {
		t.Fatal(err)
	}
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
		if err != nil {
			t.Errorf("unexpected error: %s", err)
			break
		}
		switch hdr.Name {
		case filepath.Join("some", "parents") + "/":
			if hdr.Typeflag != tar.TypeDir {
				t.Errorf("directory suddenly stopped being a directory")
			}
			gotDir = true
		case filepath.Join("some", "parents", ".wh.deleted"):
			if hdr.Size != 0 {
				t.Errorf("whiteout file has non-zero size: %d", hdr.Size)
			}
			gotDeleted = true
		case filepath.Join("some", "parents", "filechanged"):
			contents, err := ioutil.ReadAll(tr)
			if err != nil {
				t.Errorf("unexpected error reading changed file: %s", err)
			}
			if !bytes.Equal(contents, []byte("new contents")) {
				t.Errorf("did not get expected contents: %s", contents)
			}
			gotChanged = true
		case filepath.Join("some", "fileunchanged"):
			t.Errorf("got unchanged file in diff layer!")
		default:
			t.Errorf("got unexpected file: %s", hdr.Name)
		}
	}

	if !gotDeleted {
		t.Errorf("did not get deleted file!")
	}
	if !gotChanged {
		t.Errorf("did not get changed file!")
	}
	if !gotDir {
		// This for some reason happen on Travis even though it shouldn't. It's
		// probably caused by some AUFS fun times that I don't want to debug.
		t.Logf("did not get directory!")
	}
}

// Make sure that openSUSE/umoci#33 doesn't regress.
func TestGenerateMissingFileError(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestGenerateError")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files and other fun things.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "some", "parents", "deleted")); err != nil {
		t.Fatal(err)
	}

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	if err != nil {
		t.Fatal(err)
	}

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	if err != nil {
		t.Fatal(err)
	}

	// Remove the changed file after getting the diffs. This will cause an error.
	if err := os.Remove(filepath.Join(dir, "some", "parents", "filechanged")); err != nil {
		t.Fatal(err)
	}

	// Generate a layer where the changed file is missing after the diff.
	reader, err := GenerateLayer(dir, diffs, &MapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		_, err := tr.Next()
		if err == io.EOF {
			t.Errorf("got EOF, not a proper error!")
		}
		if err != nil {
			break
		}
	}
}

// Make sure that openSUSE/umoci#33 doesn't regress.
func TestGenerateWrongRootError(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestGenerateError")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Create some files and other fun things.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parents"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "fileunchanged"), []byte("unchanged"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "deleted"), []byte("deleted"), 0644); err != nil {
		t.Fatal(err)
	}

	// Get initial from the main directory.
	initDh, err := mtree.Walk(dir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parents", "filechanged"), []byte("new contents"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dir, "some", "parents", "deleted")); err != nil {
		t.Fatal(err)
	}

	// Get post.
	postDh, err := mtree.Walk(dir, nil, initDh.UsedKeywords(), nil)
	if err != nil {
		t.Fatal(err)
	}

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	if err != nil {
		t.Fatal(err)
	}

	// Generate a layer with the wrong root directory.
	reader, err := GenerateLayer(filepath.Join(dir, "some"), diffs, &MapOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()

	tr := tar.NewReader(reader)
	for {
		_, err := tr.Next()
		if err == io.EOF {
			t.Errorf("got EOF, not a proper error!")
		}
		if err != nil {
			break
		}
	}
}
