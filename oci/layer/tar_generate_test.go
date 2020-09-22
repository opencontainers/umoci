/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestTarGenerateAddFileNormal(t *testing.T) {
	reader, writer := io.Pipe()

	dir, err := ioutil.TempDir("", "umoci-TestTarGenerateAddFileNormal")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := "file"
	data := []byte("this is a normal file")
	path := filepath.Join(dir, file)

	expectedHdr := &tar.Header{
		Name:       file,
		Mode:       0644,
		ModTime:    time.Unix(123, 0),
		AccessTime: time.Unix(888, 0),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Typeflag:   tar.TypeReg,
		Size:       int64(len(data)),
	}

	te := NewTarExtractor(UnpackOptions{})
	if err := ioutil.WriteFile(path, data, 0777); err != nil {
		t.Fatalf("unexpected error creating file to add: %s", err)
	}
	if err := te.applyMetadata(path, expectedHdr); err != nil {
		t.Fatalf("apply metadata: %s", err)
	}

	tg := newTarGenerator(writer, MapOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		if err := tg.AddFile(file, path); err != nil {
			t.Errorf("AddFile: %s: unexpected error: %s", path, err)
		}
		if err := tg.tw.Close(); err != nil {
			t.Errorf("tw.Close: unexpected error: %s", err)
		}
		if err := writer.Close(); err != nil {
			t.Errorf("writer.Close: unexpected error: %s", err)
		}
	}()

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading tar archive: %s", err)
	}

	if hdr.Typeflag != expectedHdr.Typeflag {
		t.Errorf("hdr.Typeflag changed: expected %d, got %d", expectedHdr.Typeflag, hdr.Typeflag)
	}
	if hdr.Name != expectedHdr.Name {
		t.Errorf("hdr.Name changed: expected %s, got %s", expectedHdr.Name, hdr.Name)
	}
	if hdr.Uid != expectedHdr.Uid {
		t.Errorf("hdr.Uid changed: expected %d, got %d", expectedHdr.Uid, hdr.Uid)
	}
	if hdr.Gid != expectedHdr.Gid {
		t.Errorf("hdr.Gid changed: expected %d, got %d", expectedHdr.Gid, hdr.Gid)
	}
	if hdr.Size != expectedHdr.Size {
		t.Errorf("hdr.Size changed: expected %d, got %d", expectedHdr.Size, hdr.Size)
	}
	if !hdr.ModTime.Equal(expectedHdr.ModTime) {
		t.Errorf("hdr.ModTime changed: expected %s, got %s", expectedHdr.ModTime, hdr.ModTime)
	}
	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if !hdr.AccessTime.Equal(expectedHdr.AccessTime) {
		if hdr.AccessTime.IsZero() {
			t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
		} else {
			t.Errorf("hdr.AccessTime changed: expected %s, got %s", expectedHdr.AccessTime, hdr.AccessTime)
		}
	}

	gotBytes, err := ioutil.ReadAll(tr)
	if err != nil {
		t.Errorf("read all: unexpected error: %s", err)
	}
	if !bytes.Equal(gotBytes, data) {
		t.Errorf("unexpected data read from tar.Reader: expected %v, got %v", data, gotBytes)
	}

	if _, err := tr.Next(); err != io.EOF {
		t.Errorf("expected only one entry, err=%s", err)
	}
}

func TestTarGenerateAddFileDirectory(t *testing.T) {
	reader, writer := io.Pipe()

	dir, err := ioutil.TempDir("", "umoci-TestTarGenerateAddFileDirectory")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	file := "directory/"
	path := filepath.Join(dir, file)

	expectedHdr := &tar.Header{
		Name:       file,
		Mode:       0644,
		ModTime:    time.Unix(123, 0),
		AccessTime: time.Unix(888, 0),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Typeflag:   tar.TypeDir,
		Size:       0,
	}

	te := NewTarExtractor(UnpackOptions{})
	if err := os.Mkdir(path, 0777); err != nil {
		t.Fatalf("unexpected error creating file to add: %s", err)
	}
	if err := te.applyMetadata(path, expectedHdr); err != nil {
		t.Fatalf("apply metadata: %s", err)
	}

	tg := newTarGenerator(writer, MapOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		if err := tg.AddFile(file, path); err != nil {
			t.Errorf("AddFile: %s: unexpected error: %s", path, err)
		}
		if err := tg.tw.Close(); err != nil {
			t.Errorf("tw.Close: unexpected error: %s", err)
		}
		if err := writer.Close(); err != nil {
			t.Errorf("writer.Close: unexpected error: %s", err)
		}
	}()

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading tar archive: %s", err)
	}

	if hdr.Typeflag != expectedHdr.Typeflag {
		t.Errorf("hdr.Typeflag changed: expected %d, got %d", expectedHdr.Typeflag, hdr.Typeflag)
	}
	if hdr.Name != expectedHdr.Name {
		t.Errorf("hdr.Name changed: expected %s, got %s", expectedHdr.Name, hdr.Name)
	}
	if hdr.Uid != expectedHdr.Uid {
		t.Errorf("hdr.Uid changed: expected %d, got %d", expectedHdr.Uid, hdr.Uid)
	}
	if hdr.Gid != expectedHdr.Gid {
		t.Errorf("hdr.Gid changed: expected %d, got %d", expectedHdr.Gid, hdr.Gid)
	}
	if hdr.Size != expectedHdr.Size {
		t.Errorf("hdr.Size changed: expected %d, got %d", expectedHdr.Size, hdr.Size)
	}
	if !hdr.ModTime.Equal(expectedHdr.ModTime) {
		t.Errorf("hdr.ModTime changed: expected %s, got %s", expectedHdr.ModTime, hdr.ModTime)
	}
	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if !hdr.AccessTime.Equal(expectedHdr.AccessTime) {
		if hdr.AccessTime.IsZero() {
			t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
		} else {
			t.Errorf("hdr.AccessTime changed: expected %s, got %s", expectedHdr.AccessTime, hdr.AccessTime)
		}
	}

	if _, err := tr.Next(); err != io.EOF {
		t.Errorf("expected only one entry, err=%s", err)
	}
}

func TestTarGenerateAddFileSymlink(t *testing.T) {
	reader, writer := io.Pipe()

	dir, err := ioutil.TempDir("", "umoci-TestTarGenerateAddFileSymlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

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
	if err := os.Symlink(linkname, path); err != nil {
		t.Fatalf("unexpected error creating file to add: %s", err)
	}
	if err := te.applyMetadata(path, expectedHdr); err != nil {
		t.Fatalf("apply metadata: %s", err)
	}

	tg := newTarGenerator(writer, MapOptions{})
	tr := tar.NewReader(reader)

	// Create all of the tar entries in a goroutine so we can parse the tar
	// entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		if err := tg.AddFile(file, path); err != nil {
			t.Errorf("AddFile: %s: unexpected error: %s", path, err)
		}
		if err := tg.tw.Close(); err != nil {
			t.Errorf("tw.Close: unexpected error: %s", err)
		}
		if err := writer.Close(); err != nil {
			t.Errorf("writer.Close: unexpected error: %s", err)
		}
	}()

	hdr, err := tr.Next()
	if err != nil {
		t.Fatalf("reading tar archive: %s", err)
	}

	if hdr.Typeflag != expectedHdr.Typeflag {
		t.Errorf("hdr.Typeflag changed: expected %d, got %d", expectedHdr.Typeflag, hdr.Typeflag)
	}
	if hdr.Name != expectedHdr.Name {
		t.Errorf("hdr.Name changed: expected %s, got %s", expectedHdr.Name, hdr.Name)
	}
	if hdr.Linkname != expectedHdr.Linkname {
		t.Errorf("hdr.Name changed: expected %s, got %s", expectedHdr.Name, hdr.Name)
	}
	if hdr.Uid != expectedHdr.Uid {
		t.Errorf("hdr.Uid changed: expected %d, got %d", expectedHdr.Uid, hdr.Uid)
	}
	if hdr.Gid != expectedHdr.Gid {
		t.Errorf("hdr.Gid changed: expected %d, got %d", expectedHdr.Gid, hdr.Gid)
	}
	if hdr.Size != expectedHdr.Size {
		t.Errorf("hdr.Size changed: expected %d, got %d", expectedHdr.Size, hdr.Size)
	}
	if !hdr.ModTime.Equal(expectedHdr.ModTime) {
		t.Errorf("hdr.ModTime changed: expected %s, got %s", expectedHdr.ModTime, hdr.ModTime)
	}
	// This test will always fail because of a Golang bug: https://github.com/golang/go/issues/17876.
	// We will skip this test for now.
	if !hdr.AccessTime.Equal(expectedHdr.AccessTime) {
		if hdr.AccessTime.IsZero() {
			t.Logf("hdr.AccessTime doesn't match (it is zero) -- this is a Golang bug")
		} else {
			t.Errorf("hdr.AccessTime changed: expected %s, got %s", expectedHdr.AccessTime, hdr.AccessTime)
		}
	}

	if _, err := tr.Next(); err != io.EOF {
		t.Errorf("expected only one entry, err=%s", err)
	}
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

	dir, err := ioutil.TempDir("", "umoci-TestTarGenerateAddWhiteout")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Paths we want to generate whiteouts for.
	paths := []string{
		"root",
		"dir/file",
		"dir/",
		"dir/.",
	}

	tg := newTarGenerator(writer, MapOptions{})
	tr := tar.NewReader(reader)

	// Create all of the whiteout entries in a goroutine so we can parse the
	// tar entries as they're generated (io.Pipe pipes are unbuffered).
	go func() {
		for _, path := range paths {
			if err := tg.AddWhiteout(path); err != nil {
				t.Errorf("AddWhitout: %s: unexpected error: %s", path, err)
			}
		}
		if err := tg.tw.Close(); err != nil {
			t.Errorf("tw.Close: unexpected error: %s", err)
		}
		if err := writer.Close(); err != nil {
			t.Errorf("writer.Close: unexpected error: %s", err)
		}
	}()

	idx := 0
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("reading tar archive: %s", err)
		}

		if idx >= len(paths) {
			t.Fatal("got more whiteout entries than AddWhiteout calls!")
		}

		parsed, err := parseWhiteout(hdr.Name)
		if err != nil {
			t.Errorf("getting whiteout for %s: %s", paths[idx], err)
		}

		cleanPath := filepath.Clean(paths[idx])
		if parsed != cleanPath {
			t.Errorf("whiteout entry %d is out of order: expected %s, got %s", idx, cleanPath, parsed)
		}

		idx++
	}

	if idx != len(paths) {
		t.Errorf("not all paths had a whiteout entry generated (only read %d, expected %d)!", idx, len(paths))
	}
}
