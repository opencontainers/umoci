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

package system

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

// specialXattrs come from the OCI xattr remapping logic in oci/layer/xattr.go.
// These are xattrs that will be auto-set by the system and we should ignore
// their existence when doing xattr-related operations (for our tests, this
// especially means to take care of them when checking sets of xattrs that
// exist on the filesystem).
var specialXattrs = map[string]struct{}{
	"security.selinux": {},
	"system.nfs4_acl":  {},
}

func TestClearxattrFilter(t *testing.T) {
	dir := t.TempDir()

	file, err := os.CreateTemp(dir, "TestClearxattrFilter")
	require.NoError(t, err)
	defer file.Close() //nolint:errcheck

	path := file.Name()
	defer os.RemoveAll(path) //nolint:errcheck

	autosetXattrs, err := Llistxattr(path)
	require.NoErrorf(t, err, "llistxattr %q", path)
	for _, xattr := range autosetXattrs {
		require.Contains(t, specialXattrs, xattr, "auto-set xattrs must be part of special list")
	}

	xattrs := []struct {
		name, value string
		forbidden   bool
	}{
		{"user.allowed1", "test", false},
		{"user.allowed2", "test", false},
		{"user.forbidden1", "test", true},
		{"user.forbidden1.allowed", "test", false},
	}

	setXattrNames := []string{}
	forbiddenXattrNames := []string{}
	forbiddenXattrs := make(map[string]struct{})

	for _, xattr := range xattrs {
		setXattrNames = append(setXattrNames, xattr.name)
		if xattr.forbidden {
			forbiddenXattrNames = append(forbiddenXattrNames, xattr.name)
			forbiddenXattrs[xattr.name] = struct{}{}
		}

		err := unix.Lsetxattr(path, xattr.name, []byte(xattr.value), 0)
		if errors.Is(err, unix.ENOTSUP) {
			t.Skipf("xattrs unsupported on %s backing filesystem", dir)
		}
		require.NoErrorf(t, err, "lsetxattr %q=%q on %q", xattr.name, xattr.value, path)
	}
	// If we are running on an SELinux-enabled system, all new files get a
	// security.selinux xattr that gets auto-set and cannot be removed so we
	// need to include it in the expected set.
	expectAllXattrNames := append(setXattrNames, autosetXattrs...)
	expectRemainingXattrNames := append(forbiddenXattrNames, autosetXattrs...)

	// Check they're all present.
	allXattrList, err := Llistxattr(path)
	require.NoErrorf(t, err, "llistxattr %q", path)
	assert.ElementsMatch(t, expectAllXattrNames, allXattrList, "all xattrs should be present after setting")

	// Now clear them.
	err = Lclearxattrs(path, func(xattrName string) bool {
		_, ok := forbiddenXattrs[xattrName]
		return ok
	})
	require.NoErrorf(t, err, "lclearxattrs %q (forbidden=%v)", path, forbiddenXattrs)

	// Check that only the forbidden ones remain.
	remainingXattrList, err := Llistxattr(path)
	require.NoErrorf(t, err, "llistxattr %q", path)
	assert.NotElementsMatch(t, expectAllXattrNames, remainingXattrList, "there should be a different set of xattrs after clearing")
	assert.ElementsMatch(t, expectRemainingXattrNames, remainingXattrList, "only explicitly forbidden xattrs should be allowed to remain after clearing")
	assert.NotEmpty(t, remainingXattrList, "there should be some remaining xattrs after clearing")
}
