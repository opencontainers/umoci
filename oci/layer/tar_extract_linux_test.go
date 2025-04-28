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

	fi, err := os.Stat(filepath.Join(dir, "file"))
	require.NoError(t, err, "failed to stat file")

	whiteout, err := isOverlayWhiteout(fi)
	require.NoError(t, err, "isOverlayWhiteout")
	assert.True(t, whiteout, "extract should make overlay whiteout")
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

	value := make([]byte, 10)
	n, err := unix.Getxattr(filepath.Join(dir, "dir"), "trusted.overlay.opaque", value)
	require.NoError(t, err, "get overlay opaque attr")
	assert.Equal(t, "y", string(value[:n]), "bad opaque attr")
}
