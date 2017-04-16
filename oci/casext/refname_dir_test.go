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

package casext

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openSUSE/umoci/oci/cas"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func TestEngineReference(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReference")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := cas.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := cas.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := Engine{engine}
	defer engine.Close()

	for _, test := range []struct {
		name       string
		descriptor ispec.Descriptor
	}{
		{"ref1", ispec.Descriptor{}},
		{"ref2", ispec.Descriptor{MediaType: ispec.MediaTypeImageConfig, Digest: "sha256:032581de4629652b8653e4dbb2762d0733028003f1fc8f9edd61ae8181393a15", Size: 100}},
		{"ref3", ispec.Descriptor{MediaType: ispec.MediaTypeImageLayerNonDistributableGzip, Digest: "sha256:3c968ad60d3a2a72a12b864fa1346e882c32690cbf3bf3bc50ee0d0e4e39f342", Size: 8888}},
	} {
		if err := engineExt.UpdateReference(ctx, test.name, test.descriptor); err != nil {
			t.Errorf("PutReference: unexpected error: %+v", err)
		}

		gotDescriptors, err := engineExt.ResolveReference(ctx, test.name)
		if err != nil {
			t.Errorf("GetReference: unexpected error: %+v", err)
		}
		if len(gotDescriptors) != 1 {
			t.Errorf("GetReference: expected to get %d descriptors, got %d", 1, len(gotDescriptors))
		}
		gotDescriptor := gotDescriptors[0]

		if !reflect.DeepEqual(test.descriptor, gotDescriptor) {
			t.Errorf("GetReference: got different descriptor to original: expected=%v got=%v", test.descriptor, gotDescriptor)
		}

		if err := engineExt.DeleteReference(ctx, test.name); err != nil {
			t.Errorf("DeleteReference: unexpected error: %+v", err)
		}

		if _, err := engineExt.ResolveReference(ctx, test.name); !os.IsNotExist(errors.Cause(err)) {
			if err == nil {
				t.Errorf("GetReference: still got reference descriptor after DeleteReference!")
			} else {
				t.Errorf("GetReference: unexpected error: %+v", err)
			}
		}

		// DeleteBlob is idempotent. It shouldn't cause an error.
		if err := engineExt.DeleteReference(ctx, test.name); err != nil {
			t.Errorf("DeleteReference: unexpected error on double-delete: %+v", err)
		}
	}
}

func TestEngineReferenceReadonly(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReferenceReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := cas.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	for _, test := range []struct {
		name       string
		descriptor ispec.Descriptor
	}{
		{"ref1", ispec.Descriptor{}},
		{"ref2", ispec.Descriptor{MediaType: ispec.MediaTypeImageConfig, Digest: "sha256:032581de4629652b8653e4dbb2762d0733028003f1fc8f9edd61ae8181393a15", Size: 100}},
		{"ref3", ispec.Descriptor{MediaType: ispec.MediaTypeImageLayerNonDistributableGzip, Digest: "sha256:3c968ad60d3a2a72a12b864fa1346e882c32690cbf3bf3bc50ee0d0e4e39f342", Size: 8888}},
	} {

		engine, err := cas.Open(image)
		if err != nil {
			t.Fatalf("unexpected error opening image: %+v", err)
		}
		engineExt := Engine{engine}

		if err := engineExt.UpdateReference(ctx, test.name, test.descriptor); err != nil {
			t.Errorf("PutReference: unexpected error: %+v", err)
		}

		if err := engine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered: %+v", err)
		}

		// make it readonly
		readonly(t, image)

		newEngine, err := cas.Open(image)
		if err != nil {
			t.Errorf("unexpected error opening ro image: %+v", err)
		}
		newEngineExt := Engine{engine}

		gotDescriptors, err := newEngineExt.ResolveReference(ctx, test.name)
		if err != nil {
			t.Errorf("GetReference: unexpected error: %+v", err)
		}
		if len(gotDescriptors) != 1 {
			t.Errorf("GetReference: expected to get %d descriptors, got %d", 1, len(gotDescriptors))
		}
		gotDescriptor := gotDescriptors[0]

		if !reflect.DeepEqual(test.descriptor, gotDescriptor) {
			t.Errorf("GetReference: got different descriptor to original: expected=%v got=%v", test.descriptor, gotDescriptor)
		}

		// Make sure that writing will FAIL.
		if err := newEngineExt.UpdateReference(ctx, test.name+"new", test.descriptor); err == nil {
			t.Errorf("PutReference: expected error on ro image!")
		}

		if err := newEngine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered on ro: %+v", err)
		}

		// make it readwrite again.
		readwrite(t, image)
	}
}
