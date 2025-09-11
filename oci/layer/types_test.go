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

package layer

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUnpackOptionsHelpers(t *testing.T) {
	expected := &UnpackOptions{
		OnDiskFormat: DirRootfs{},
	}

	t.Run("NilStruct", func(t *testing.T) {
		nilOpt := (*UnpackOptions)(nil)
		got := nilOpt.fill()

		assert.Equal(t, expected, got, "nil UnpackOptions.fill should fill nil values")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after UnpackOptions.fill")
		assert.Nil(t, nilOpt, "nil UnpackOptions.fill won't affect original value")
	})

	t.Run("ZeroStruct", func(t *testing.T) {
		zeroOpt := &UnpackOptions{}
		got := zeroOpt.fill()

		assert.Equal(t, expected, got, "zero UnpackOptions.fill should fill nil values")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after UnpackOptions.fill")
		assert.Equal(t, expected, zeroOpt, "zero UnpackOptions.fill should update original struct")

		assert.Zero(t, UnpackOptions{}.MapOptions(), "MapOptions should return zero-value for nil OnDiskFormat")
	})

	t.Run("ExplicitDirRootfs", func(t *testing.T) {
		onDiskFmt := DirRootfs{MapOptions: MapOptions{Rootless: true}}
		dirOpt := &UnpackOptions{
			OnDiskFormat: onDiskFmt,
		}
		got := dirOpt.fill()

		assert.NotEqual(t, expected, got, "UnpackOptions.fill should not modify already-set OnDiskFormat")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after UnpackOptions.fill")
		assert.Equal(t, onDiskFmt, got.OnDiskFormat, "UnpackOptions.fill should not modify already-set OnDiskFormat")
		assert.Equal(t, onDiskFmt, dirOpt.OnDiskFormat, "UnpackOptions.fill should not modify already-set OnDiskFormat")

		assert.Equal(t, onDiskFmt.MapOptions, dirOpt.MapOptions(), "MapOptions should match actual MapOptions")
	})

	t.Run("OverlayfsRootfs", func(t *testing.T) {
		onDiskFmt := DirRootfs{MapOptions: MapOptions{Rootless: true}}
		overlayOpt := &UnpackOptions{
			OnDiskFormat: onDiskFmt,
		}
		got := overlayOpt.fill()

		assert.NotEqual(t, expected, got, "UnpackOptions.fill should not modify already-set OnDiskFormat")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after UnpackOptions.fill")
		assert.Equal(t, onDiskFmt, got.OnDiskFormat, "UnpackOptions.fill should not modify already-set OnDiskFormat")
		assert.Equal(t, onDiskFmt, overlayOpt.OnDiskFormat, "UnpackOptions.fill should not modify already-set OnDiskFormat")

		assert.Equal(t, onDiskFmt.MapOptions, overlayOpt.MapOptions(), "MapOptions should match actual MapOptions")
	})
}

func TestRepackOptionsHelpers(t *testing.T) {
	expected := &RepackOptions{
		OnDiskFormat: DirRootfs{},
	}

	t.Run("NilStruct", func(t *testing.T) {
		nilOpt := (*RepackOptions)(nil)
		got := nilOpt.fill()

		assert.Equal(t, expected, got, "nil RepackOptions.fill should fill nil values")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after RepackOptions.fill")
		assert.Nil(t, nilOpt, "nil RepackOptions.fill won't affect original value")
	})

	t.Run("ZeroStruct", func(t *testing.T) {
		zeroOpt := &RepackOptions{}
		got := zeroOpt.fill()

		assert.Equal(t, expected, got, "zero RepackOptions.fill should fill nil values")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after RepackOptions.fill")
		assert.Equal(t, expected, zeroOpt, "zero RepackOptions.fill should update original struct")
	})

	t.Run("ExplicitDirRootfs", func(t *testing.T) {
		onDiskFmt := DirRootfs{MapOptions: MapOptions{Rootless: true}}
		dirOpt := &RepackOptions{
			OnDiskFormat: onDiskFmt,
		}
		got := dirOpt.fill()

		assert.NotEqual(t, expected, got, "RepackOptions.fill should not modify already-set OnDiskFormat")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after RepackOptions.fill")
		assert.Equal(t, onDiskFmt, got.OnDiskFormat, "RepackOptions.fill should not modify already-set OnDiskFormat")
		assert.Equal(t, onDiskFmt, dirOpt.OnDiskFormat, "RepackOptions.fill should not modify already-set OnDiskFormat")

		assert.Equal(t, onDiskFmt.MapOptions, dirOpt.MapOptions(), "MapOptions should match actual MapOptions")
	})

	t.Run("OverlayfsRootfs", func(t *testing.T) {
		onDiskFmt := DirRootfs{MapOptions: MapOptions{Rootless: true}}
		overlayOpt := &RepackOptions{
			OnDiskFormat: onDiskFmt,
		}
		got := overlayOpt.fill()

		assert.NotEqual(t, expected, got, "RepackOptions.fill should not modify already-set OnDiskFormat")
		assert.NotNil(t, got.OnDiskFormat, "OnDiskFormat must be non-nil after RepackOptions.fill")
		assert.Equal(t, onDiskFmt, got.OnDiskFormat, "RepackOptions.fill should not modify already-set OnDiskFormat")
		assert.Equal(t, onDiskFmt, overlayOpt.OnDiskFormat, "RepackOptions.fill should not modify already-set OnDiskFormat")

		assert.Equal(t, onDiskFmt.MapOptions, overlayOpt.MapOptions(), "MapOptions should match actual MapOptions")
	})
}

func TestOverlayfsRootfs_XattrNamespace(t *testing.T) {
	for _, test := range []struct {
		onDiskFmt         OverlayfsRootfs
		expectedNamespace string
	}{
		{OverlayfsRootfs{UserXattr: false}, "trusted."},
		{OverlayfsRootfs{UserXattr: true}, "user."},
	} {
		t.Run(fmt.Sprintf("UserXattr=%v", test.onDiskFmt.UserXattr), func(t *testing.T) {
			assert.Equalf(t, test.expectedNamespace, test.onDiskFmt.xattrNamespace(), "OverlayfsRootfs{UserXattr: %v}.xattrNamespace()", test.onDiskFmt.UserXattr)
		})
	}
}

func TestOverlayfsRootfs_Xattr(t *testing.T) {
	for _, test := range []struct {
		name      string
		onDiskFmt OverlayfsRootfs
		subxattr  []string
		expected  string
	}{
		{"TrustedXattr-NoArgs", OverlayfsRootfs{UserXattr: false}, []string{}, "trusted.overlay."},
		{"UserXattr-NoArgs", OverlayfsRootfs{UserXattr: true}, []string{}, "user.overlay."},
		{"TrustedXattr-SubXattr", OverlayfsRootfs{UserXattr: false}, []string{"opaque"}, "trusted.overlay.opaque"},
		{"UserXattr-SubXattr", OverlayfsRootfs{UserXattr: true}, []string{"whiteout"}, "user.overlay.whiteout"},
		{"TrustedXattr-MultiSubXattr", OverlayfsRootfs{UserXattr: false}, []string{"foo", "bar.baz", "boop"}, "trusted.overlay.foo.bar.baz.boop"},
		{"UserXattr-MultiSubXattr", OverlayfsRootfs{UserXattr: true}, []string{"abc", "def", "ghi.jkl."}, "user.overlay.abc.def.ghi.jkl."},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := test.onDiskFmt.xattr(test.subxattr...)
			assert.Equalf(t, test.expected, got, "OverlayfsRootfs{UserXattr: %v}.xattr(%#v)", test.onDiskFmt.UserXattr, test.subxattr)
		})
	}
}
