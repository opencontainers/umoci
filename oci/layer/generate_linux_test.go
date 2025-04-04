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
	"io"
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/system"
)

func TestInsertLayerTranslateOverlayWhiteouts(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	mknodOk, err := canMknod(dir)
	if err != nil {
		t.Fatalf("couldn't mknod in dir: %v", err)
	}

	if !mknodOk {
		t.Skip("skipping overlayfs test on kernel < 5.8")
	}

	testNode := path.Join(dir, "test")
	err = system.Mknod(testNode, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	assert.NoError(err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}
	reader := GenerateInsertLayer(dir, "/", false, &packOptions)
	defer reader.Close()

	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	assert.NoError(err)
	assert.Equal(hdr.Name, "/")

	hdr, err = tr.Next()
	assert.NoError(err)

	assert.Equal(int32(hdr.Typeflag), int32(tar.TypeReg))
	assert.Equal(hdr.Name, whPrefix+"test")
	_, err = tr.Next()
	assert.Equal(err, io.EOF)
}

func TestGenerateLayerTranslateOverlayWhiteouts(t *testing.T) {
	assert := assert.New(t)
	dir := t.TempDir()

	mknodOk, err := canMknod(dir)
	if err != nil {
		t.Fatalf("couldn't mknod in dir: %v", err)
	}

	if !mknodOk {
		t.Skip("skipping overlayfs test on kernel < 5.8")
	}

	testNode := path.Join(dir, "test")
	err = system.Mknod(testNode, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	assert.NoError(err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}
	// something reasonable
	mtreeKeywords := []mtree.Keyword{
		"size",
		"type",
		"uid",
		"gid",
		"mode",
	}
	deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
	assert.NoError(err)

	reader, err := GenerateLayer(dir, deltas, &packOptions)
	assert.NoError(err)
	defer reader.Close()

	tr := tar.NewReader(reader)

	hdr, err := tr.Next()
	assert.NoError(err)

	assert.Equal(int32(hdr.Typeflag), int32(tar.TypeReg))
	assert.Equal(path.Base(hdr.Name), whPrefix+"test")
	_, err = tr.Next()
	assert.Equal(err, io.EOF)
}
