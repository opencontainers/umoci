/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2018 Cisco Systems
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

package umoci

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestCreateExistingFails(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci_testCreateExistingFails")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// opening a bad layout should fail
	_, err = OpenLayout(dir)
	if err == nil {
		t.Fatal("opening non-existent layout succeeded?")
	}

	// remove directory so that create can create it
	os.RemoveAll(dir)

	// create should work
	_, err = CreateLayout(dir)
	if err != nil {
		t.Fatal(err)
	}

	// but not twice
	_, err = CreateLayout(dir)
	if err == nil {
		t.Fatal("create worked twice?")
	}

	// but open should work now
	_, err = OpenLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
}
