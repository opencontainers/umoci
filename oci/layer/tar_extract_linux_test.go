//go:build linux
// +build linux

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
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/system"
)

func testNeedsMknod(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test-inode")

	err := system.Mknod(path, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
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

func assertIsPlainWhiteout(t *testing.T, path string) {
	woType, isWo, err := isOverlayWhiteout(path, fseval.Default)
	require.NoErrorf(t, err, "isOverlayWhiteout(%q)", path)
	assert.True(t, isWo, "extract should make overlay whiteout")
	assert.Equal(t, overlayWhiteoutPlain, woType, "extract should make a plain whiteout")
}

func assertIsOpaqueWhiteout(t *testing.T, path string) {
	val, err := system.Lgetxattr(path, "trusted.overlay.opaque")
	require.NoErrorf(t, err, "get overlay opaque attr for %q", path)
	assert.Equalf(t, "y", string(val), "bad opaque attr for %q", path)

	woType, isWo, err := isOverlayWhiteout(path, fseval.Default)
	require.NoErrorf(t, err, "isOverlayWhiteout(%q)", path)
	assert.Truef(t, isWo, "extract should make %q an overlay whiteout", path)
	assert.Equalf(t, overlayWhiteoutOpaque, woType, "extract should make %q an opaque whiteout", path)
}

func assertNoPathExists(t *testing.T, path string) {
	_, err := os.Lstat(path)
	assert.ErrorIsf(t, err, os.ErrNotExist, "path %q should not have existed", path)
}

func TestUnpackEntry_OverlayFSWhiteout(t *testing.T) {
	dir := t.TempDir()

	testNeedsMknod(t)

	dentries := []tarDentry{
		{path: "file", ftype: tar.TypeReg},
		{path: whPrefix + "file", ftype: tar.TypeReg},
	}

	unpackOptions := UnpackOptions{
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, de := range dentries {
		hdr, rdr := tarFromDentry(de)
		err := te.UnpackEntry(dir, hdr, rdr)
		assert.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	assertIsPlainWhiteout(t, filepath.Join(dir, "file"))
}

func TestUnpackEntry_OverlayFSOpaqueWhiteout(t *testing.T) {
	dir := t.TempDir()

	testNeedsMknod(t)
	testNeedsTrustedOverlayXattrs(t)

	dentries := []tarDentry{
		{path: "dir", ftype: tar.TypeDir},
		{path: "dir/fileindir", ftype: tar.TypeReg},
		{path: "dir/" + whOpaque, ftype: tar.TypeReg},
	}

	unpackOptions := UnpackOptions{
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, de := range dentries {
		hdr, rdr := tarFromDentry(de)
		err := te.UnpackEntry(dir, hdr, rdr)
		assert.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	assertIsOpaqueWhiteout(t, filepath.Join(dir, "dir"))
}

func TestUnpackEntry_OverlayFSWhiteout_MissingDirs(t *testing.T) {
	dir := t.TempDir()

	testNeedsMknod(t)
	testNeedsTrustedOverlayXattrs(t)

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
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, de := range dentries {
		hdr, rdr := tarFromDentry(de)
		err := te.UnpackEntry(dir, hdr, rdr)
		assert.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-noparent/a/b/c"))
	assertIsPlainWhiteout(t, filepath.Join(dir, "whiteout-noparent/a/b/c"))
	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-nondir"))
	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-whiteout"))
}

func TestUnpackEntry_OverlayFSWhiteout_Nested(t *testing.T) {
	dir := t.TempDir()

	testNeedsMknod(t)
	testNeedsTrustedOverlayXattrs(t)

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
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, de := range dentries {
		hdr, rdr := tarFromDentry(de)
		err := te.UnpackEntry(dir, hdr, rdr)
		assert.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-innerplain"))
	assertNoPathExists(t, filepath.Join(dir, "opaque-innerplain/foo/bar/baz/before"))
	assertNoPathExists(t, filepath.Join(dir, "opaque-innerplain/a/b/c/d/e/f/after"))
	assert.FileExists(t, filepath.Join(dir, "opaque-innerplain/a/b/c/d/regfile"))
	assert.FileExists(t, filepath.Join(dir, "opaque-innerplain/regfile"))

	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-nested"))
	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-nested/a/b"))
	assertIsOpaqueWhiteout(t, filepath.Join(dir, "opaque-nested/a/b/c/d"))
}
