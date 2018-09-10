/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017, 2018 SUSE LLC.
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

package system

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

func TestClearxattrFilter(t *testing.T) {
	file, err := ioutil.TempFile("", "TestClearxattrFilter")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	path := file.Name()
	defer os.RemoveAll(path)

	xattrs := []struct {
		name, value string
		forbidden   bool
	}{
		{"user.allowed1", "test", false},
		{"user.allowed2", "test", false},
		{"user.forbidden1", "test", true},
		{"user.forbidden1.allowed", "test", false},
	}

	allXattrCount := make(map[string]int)
	forbiddenXattrCount := make(map[string]int)
	forbiddenXattrs := make(map[string]struct{})

	for _, xattr := range xattrs {
		allXattrCount[xattr.name] = 0
		if xattr.forbidden {
			forbiddenXattrCount[xattr.name] = 0
			forbiddenXattrs[xattr.name] = struct{}{}
		}

		if err := unix.Lsetxattr(path, xattr.name, []byte(xattr.value), 0); err != nil {
			if errors.Cause(err) == unix.ENOTSUP {
				t.Skip("xattrs unsupported on backing filesystem")
			}
			t.Fatalf("unexpected error setting %v=%v on %v: %v", xattr.name, xattr.value, path, err)
		}
	}

	// Check they're all present.
	allXattrList, err := Llistxattr(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, xattr := range allXattrList {
		if _, ok := allXattrCount[xattr]; !ok {
			t.Errorf("saw unexpected xattr in all list: %q", xattr)
		} else {
			allXattrCount[xattr]++
		}
	}
	for xattr, count := range allXattrCount {
		if count != 1 {
			t.Errorf("all xattr count inconsistent: saw %q %v times", xattr, count)
		}
	}

	// Now clear them.
	if err := Lclearxattrs(path, forbiddenXattrs); err != nil {
		t.Fatal(err)
	}

	// Check that only the forbidden ones remain.
	forbiddenXattrList, err := Llistxattr(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, xattr := range forbiddenXattrList {
		if _, ok := forbiddenXattrCount[xattr]; !ok {
			t.Errorf("saw unexpected xattr in forbidden list: %q", xattr)
		} else {
			forbiddenXattrCount[xattr]++
		}
	}
	for xattr, count := range forbiddenXattrCount {
		if count != 1 {
			t.Errorf("forbidden xattr count inconsistent: saw %q %v times", xattr, count)
		}
	}
}
