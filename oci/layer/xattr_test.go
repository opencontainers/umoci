// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
 * Copyright (C) 2020 Cisco Inc.
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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDoesXattrMatch(t *testing.T) {
	for _, test := range []struct {
		filter, xattr string
		expected      bool
	}{
		// exact match mode
		{"foo.bar", "foo.bar", true},
		{"foo.bar", "foo.bara", false},
		{"foo.bar", "foo.bar.a", false},
		{"foo.bar", "foo.bar.a.b.c", false},
		// prefix mode
		{"foo.bar.", "foo.bar", false},
		{"foo.bar.", "foo.bara", false},
		{"foo.bar.", "foo.bar.a", true},
		{"foo.bar.", "foo.bar.a.b.c", true},
	} {
		got := doesXattrMatch(test.xattr, test.filter)
		assert.Equalf(t, test.expected, got, "doesXattrMatch(%q, %q)", test.xattr, test.filter)
	}
}

func TestGetXattrFilter(t *testing.T) {
	for _, test := range []struct {
		xattr          string
		expectedFilter xattrFilter
		expectedOk     bool
	}{
		// exact matches
		{"security.selinux", forbiddenXattrFilter{}, true},
		{"security.selinux.foo", nil, false},
		{"security.sel", nil, false},
		{"security.capability", nil, false},
		{"system.nfs4_acl", forbiddenXattrFilter{}, true},
		{"system.nfs4_foo", nil, false},
		{"system.foo", nil, false},
		// prefixes
		{"trusted.overlay.opaque", overlayXattrFilter{"trusted."}, true},
		{"trusted.overlay.whiteout", overlayXattrFilter{"trusted."}, true},
		{"trusted.overlay.foobar", overlayXattrFilter{"trusted."}, true},
		{"trusted.overlay.a.b.c", overlayXattrFilter{"trusted."}, true},
		{"trusted.overlay", nil, false},
		{"user.overlay.opaque", overlayXattrFilter{"user."}, true},
		{"user.overlay.whiteout", overlayXattrFilter{"user."}, true},
		{"user.overlay.foobar", overlayXattrFilter{"user."}, true},
		{"user.overlay.a.b.c", overlayXattrFilter{"user."}, true},
		{"user.overlay", nil, false},
		// unrelated
		{"user.foo.bar", nil, false},
		{"user.trusted.overlay.opaque", nil, false},
		{"user.rootlesscontainers", nil, false},
	} {
		filter, ok := getXattrFilter(test.xattr)
		assert.Equalf(t, test.expectedOk, ok, "getXattrFilter(%q)", test.xattr)
		assert.Equalf(t, test.expectedFilter, filter, "getXattrFilter(%q)", test.xattr)
	}
}

func TestOverlayXattrFilter(t *testing.T) {
	for _, test := range []struct {
		name          string
		xattr         string
		onDiskFmt     OnDiskFormat
		toDisk, toTar string
	}{
		{"NormalXattr", "trusted.example.xattr", OverlayfsRootfs{}, "trusted.example.xattr", "trusted.example.xattr"},
		// With UserXattr == false, only trusted.overlay.* xattrs are affected by the filter.
		{"TrustedOverlayXattr", "trusted.overlay.foo", OverlayfsRootfs{}, "trusted.overlay.overlay.foo", ""},
		{"TrustedOverlayXattr-Escaped", "trusted.overlay.overlay.foo", OverlayfsRootfs{}, "trusted.overlay.overlay.overlay.foo", "trusted.overlay.foo"},
		{"UserOverlayXattr", "user.overlay.foo", OverlayfsRootfs{}, "user.overlay.foo", "user.overlay.foo"},
		{"UserOverlayXattr-Escaped", "user.overlay.overlay.foo", OverlayfsRootfs{}, "user.overlay.overlay.foo", "user.overlay.overlay.foo"},
		// But with UserXattr == true, only user.overlay.* xattrs are affected by the filter.
		{"UserXattr-TrustedOverlayXattr", "trusted.overlay.foo", OverlayfsRootfs{UserXattr: true}, "trusted.overlay.foo", "trusted.overlay.foo"},
		{"UserXattr-TrustedOverlayXattr-Escaped", "trusted.overlay.overlay.foo", OverlayfsRootfs{UserXattr: true}, "trusted.overlay.overlay.foo", "trusted.overlay.overlay.foo"},
		{"UserXattr-UserOverlayXattr", "user.overlay.foo", OverlayfsRootfs{UserXattr: true}, "user.overlay.overlay.foo", ""},
		{"UserXattr-UserOverlayXattr-Escaped", "user.overlay.overlay.foo", OverlayfsRootfs{UserXattr: true}, "user.overlay.overlay.overlay.foo", "user.overlay.foo"},
	} {
		t.Run(test.name, func(t *testing.T) {
			filter, ok := getXattrFilter(test.xattr)
			if !ok {
				// For test purposes, use a dummy overlayXattrFilter if the
				// xattr is not the right xattr.
				filter = overlayXattrFilter{"trusted."}
			}

			expectMasked := test.toTar == ""
			gotMasked := filter.MaskedOnDisk(test.onDiskFmt, test.xattr)
			assert.Equal(t, expectMasked, gotMasked, "MaskedOnDisk(%#v, %q)", test.onDiskFmt, test.xattr)

			gotToDisk := filter.ToDisk(test.onDiskFmt, test.xattr)
			assert.Equal(t, test.toDisk, gotToDisk, "ToDisk(%#v, %q)", test.onDiskFmt, test.xattr)

			gotToTar := filter.ToTar(test.onDiskFmt, test.xattr)
			assert.Equal(t, test.toTar, gotToTar, "ToTar(%#v, %q)", test.onDiskFmt, test.xattr)
		})
	}
}
