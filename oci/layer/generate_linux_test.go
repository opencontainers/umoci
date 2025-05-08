//go:build linux
// +build linux

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

	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/system"
)

func TestTranslateOverlayWhiteouts_Char00(t *testing.T) {
	dir := t.TempDir()

	testNeedsMknod(t)

	err := system.Mknod(filepath.Join(dir, "test"), unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	require.NoError(t, err, "mknod")
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0644)
	require.NoError(t, err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}

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
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "test", ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "test", ftype: tar.TypeReg},
		})
	})
}

func TestTranslateOverlayWhiteouts_XattrOpaque(t *testing.T) {
	dir := t.TempDir()

	testNeedsTrustedOverlayXattrs(t)

	err := os.Mkdir(filepath.Join(dir, "wodir"), 0755)
	require.NoError(t, err)
	err = unix.Lsetxattr(filepath.Join(dir, "wodir"), "trusted.overlay.opaque", []byte("y"), 0)
	require.NoError(t, err, "lsetxattr trusted.overlay.opaque")
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0644)
	require.NoError(t, err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}

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
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: "wodir/", ftype: tar.TypeDir},
			{path: "wodir/" + whOpaque, ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: "wodir/", ftype: tar.TypeDir},
			{path: "wodir/" + whOpaque, ftype: tar.TypeReg},
		})
	})
}

func TestTranslateOverlayWhiteouts_XattrWhiteout(t *testing.T) {
	dir := t.TempDir()

	testNeedsTrustedOverlayXattrs(t)

	err := os.WriteFile(filepath.Join(dir, "woreg"), []byte{}, 0755)
	require.NoError(t, err)
	err = unix.Lsetxattr(filepath.Join(dir, "woreg"), "trusted.overlay.whiteout", []byte("foobar"), 0)
	require.NoError(t, err, "lsetxattr trusted.overlay.whiteout")
	err = os.WriteFile(filepath.Join(dir, "reg"), []byte("dummy file"), 0644)
	require.NoError(t, err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}

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
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: ".", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "woreg", ftype: tar.TypeReg},
		})
	})

	t.Run("GenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, "/", false, &packOptions)
		defer reader.Close()

		checkLayerEntries(t, reader, []tarDentry{
			{path: "/", ftype: tar.TypeDir},
			{path: "reg", ftype: tar.TypeReg, contents: "dummy file"},
			{path: whPrefix + "woreg", ftype: tar.TypeReg},
		})
	})
}
