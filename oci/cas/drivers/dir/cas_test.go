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

package dir

import (
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/openSUSE/umoci/oci/cas"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// NOTE: These tests aren't really testing OCI-style manifests. It's all just
//       example structures to make sure that the CAS acts properly.

func TestCreateLayout(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestCreateLayout")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	defer engine.Close()

	// We should have an empty index and no blobs.
	if index, err := engine.GetIndex(ctx); err != nil {
		t.Errorf("unexpected error getting top-level index: %+v", err)
	} else if len(index.Manifests) > 0 {
		t.Errorf("got manifests in top-level index in a newly created image: %v", index.Manifests)
	}
	if blobs, err := engine.ListBlobs(ctx); err != nil {
		t.Errorf("unexpected error getting list of blobs: %+v", err)
	} else if len(blobs) > 0 {
		t.Errorf("got blobs in a newly created image: %v", blobs)
	}

	// We should get an error if we try to create a new image atop an old one.
	if err := Create(image); err == nil {
		t.Errorf("expected to get a cowardly no-clobber error!")
	}
}

func TestEngineBlob(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineBlob")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	defer engine.Close()

	for _, test := range []struct {
		bytes []byte
	}{
		{[]byte("")},
		{[]byte("some blob")},
		{[]byte("another blob")},
	} {
		digester := cas.BlobAlgorithm.Digester()
		if _, err := io.Copy(digester.Hash(), bytes.NewReader(test.bytes)); err != nil {
			t.Fatalf("could not hash bytes: %+v", err)
		}
		expectedDigest := digester.Digest()

		digest, size, err := engine.PutBlob(ctx, bytes.NewReader(test.bytes))
		if err != nil {
			t.Errorf("PutBlob: unexpected error: %+v", err)
		}

		if digest != expectedDigest {
			t.Errorf("PutBlob: digest doesn't match: expected=%s got=%s", expectedDigest, digest)
		}
		if size != int64(len(test.bytes)) {
			t.Errorf("PutBlob: length doesn't match: expected=%d got=%d", len(test.bytes), size)
		}

		blobReader, err := engine.GetBlob(ctx, digest)
		if err != nil {
			t.Errorf("GetBlob: unexpected error: %+v", err)
		}
		defer blobReader.Close()

		gotBytes, err := ioutil.ReadAll(blobReader)
		if err != nil {
			t.Errorf("GetBlob: failed to ReadAll: %+v", err)
		}
		if !bytes.Equal(test.bytes, gotBytes) {
			t.Errorf("GetBlob: bytes did not match: expected=%s got=%s", string(test.bytes), string(gotBytes))
		}

		if err := engine.DeleteBlob(ctx, digest); err != nil {
			t.Errorf("DeleteBlob: unexpected error: %+v", err)
		}

		if br, err := engine.GetBlob(ctx, digest); !os.IsNotExist(errors.Cause(err)) {
			if err == nil {
				br.Close()
				t.Errorf("GetBlob: still got blob contents after DeleteBlob!")
			} else {
				t.Errorf("GetBlob: unexpected error: %+v", err)
			}
		}

		// DeleteBlob is idempotent. It shouldn't cause an error.
		if err := engine.DeleteBlob(ctx, digest); err != nil {
			t.Errorf("DeleteBlob: unexpected error on double-delete: %+v", err)
		}
	}

	// Should be no blobs left.
	if blobs, err := engine.ListBlobs(ctx); err != nil {
		t.Errorf("unexpected error getting list of blobs: %+v", err)
	} else if len(blobs) > 0 {
		t.Errorf("got blobs in a clean image: %v", blobs)
	}
}

func TestEngineValidate(t *testing.T) {
	root, err := ioutil.TempDir("", "umoci-TestEngineValidate")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	var engine cas.Engine
	var image string

	// Empty directory.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Invalid oci-layout.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(image, layoutFile), []byte("invalid JSON"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Invalid oci-layout.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(image, layoutFile), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Missing blobdir.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(image); err != nil {
		t.Fatal(err)
	}
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}
	if err := os.RemoveAll(filepath.Join(image, blobDirectory)); err != nil {
		t.Fatalf("unexpected error deleting blobdir: %+v", err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// blobdir is not a directory.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(image); err != nil {
		t.Fatal(err)
	}
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}
	if err := os.RemoveAll(filepath.Join(image, blobDirectory)); err != nil {
		t.Fatalf("unexpected error deleting blobdir: %+v", err)
	}
	if err := ioutil.WriteFile(filepath.Join(image, blobDirectory), []byte(""), 0755); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Missing index.json.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(image); err != nil {
		t.Fatal(err)
	}
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}
	if err := os.RemoveAll(filepath.Join(image, indexFile)); err != nil {
		t.Fatalf("unexpected error deleting index: %+v", err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// index is not a valid file.
	image, err = ioutil.TempDir(root, "image")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(image); err != nil {
		t.Fatal(err)
	}
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}
	if err := os.RemoveAll(filepath.Join(image, indexFile)); err != nil {
		t.Fatalf("unexpected error deleting index: %+v", err)
	}
	if err := os.Mkdir(filepath.Join(image, indexFile), 0755); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// No such directory.
	image = filepath.Join(root, "non-exist")
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}
}
