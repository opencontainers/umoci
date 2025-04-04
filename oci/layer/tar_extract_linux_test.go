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

func canMknod(dir string) (bool, error) {
	testNode := filepath.Join(dir, "test")
	err := system.Mknod(testNode, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	if err != nil {
		if errors.Is(err, os.ErrPermission) {
			return false, nil
		}

		return false, err
	}
	return true, os.Remove(testNode)
}

func TestUnpackEntryOverlayFSWhiteout(t *testing.T) {
	dir := t.TempDir()

	mknodOk, err := canMknod(dir)
	require.NoError(t, err, "check if can mknod")

	if !mknodOk {
		t.Skip("skipping overlayfs test on kernel < 5.8")
	}

	headers := []pseudoHdr{
		{"file", "", tar.TypeReg},
		{whPrefix + "file", "", tar.TypeReg},
	}

	canSetTrustedXattrs := os.Geteuid() == 0

	if canSetTrustedXattrs {
		headers = append(headers, []pseudoHdr{
			{"dir", "", tar.TypeDir},
			{"dir/fileindir", "dir", tar.TypeReg},
			{"dir/" + whOpaque, "dir", tar.TypeReg},
		}...)
	}

	unpackOptions := UnpackOptions{
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, ph := range headers {
		hdr, rdr := fromPseudoHdr(ph)
		err := te.UnpackEntry(dir, hdr, rdr)
		assert.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	fi, err := os.Stat(filepath.Join(dir, "file"))
	require.NoError(t, err, "failed to stat file")

	whiteout, err := isOverlayWhiteout(fi)
	require.NoError(t, err, "isOverlayWhiteout")
	assert.True(t, whiteout, "extract should make overlay whiteout")

	if canSetTrustedXattrs {
		value := make([]byte, 10)
		n, err := unix.Getxattr(filepath.Join(dir, "dir"), "trusted.overlay.opaque", value)
		require.NoError(t, err, "get overlay opaque attr")
		assert.Equal(t, "y", string(value[:n]), "bad opaque attr")
	}
}
