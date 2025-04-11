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
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/pkg/testutils"
)

// NOTE: These tests aren't really testing OCI-style manifests. It's all just
//       example structures to make sure that the CAS acts properly.

func TestCreateLayout(t *testing.T) {
	ctx := context.Background()

	root := t.TempDir()

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

	root := t.TempDir()

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

		if br, err := engine.GetBlob(ctx, digest); !errors.Is(err, os.ErrNotExist) {
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
	var (
		engine cas.Engine
		image  string
		err    error
	)

	// Empty directory.
	image = t.TempDir()
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Invalid oci-layout.
	image = t.TempDir()
	if err := ioutil.WriteFile(filepath.Join(image, layoutFile), []byte("invalid JSON"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Invalid oci-layout.
	image = t.TempDir()
	if err := ioutil.WriteFile(filepath.Join(image, layoutFile), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}

	// Missing blobdir.
	image = t.TempDir()
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
	image = t.TempDir()
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
	image = t.TempDir()
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
	image = t.TempDir()
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
	image = filepath.Join(t.TempDir(), "non-exist")
	engine, err = Open(image)
	if err == nil {
		t.Errorf("expected to get an error")
		engine.Close()
	}
}

// Make sure that opencontainers/umoci#63 doesn't have a regression. We
// shouldn't GC any blobs which are currently locked.
func TestEngineGCLocking(t *testing.T) {
	ctx := context.Background()

	root := t.TempDir()

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	content := []byte("here's some sample content")

	// Open a reference to the CAS, and make sure that it has a .temp set up.
	engine, err := Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}

	digester := cas.BlobAlgorithm.Digester()
	if _, err := io.Copy(digester.Hash(), bytes.NewReader(content)); err != nil {
		t.Fatalf("could not hash bytes: %+v", err)
	}
	expectedDigest := digester.Digest()

	digest, size, err := engine.PutBlob(ctx, bytes.NewReader(content))
	if err != nil {
		t.Errorf("PutBlob: unexpected error: %+v", err)
	}

	if digest != expectedDigest {
		t.Errorf("PutBlob: digest doesn't match: expected=%s got=%s", expectedDigest, digest)
	}
	if size != int64(len(content)) {
		t.Errorf("PutBlob: length doesn't match: expected=%d got=%d", len(content), size)
	}

	if engine.(*dirEngine).temp == "" {
		t.Errorf("engine doesn't have a tempdir after putting a blob!")
	}

	// Create umoci and other directories and files to make sure things work.
	umociTestDir, err := ioutil.TempDir(image, ".umoci-dead-")
	if err != nil {
		t.Fatal(err)
	}

	otherTestDir, err := ioutil.TempDir(image, "other-")
	if err != nil {
		t.Fatal(err)
	}

	// Open a new reference and GC it.
	gcEngine, err := Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}

	// TODO: This should be done with casext.GC...
	if err := gcEngine.Clean(ctx); err != nil {
		t.Fatalf("unexpected error while GCing image: %+v", err)
	}

	for _, path := range []string{
		engine.(*dirEngine).temp,
		otherTestDir,
	} {
		if _, err := os.Lstat(path); err != nil {
			t.Errorf("expected %s to still exist after GC: %+v", path, err)
		}
	}

	for _, path := range []string{
		umociTestDir,
	} {
		if _, err := os.Lstat(path); err == nil {
			t.Errorf("expected %s to not exist after GC", path)
		} else if !errors.Is(err, os.ErrNotExist) {
			t.Errorf("expected IsNotExist for %s after GC: %+v", path, err)
		}
	}
}

func TestCreateLayoutReadonly(t *testing.T) {
	ctx := context.Background()

	root := t.TempDir()

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	// make it readonly
	testutils.MakeReadOnly(t, image)
	defer testutils.MakeReadWrite(t, image)

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
}

func TestEngineBlobReadonly(t *testing.T) {
	ctx := context.Background()

	root := t.TempDir()

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	for _, test := range []struct {
		bytes []byte
	}{
		{[]byte("")},
		{[]byte("some blob")},
		{[]byte("another blob")},
	} {
		engine, err := Open(image)
		if err != nil {
			t.Fatalf("unexpected error opening image: %+v", err)
		}

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

		if err := engine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered: %+v", err)
		}

		// make it readonly
		testutils.MakeReadOnly(t, image)

		newEngine, err := Open(image)
		if err != nil {
			t.Errorf("unexpected error opening ro image: %+v", err)
		}

		blobReader, err := newEngine.GetBlob(ctx, digest)
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

		// Make sure that writing again will FAIL.
		_, _, err = newEngine.PutBlob(ctx, bytes.NewReader(test.bytes))
		if err == nil {
			t.Logf("PutBlob: e.temp = %s", newEngine.(*dirEngine).temp)
			t.Errorf("PutBlob: expected error on ro image!")
		}

		if err := newEngine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered on ro: %+v", err)
		}

		// make it readwrite again.
		testutils.MakeReadWrite(t, image)
	}
}
