//go:build linux

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
	"archive/tar"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/pkg/fseval"
)

func testTranslateOverlayWhiteouts_Char00(t *testing.T, onDiskFmt OverlayfsRootfs) { //nolint:revive // var-naming is less important than matching the func TestXyz name
	dir := t.TempDir()

	testNeedsMknod(t)

	err := system.Mknod(filepath.Join(dir, "test"), unix.S_IFCHR|0o666, unix.Mkdev(0, 0))
	require.NoError(t, err, "mknod")
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0o644)
	require.NoError(t, err)

	packOptions := RepackOptions{OnDiskFormat: onDiskFmt}

	t.Run("GenerateLayer", func(t *testing.T) {
		// something reasonable
		mtreeKeywords := []mtree.Keyword{
			"size",
			"type",
			"uid",
			"gid",
			"mode",
		}
		deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
		require.NoError(t, err, "mtree check")

		reader, err := GenerateLayer(dir, deltas, &packOptions)
		require.NoError(t, err, "generate layer")
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "test", ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "test", ftype: tar.TypeReg},
		})
	})
}

func TestTranslateOverlayWhiteouts_Char00(t *testing.T) {
	t.Run("trusted.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_Char00(t, OverlayfsRootfs{})
	})

	t.Run("user.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_Char00(t, OverlayfsRootfs{UserXattr: true})
	})
}

func testTranslateOverlayWhiteouts_XattrOpaque(t *testing.T, onDiskFmt OverlayfsRootfs) { //nolint:revive // var-naming is less important than matching the func TestXyz name
	dir := t.TempDir()

	if !onDiskFmt.UserXattr {
		testNeedsTrustedOverlayXattrs(t)
	}
	opaqueXattr := onDiskFmt.xattr("opaque")

	err := os.Mkdir(filepath.Join(dir, "wodir"), 0o755)
	require.NoError(t, err)
	err = unix.Lsetxattr(filepath.Join(dir, "wodir"), opaqueXattr, []byte("y"), 0)
	require.NoErrorf(t, err, "lsetxattr %s", opaqueXattr)
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0o644)
	require.NoError(t, err)

	packOptions := RepackOptions{OnDiskFormat: onDiskFmt}

	t.Run("GenerateLayer", func(t *testing.T) {
		// something reasonable
		mtreeKeywords := []mtree.Keyword{
			"size",
			"type",
			"uid",
			"gid",
			"mode",
		}
		deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
		require.NoError(t, err, "mtree check")

		reader, err := GenerateLayer(dir, deltas, &packOptions)
		require.NoError(t, err, "generate layer")
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: "wodir/", ftype: tar.TypeDir},
			{path: "wodir/" + whOpaque, ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: "wodir/", ftype: tar.TypeDir},
			{path: "wodir/" + whOpaque, ftype: tar.TypeReg},
		})
	})
}

func TestTranslateOverlayWhiteouts_XattrOpaque(t *testing.T) {
	t.Run("trusted.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_XattrOpaque(t, OverlayfsRootfs{})
	})

	t.Run("user.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_XattrOpaque(t, OverlayfsRootfs{UserXattr: true})
	})
}

func testTranslateOverlayWhiteouts_XattrWhiteout(t *testing.T, onDiskFmt OverlayfsRootfs) { //nolint:revive // var-naming is less important than matching the func TestXyz name
	dir := t.TempDir()

	if !onDiskFmt.UserXattr {
		testNeedsTrustedOverlayXattrs(t)
	}
	whiteoutXattr := onDiskFmt.xattr("whiteout")

	err := os.WriteFile(filepath.Join(dir, "woreg"), []byte{}, 0o755)
	require.NoError(t, err)
	err = unix.Lsetxattr(filepath.Join(dir, "woreg"), whiteoutXattr, []byte("foobar"), 0)
	require.NoErrorf(t, err, "lsetxattr %s", whiteoutXattr)
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0o644)
	require.NoError(t, err)

	packOptions := RepackOptions{OnDiskFormat: onDiskFmt}

	t.Run("GenerateLayer", func(t *testing.T) {
		// something reasonable
		mtreeKeywords := []mtree.Keyword{
			"size",
			"type",
			"uid",
			"gid",
			"mode",
		}
		deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
		require.NoError(t, err, "mtree check")

		reader, err := GenerateLayer(dir, deltas, &packOptions)
		require.NoError(t, err, "generate layer")
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "woreg", ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close() //nolint:errcheck

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "woreg", ftype: tar.TypeReg},
		})
	})
}

func TestTranslateOverlayWhiteouts_XattrWhiteout(t *testing.T) {
	t.Run("trusted.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_XattrWhiteout(t, OverlayfsRootfs{})
	})

	t.Run("user.overlay", func(t *testing.T) {
		testTranslateOverlayWhiteouts_XattrWhiteout(t, OverlayfsRootfs{UserXattr: true})
	})
}
