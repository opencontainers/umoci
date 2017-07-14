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
	"syscall"
	"testing"

	"github.com/openSUSE/umoci/oci/cas"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// NOTE: These tests aren't really testing OCI-style manifests. It's all just
//       example structures to make sure that the CAS acts properly.

// readonly makes the given path read-only (by bind-mounting it as "ro").
// TODO: This should be done through an interface restriction in the test
//       (which is then backed up by the readonly mount if necessary). The fact
//       this test is necessary is a sign that we need a better split up of the
//       CAS interface.
func readonly(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Log("readonly tests only work with root privileges")
		t.Skip()
	}

	t.Logf("mounting %s as readonly", path)

	if err := syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
	if err := syscall.Mount("none", path, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
}

// readwrite undoes the effect of readonly.
func readwrite(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Log("readonly tests only work with root privileges")
		t.Skip()
	}

	if err := syscall.Unmount(path, syscall.MNT_DETACH); err != nil {
		t.Fatalf("unmount %s: %s", path, err)
	}
}

func TestCreateLayoutReadonly(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestCreateLayoutReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	// make it readonly
	readonly(t, image)
	defer readwrite(t, image)

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

	root, err := ioutil.TempDir("", "umoci-TestEngineBlobReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

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
		readonly(t, image)

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
		readwrite(t, image)
	}
}

// Make sure that openSUSE/umoci#63 doesn't have a regression where we start
// deleting files and directories that other people are using.
func TestEngineGCLocking(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestCreateLayoutReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

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

	// Create tempdir to make sure things work.
	removedDir, err := ioutil.TempDir(image, "testdir")
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

	// Make sure that engine.temp is still around.
	if _, err := os.Lstat(engine.(*dirEngine).temp); err != nil {
		t.Errorf("expected active direngine.temp to still exist after GC: %+v", err)
	}

	// Make sure that removedDir is still around
	if _, err := os.Lstat(removedDir); err == nil {
		t.Errorf("expected inactive temporary dir to not exist after GC")
	} else if !os.IsNotExist(errors.Cause(err)) {
		t.Errorf("expected IsNotExist for temporary dir after GC: %+v", err)
	}
}
