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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTarGenerateAddFileNormal(t *testing.T) {
	reader, writer := io.Pipe()

	dir := t.TempDir()

	file := "file"
	data := []byte("this is a normal file")
	path := filepath.Join(dir, file)

	expectedHdr := &tar.Header{
		Name:       file,
		Mode:       0o644,
		ModTime:    time.Unix(123, 0),
		AccessTime: time.Unix(888, 0),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Typeflag:   tar.TypeReg,
		Size:       int64(len(data)),
	}

	te := NewTarExtractor(UnpackOptions{})
	err := os.WriteFile(path, data, 0o777)
	require.NoError(t, err)
	err = te.applyMetadata(path, expectedHdr)
	require.NoError(t, err, "apply metadata")

	tg := newTarGenerator(writer, RepackOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		err := tg.AddFile(file, path)
		assert.NoErrorf(t, err, "AddFile %s", path)

		err = tg.tw.Close()
		assert.NoError(t, err, "close tar writer")

		err = writer.Close()
		assert.NoError(t, err, "close pipe writer")
	}()

	hdr, err := tr.Next()
	require.NoError(t, err, "read tar entry")

	// TODO: Can we switch to just doing assert.Equal for the entire header?
	assert.Equal(t, expectedHdr.Typeflag, hdr.Typeflag, "generated tar header Typeflag")
	assert.Equal(t, expectedHdr.Name, hdr.Name, "generated tar header Name")
	assert.Empty(t, hdr.Linkname, "generated tar header Linkname")
	assert.Equal(t, expectedHdr.Uid, hdr.Uid, "generated tar header Uid")
	assert.Equal(t, expectedHdr.Gid, hdr.Gid, "generated tar header Gid")
	assert.Equal(t, expectedHdr.Size, hdr.Size, "generated tar header Size")
	assert.Equal(t, expectedHdr.ModTime, hdr.ModTime, "generated tar header ModTime")

	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if hdr.AccessTime.IsZero() {
		t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
	} else {
		assert.Equal(t, expectedHdr.AccessTime, hdr.AccessTime, "generated tar header AccessTime")
	}

	gotBytes, err := io.ReadAll(tr)
	require.NoError(t, err, "read file data from tar reader")
	assert.Equal(t, data, gotBytes, "file data from tar reader should match input")

	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF, "should reach end of tar archive")
}

func TestTarGenerateAddFileDirectory(t *testing.T) {
	reader, writer := io.Pipe()

	dir := t.TempDir()

	file := "directory/"
	path := filepath.Join(dir, file)

	expectedHdr := &tar.Header{
		Name:       file,
		Mode:       0o644,
		ModTime:    time.Unix(123, 0),
		AccessTime: time.Unix(888, 0),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Typeflag:   tar.TypeDir,
		Size:       0,
	}

	te := NewTarExtractor(UnpackOptions{})
	err := os.Mkdir(path, 0o777)
	require.NoError(t, err)
	err = te.applyMetadata(path, expectedHdr)
	require.NoError(t, err, "apply metadata")

	tg := newTarGenerator(writer, RepackOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		err := tg.AddFile(file, path)
		assert.NoErrorf(t, err, "AddFile %s", path)

		err = tg.tw.Close()
		assert.NoError(t, err, "close tar writer")

		err = writer.Close()
		assert.NoError(t, err, "close pipe writer")
	}()

	hdr, err := tr.Next()
	require.NoError(t, err, "read tar entry")

	// TODO: Can we switch to just doing assert.Equal for the entire header?
	assert.Equal(t, expectedHdr.Typeflag, hdr.Typeflag, "generated tar header Typeflag")
	assert.Equal(t, expectedHdr.Name, hdr.Name, "generated tar header Name")
	assert.Empty(t, hdr.Linkname, "generated tar header Linkname")
	assert.Equal(t, expectedHdr.Uid, hdr.Uid, "generated tar header Uid")
	assert.Equal(t, expectedHdr.Gid, hdr.Gid, "generated tar header Gid")
	assert.Equal(t, expectedHdr.Size, hdr.Size, "generated tar header Size")
	assert.Equal(t, expectedHdr.ModTime, hdr.ModTime, "generated tar header ModTime")

	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if hdr.AccessTime.IsZero() {
		t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
	} else {
		assert.Equal(t, expectedHdr.AccessTime, hdr.AccessTime, "generated tar header AccessTime")
	}

	gotBytes, err := io.ReadAll(tr)
	require.NoError(t, err, "read file data from tar reader")
	assert.Empty(t, gotBytes, "directory should have no tar data")

	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF, "should reach end of tar archive")
}

func TestTarGenerateAddFileSymlink(t *testing.T) {
	reader, writer := io.Pipe()

	dir := t.TempDir()

	file := "link"
	linkname := "/test"
	path := filepath.Join(dir, file)

	expectedHdr := &tar.Header{
		Name:       file,
		Linkname:   linkname,
		ModTime:    time.Unix(123, 0),
		AccessTime: time.Unix(888, 0),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Typeflag:   tar.TypeSymlink,
		Size:       0,
	}

	te := NewTarExtractor(UnpackOptions{})
	err := os.Symlink(linkname, path)
	require.NoError(t, err)
	err = te.applyMetadata(path, expectedHdr)
	require.NoError(t, err, "apply metadata")

	tg := newTarGenerator(writer, RepackOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		err := tg.AddFile(file, path)
		assert.NoErrorf(t, err, "AddFile %s", path)

		err = tg.tw.Close()
		assert.NoError(t, err, "close tar writer")

		err = writer.Close()
		assert.NoError(t, err, "close pipe writer")
	}()

	hdr, err := tr.Next()
	require.NoError(t, err, "read tar entry")

	// TODO: Can we switch to just doing assert.Equal for the entire header?
	assert.Equal(t, expectedHdr.Typeflag, hdr.Typeflag, "generated tar header Typeflag")
	assert.Equal(t, expectedHdr.Name, hdr.Name, "generated tar header Name")
	assert.Equal(t, expectedHdr.Linkname, hdr.Linkname, "generated tar header Linkname")
	assert.Equal(t, expectedHdr.Uid, hdr.Uid, "generated tar header Uid")
	assert.Equal(t, expectedHdr.Gid, hdr.Gid, "generated tar header Gid")
	assert.Equal(t, expectedHdr.Size, hdr.Size, "generated tar header Size")
	assert.Equal(t, expectedHdr.ModTime, hdr.ModTime, "generated tar header ModTime")

	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if hdr.AccessTime.IsZero() {
		t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
	} else {
		assert.Equal(t, expectedHdr.AccessTime, hdr.AccessTime, "generated tar header AccessTime")
	}

	gotBytes, err := io.ReadAll(tr)
	require.NoError(t, err, "read file data from tar reader")
	assert.Empty(t, gotBytes, "directory should have no tar data")

	_, err = tr.Next()
	assert.ErrorIs(t, err, io.EOF, "should reach end of tar archive")
}

func parseWhiteout(path string) (string, error) {
	path = filepath.Clean(path)
	dir, file := filepath.Split(path)
	if !strings.HasPrefix(file, whPrefix) {
		return "", fmt.Errorf("not a whiteout path: %s", path)
	}
	return filepath.Join(dir, strings.TrimPrefix(file, whPrefix)), nil
}

func TestTarGenerateAddWhiteout(t *testing.T) {
	reader, writer := io.Pipe()

	// Paths we want to generate whiteouts for.
	paths := []string{
		"root",
		"dir/file",
		"dir/",
		"dir/.",
	}

	tg := newTarGenerator(writer, RepackOptions{})
	tr := tar.NewReader(reader)

	// Create all of the whiteout entries in a goroutine so we can parse the
	// tar entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		for _, path := range paths {
			err := tg.AddWhiteout(path)
			assert.NoErrorf(t, err, "AddWhitout %s", path)
		}

		err := tg.tw.Close()
		assert.NoError(t, err, "close tar writer")

		err = writer.Close()
		assert.NoError(t, err, "close pipe writer")
	}()

	idx := 0
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		require.NoError(t, err, "read tar archive")
		require.Less(t, idx, len(paths), "should never get more whiteout entires than AddWhitout calls")

		// The entries should be in the same order as the original set.
		path := paths[idx]
		parsed, err := parseWhiteout(hdr.Name)
		if assert.NoErrorf(t, err, "getting whiteout for %s", path) {
			cleanPath := filepath.Clean(path)
			assert.Equalf(t, cleanPath, parsed, "whiteout entry %d is out of order", idx)
		}

		idx++
	}
	assert.Len(t, paths, idx, "all paths should have a whiteout entry generated")
}
