//go:build linux

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
	"archive/tar"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/pkg/fseval"
)

func testNeedsMknod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-inode")

	err := system.Mknod(path, unix.S_IFCHR|0o666, unix.Mkdev(0, 0))
	if errors.Is(err, os.ErrPermission) {
		t.Skipf("skipping test -- cannot mknod: %v", err)
	}
	require.NoError(t, err, "mknod should either succeed or error with ErrPermission")
}

func testNeedsTrustedOverlayXattrs(t *testing.T) {
	dir := t.TempDir()

	err := unix.Setxattr(dir, "trusted.overlay.opaque", []byte("y"), 0)
	if errors.Is(err, os.ErrPermission) {
		t.Skipf("skipping test -- cannot setxattr trusted.overlay.opaque: %v", err)
	}
	require.NoError(t, err, "setxattr trusted.overlay.opaque should succeed or error with ErrPermission")
}

func assertIsPlainWhiteout(t *testing.T, onDiskFmt OverlayfsRootfs, path string) {
	woType, isWo, err := isOverlayWhiteout(onDiskFmt, path, fseval.Default)
	require.NoErrorf(t, err, "isOverlayWhiteout({UserXattr: %v}, %q)", onDiskFmt.UserXattr, path)
	assert.True(t, isWo, "extract should make overlay whiteout")
	assert.Equal(t, overlayWhiteoutPlain, woType, "extract should make a plain whiteout")
}

func assertIsOpaqueWhiteout(t *testing.T, onDiskFmt OverlayfsRootfs, path string) {
	opaqueXattr := onDiskFmt.xattr("opaque")
	val, err := system.Lgetxattr(path, opaqueXattr)
	require.NoErrorf(t, err, "get overlay opaque attr for %q", path)
	assert.Equalf(t, "y", string(val), "bad opaque attr for %q", path)

	woType, isWo, err := isOverlayWhiteout(onDiskFmt, path, fseval.Default)
	require.NoErrorf(t, err, "isOverlayWhiteout({UserXattr: %v}, %q)", onDiskFmt.UserXattr, path)
	assert.Truef(t, isWo, "extract should make %q an overlay whiteout", path)
	assert.Equalf(t, overlayWhiteoutOpaque, woType, "extract should make %q an opaque whiteout", path)
}

func assertNoPathExists(t *testing.T, path string) {
	_, err := os.Lstat(path)
	assert.ErrorIsf(t, err, os.ErrNotExist, "path %q should not have existed", path)
}

func TestUnpackEntry_OverlayfsRootfs_Whiteout(t *testing.T) {
	testNeedsMknod(t)

	for _, userxattr := range []bool{true, false} {
		t.Run(fmt.Sprintf("UserXattr=%v", userxattr), func(t *testing.T) {
			dir := t.TempDir()

			onDiskFmt := OverlayfsRootfs{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
				UserXattr: userxattr,
			}

			dentries := []tarDentry{
				{path: "file", ftype: tar.TypeReg},
				{path: whPrefix + "file", ftype: tar.TypeReg},
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: onDiskFmt,
			}
			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			assertIsPlainWhiteout(t, onDiskFmt, filepath.Join(dir, "file"))
		})
	}
}

func TestUnpackEntry_OverlayfsRootfs_OpaqueWhiteout(t *testing.T) {
	testNeedsMknod(t)

	for _, userxattr := range []bool{true, false} {
		t.Run(fmt.Sprintf("UserXattr=%v", userxattr), func(t *testing.T) {
			dir := t.TempDir()

			if !userxattr {
				testNeedsTrustedOverlayXattrs(t)
			}

			onDiskFmt := OverlayfsRootfs{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
				UserXattr: userxattr,
			}

			dentries := []tarDentry{
				{path: "dir", ftype: tar.TypeDir},
				{path: "dir/fileindir", ftype: tar.TypeReg},
				{path: "dir/" + whOpaque, ftype: tar.TypeReg},
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: onDiskFmt,
			}
			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "dir"))
		})
	}
}

func TestUnpackEntry_OverlayfsRootfs_Whiteout_MissingDirs(t *testing.T) {
	testNeedsMknod(t)

	for _, userxattr := range []bool{true, false} {
		t.Run(fmt.Sprintf("UserXattr=%v", userxattr), func(t *testing.T) {
			dir := t.TempDir()

			if !userxattr {
				testNeedsTrustedOverlayXattrs(t)
			}

			onDiskFmt := OverlayfsRootfs{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
				UserXattr: userxattr,
			}

			dentries := []tarDentry{
				// entry with no parent directory entries
				{path: "opaque-noparent/a/b/c/" + whOpaque, ftype: tar.TypeReg},
				{path: "whiteout-noparent/a/b/" + whPrefix + "c", ftype: tar.TypeReg},
				// implicitly changing the type with an opaque dir
				{path: "opaque-nondir", ftype: tar.TypeReg},
				{path: "opaque-nondir/" + whOpaque, ftype: tar.TypeReg},
				// plain whiteout converted to opaque dir
				{path: whPrefix + "opaque-whiteout", ftype: tar.TypeReg},
				{path: "opaque-whiteout/" + whOpaque, ftype: tar.TypeReg},
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: onDiskFmt,
			}
			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-noparent/a/b/c"))
			assertIsPlainWhiteout(t, onDiskFmt, filepath.Join(dir, "whiteout-noparent/a/b/c"))
			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-nondir"))
			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-whiteout"))
		})
	}
}

func TestUnpackEntry_OverlayfsRootfs_Whiteout_Nested(t *testing.T) {
	testNeedsMknod(t)

	for _, userxattr := range []bool{true, false} {
		t.Run(fmt.Sprintf("UserXattr=%v", userxattr), func(t *testing.T) {
			dir := t.TempDir()

			if !userxattr {
				testNeedsTrustedOverlayXattrs(t)
			}

			onDiskFmt := OverlayfsRootfs{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
				UserXattr: userxattr,
			}

			dentries := []tarDentry{
				// make sure that whiteouts before and after are all missing from the
				// final layer
				{path: "opaque-innerplain/foo/bar/baz/" + whPrefix + "before", ftype: tar.TypeReg},
				{path: "opaque-innerplain/regfile", ftype: tar.TypeReg},
				{path: "opaque-innerplain/" + whOpaque, ftype: tar.TypeReg},
				{path: "opaque-innerplain/a/b/c/d/e/f/" + whPrefix + "after", ftype: tar.TypeReg},
				{path: "opaque-innerplain/a/b/c/d/regfile", ftype: tar.TypeReg},
				// nested opaque directories should stay opaque
				{path: "opaque-nested/a/b/c/d/" + whOpaque, ftype: tar.TypeReg},
				{path: "opaque-nested/a/b/" + whOpaque, ftype: tar.TypeReg},
				{path: "opaque-nested/" + whOpaque, ftype: tar.TypeReg},
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: onDiskFmt,
			}
			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-innerplain"))
			assertNoPathExists(t, filepath.Join(dir, "opaque-innerplain/foo/bar/baz/before"))
			assertNoPathExists(t, filepath.Join(dir, "opaque-innerplain/a/b/c/d/e/f/after"))
			assert.FileExists(t, filepath.Join(dir, "opaque-innerplain/a/b/c/d/regfile"))
			assert.FileExists(t, filepath.Join(dir, "opaque-innerplain/regfile"))

			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-nested"))
			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-nested/a/b"))
			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "opaque-nested/a/b/c/d"))
		})
	}
}

func TestUnpackEntry_OverlayfsRootfs_OpaqueWhiteoutConvert(t *testing.T) {
	testNeedsMknod(t)

	for _, userxattr := range []bool{true, false} {
		t.Run(fmt.Sprintf("UserXattr=%v", userxattr), func(t *testing.T) {
			dir := t.TempDir()

			if !userxattr {
				testNeedsTrustedOverlayXattrs(t)
			}

			onDiskFmt := OverlayfsRootfs{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
				UserXattr: userxattr,
			}

			dentries := []tarDentry{
				{path: "autoconvert-opaquedir/before", ftype: tar.TypeReg},
				// a directory that got deleted...
				{path: whPrefix + "autoconvert-opaquedir", ftype: tar.TypeReg},
				// ... should be converted to opaque if we create a subdir
				{path: "autoconvert-opaquedir/foo/bar", ftype: tar.TypeReg},
				{path: "autoconvert-opaquedir/abc/def", ftype: tar.TypeReg},
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: onDiskFmt,
			}
			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			assertIsOpaqueWhiteout(t, onDiskFmt, filepath.Join(dir, "autoconvert-opaquedir"))
			assertNoPathExists(t, filepath.Join(dir, "autoconvert-opaquedir/before"))
			assert.FileExists(t, filepath.Join(dir, "autoconvert-opaquedir/foo/bar"))
			assert.FileExists(t, filepath.Join(dir, "autoconvert-opaquedir/abc/def"))
		})
	}
}
