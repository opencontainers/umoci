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
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

func TestEngineBlobJSON(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineBlobJSON")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)
	defer engine.Close()

	type object struct {
		A string `json:"A"`
		B int64  `json:"B,omitempty"`
	}

	for _, test := range []struct {
		object object
	}{
		{object{}},
		{object{"a value", 100}},
		{object{"another value", 200}},
	} {
		digest, _, err := engineExt.PutBlobJSON(ctx, test.object)
		if err != nil {
			t.Errorf("PutBlobJSON: unexpected error: %+v", err)
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

		var gotObject object
		if err := json.Unmarshal(gotBytes, &gotObject); err != nil {
			t.Errorf("GetBlob: got an invalid JSON blob: %+v", err)
		}
		if !reflect.DeepEqual(test.object, gotObject) {
			t.Errorf("GetBlob: got different object to original JSON. expected=%v got=%v gotBytes=%v", test.object, gotObject, gotBytes)
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

func TestEngineBlobJSONReadonly(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineBlobJSONReadonly")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	type object struct {
		A string `json:"A"`
		B int64  `json:"B,omitempty"`
	}

	for _, test := range []struct {
		object object
	}{
		{object{}},
		{object{"a value", 100}},
		{object{"another value", 200}},
	} {
		engine, err := dir.Open(image)
		if err != nil {
			t.Fatalf("unexpected error opening image: %+v", err)
		}
		engineExt := NewEngine(engine)

		digest, _, err := engineExt.PutBlobJSON(ctx, test.object)
		if err != nil {
			t.Errorf("PutBlobJSON: unexpected error: %+v", err)
		}

		if err := engine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered: %+v", err)
		}

		// make it readonly
		readonly(t, image)

		newEngine, err := dir.Open(image)
		if err != nil {
			t.Errorf("unexpected error opening ro image: %+v", err)
		}
		newEngineExt := NewEngine(newEngine)

		blobReader, err := newEngineExt.GetBlob(ctx, digest)
		if err != nil {
			t.Errorf("GetBlob: unexpected error: %+v", err)
		}
		defer blobReader.Close()

		gotBytes, err := ioutil.ReadAll(blobReader)
		if err != nil {
			t.Errorf("GetBlob: failed to ReadAll: %+v", err)
		}

		var gotObject object
		if err := json.Unmarshal(gotBytes, &gotObject); err != nil {
			t.Errorf("GetBlob: got an invalid JSON blob: %+v", err)
		}
		if !reflect.DeepEqual(test.object, gotObject) {
			t.Errorf("GetBlob: got different object to original JSON. expected=%v got=%v gotBytes=%v", test.object, gotObject, gotBytes)
		}

		// Make sure that writing again will FAIL.
		_, _, err = newEngineExt.PutBlobJSON(ctx, test.object)
		if err == nil {
			t.Errorf("PutBlob: expected error on ro image!")
		}

		if err := newEngine.Close(); err != nil {
			t.Errorf("Close: unexpected error encountered on ro: %+v", err)
		}

		// make it readwrite again.
		readwrite(t, image)
	}
}
