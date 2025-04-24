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

	whiteoutPath := filepath.Join(dir, "file")

	woType, isWo, err := isOverlayWhiteout(whiteoutPath, fseval.Default)
	require.NoError(t, err, "isOverlayWhiteout")
	assert.True(t, isWo, "extract should make overlay whiteout")
	assert.Equal(t, overlayWhiteoutPlain, woType, "extract should make a plain whiteout")
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

	whiteoutPath := filepath.Join(dir, "dir")

	val, err := system.Lgetxattr(whiteoutPath, "trusted.overlay.opaque")
	require.NoError(t, err, "get overlay opaque attr")
	assert.Equal(t, "y", string(val), "bad opaque attr")

	woType, isWo, err := isOverlayWhiteout(whiteoutPath, fseval.Default)
	require.NoError(t, err, "isOverlayWhiteout")
	assert.True(t, isWo, "extract should make overlay whiteout")
	assert.Equal(t, overlayWhiteoutOpaque, woType, "extract should make an opaque whiteout")
}
