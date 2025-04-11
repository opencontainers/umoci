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

package unpriv

import (
	"archive/tar"
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/opencontainers/umoci/pkg/testutils"
)

func TestWrapNoTricks(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestWrapNoTricks")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Make sure that no error is returned an no trickery is done if fn() works
	// the first time. This is important to make sure that we're not doing
	// dodgy stuff if unnecessary.
	if err := Wrap(filepath.Join(dir, "nonexistant", "path"), func(path string) error {
		return nil
	}); err != nil {
		t.Errorf("wrap returned error in the simple case: %s", err)
	}

	// Now make sure that Wrap doesn't mess with any directories in the same case.
	if err := os.MkdirAll(filepath.Join(dir, "parent", "directory"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := Wrap(filepath.Join(dir, "parent", "directory"), func(path string) error {
		return nil
	}); err != nil {
		t.Errorf("wrap returned error in the simple case: %s", err)
	}
}

func TestLstat(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestLstat")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), []byte("some content"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	var fi os.FileInfo

	// Check that the mode was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestReadlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestReadlink")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("some path", filepath.Join(dir, "some", "parent", "directories", "link1")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("..", filepath.Join(dir, "some", "parent", "directories", "link2")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	var linkname string

	// Check that the links can be read.
	linkname, err = Readlink(filepath.Join(dir, "some", "parent", "directories", "link1"))
	if err != nil {
		t.Errorf("unexpected unpriv.readlink error: %s", err)
	}
	if linkname != "some path" {
		t.Errorf("unexpected linkname for path %s: %s", "link1", linkname)
	}
	linkname, err = Readlink(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err != nil {
		t.Errorf("unexpected unpriv.readlink error: %s", err)
	}
	if linkname != ".." {
		t.Errorf("unexpected linkname for path %s: %s", "link2", linkname)
	}

	var fi os.FileInfo

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link1"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestSymlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestSymlink")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// unpriv.Symlink.
	if err := Symlink("some path", filepath.Join(dir, "some", "parent", "directories", "link1")); err != nil {
		t.Fatal(err)
	}
	if err := Symlink("..", filepath.Join(dir, "some", "parent", "directories", "link2")); err != nil {
		t.Fatal(err)
	}

	var linkname string

	// Check that the links can be read.
	linkname, err = Readlink(filepath.Join(dir, "some", "parent", "directories", "link1"))
	if err != nil {
		t.Errorf("unexpected unpriv.readlink error: %s", err)
	}
	if linkname != "some path" {
		t.Errorf("unexpected linkname for path %s: %s", "link1", linkname)
	}
	linkname, err = Readlink(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err != nil {
		t.Errorf("unexpected unpriv.readlink error: %s", err)
	}
	if linkname != ".." {
		t.Errorf("unexpected linkname for path %s: %s", "link2", linkname)
	}

	var fi os.FileInfo

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link1"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestOpen")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "file"), []byte("parent"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "file"), []byte("some"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "file"), []byte("dir"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh.Close()

	var fi os.FileInfo

	// Check that the mode was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Check using fh.Stat.
	fi, err = fh.Stat()
	if err != nil {
		t.Errorf("unexpected unpriv.open.stat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Read the file contents.
	gotContent, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Errorf("unexpected error reading from unpriv.open: %s", err)
	}
	if !bytes.Equal(gotContent, fileContent) {
		t.Errorf("unpriv.open content doesn't match actual content: expected=%s got=%s", fileContent, gotContent)
	}

	// Now change the mode using fh.Chmod.
	if err := fh.Chmod(0755); err != nil {
		t.Errorf("unexpected error doing fh.chown: %s", err)
	}

	// Double check it was changed.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0755 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Change it back.
	if err := fh.Chmod(0); err != nil {
		t.Errorf("unexpected error doing fh.chown: %s", err)
	}

	// Double check it was changed.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestReaddir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestReaddir")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file1"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file2"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file3"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "some", "parent", "directories", "dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file1"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file2"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file3"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "dir"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// Make sure that the naive Open.Readdir will fail.
	fh, err := Open(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh.Close()

	_, err = fh.Readdir(-1)
	if err == nil {
		t.Errorf("unexpected unpriv.open.readdir success (unwrapped readdir)!")
	}

	// Check that Readdir() only returns the relevant results.
	infos, err := Readdir(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.readdir error: %s", err)
	}
	if len(infos) != 4 {
		t.Errorf("expected unpriv.readdir to give %d results, got %d", 4, len(infos))
	}
	for _, info := range infos {
		if info.Mode()&os.ModePerm != 0 {
			t.Errorf("unexpected modeperm for path %s: %o", info.Name(), info.Mode()&os.ModePerm)
		}
	}

	var fi os.FileInfo

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}

	// Make sure that the naive Open.Readdir will still fail.
	fh, err = Open(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh.Close()

	_, err = fh.Readdir(-1)
	if err == nil {
		t.Errorf("unexpected unpriv.open.readdir success (unwrapped readdir)!")
	}
}

func TestWrapWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestWrapWrite")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	if err := Wrap(filepath.Join(dir, "some", "parent", "directories", "lolpath"), func(path string) error {
		return ioutil.WriteFile(path, fileContent, 0755)
	}); err != nil {
		t.Errorf("unpexected unpriv.wrap writing error: %s", err)
	}

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "lolpath"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh.Close()

	// Read the file contents.
	gotContent, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Errorf("unexpected error reading from unpriv.open: %s", err)
	}
	if !bytes.Equal(gotContent, fileContent) {
		t.Errorf("unpriv.open content doesn't match actual content: expected=%s got=%s", fileContent, gotContent)
	}

	var fi os.FileInfo

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestLink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestLink")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh.Close()

	var fi os.FileInfo

	// Read the file contents.
	gotContent, err := ioutil.ReadAll(fh)
	if err != nil {
		t.Errorf("unexpected error reading from unpriv.open: %s", err)
	}
	if !bytes.Equal(gotContent, fileContent) {
		t.Errorf("unpriv.open content doesn't match actual content: expected=%s got=%s", fileContent, gotContent)
	}

	// Make new links.
	if err := Link(filepath.Join(dir, "some", "parent", "directories", "file"), filepath.Join(dir, "some", "parent", "directories", "file2")); err != nil {
		t.Errorf("unexpected unpriv.link error: %s", err)
	}
	if err := Link(filepath.Join(dir, "some", "parent", "directories", "file"), filepath.Join(dir, "some", "parent", "file2")); err != nil {
		t.Errorf("unexpected unpriv.link error: %s", err)
	}

	// Check the contents.
	fh1, err := Open(filepath.Join(dir, "some", "parent", "directories", "file2"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh1.Close()
	gotContent1, err := ioutil.ReadAll(fh1)
	if err != nil {
		t.Errorf("unexpected error reading from unpriv.open: %s", err)
	}
	if !bytes.Equal(gotContent1, fileContent) {
		t.Errorf("unpriv.open content doesn't match actual content: expected=%s got=%s", fileContent, gotContent1)
	}

	// And the other link.
	fh2, err := Open(filepath.Join(dir, "some", "parent", "file2"))
	if err != nil {
		t.Errorf("unexpected unpriv.open error: %s", err)
	}
	defer fh2.Close()
	gotContent2, err := ioutil.ReadAll(fh2)
	if err != nil {
		t.Errorf("unexpected error reading from unpriv.open: %s", err)
	}
	if !bytes.Equal(gotContent2, fileContent) {
		t.Errorf("unpriv.open content doesn't match actual content: expected=%s got=%s", fileContent, gotContent2)
	}

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi1, err := Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi2, err := Lstat(filepath.Join(dir, "some", "parent", "file2"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Check that the files are the same.
	if !os.SameFile(fi, fi1) {
		t.Errorf("link1 and original file not the same!")
	}
	if !os.SameFile(fi, fi2) {
		t.Errorf("link2 and original file not the same!")
	}
	if !os.SameFile(fi1, fi2) {
		t.Errorf("link1 and link2 not the same!")
	}

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestChtimes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestChtimes")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	var fi os.FileInfo

	// Get the atime and mtime of one of the paths.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrOld, _ := tar.FileInfoHeader(fi, "")

	// Modify the times.
	atime := testutils.Unix(12345678, 12421512)
	mtime := testutils.Unix(11245631, 13373321)
	if err := Chtimes(filepath.Join(dir, "some", "parent", "directories"), atime, mtime); err != nil {
		t.Errorf("unexpected error from unpriv.chtimes: %s", err)
	}

	// Get the new atime and mtime.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrNew, _ := tar.FileInfoHeader(fi, "")

	if hdrNew.AccessTime.Equal(hdrOld.AccessTime) {
		t.Errorf("atime was unchanged! %s", hdrNew.AccessTime)
	}
	if hdrNew.ModTime.Equal(hdrOld.ModTime) {
		t.Errorf("mtime was unchanged! %s", hdrNew.ModTime)
	}
	if !hdrNew.ModTime.Equal(mtime) {
		t.Errorf("mtime was not change to correct value. expected=%q got=%q", mtime, hdrNew.ModTime)
	}
	if !hdrNew.AccessTime.Equal(atime) {
		t.Errorf("atime was not change to correct value. expected=%q got=%q", atime, hdrNew.AccessTime)
	}

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestLutimes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestLutimes")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", filepath.Join(dir, "some", "parent", "directories", "link2")); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	var fi os.FileInfo

	// Get the atime and mtime of one of the paths.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrDirOld, _ := tar.FileInfoHeader(fi, "")

	// Modify the times.
	atime := testutils.Unix(12345678, 12421512)
	mtime := testutils.Unix(11245631, 13373321)
	if err := Lutimes(filepath.Join(dir, "some", "parent", "directories"), atime, mtime); err != nil {
		t.Errorf("unexpected error from unpriv.lutimes: %s", err)
	}

	// Get the new atime and mtime.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrDirNew, _ := tar.FileInfoHeader(fi, "")

	if hdrDirNew.AccessTime.Equal(hdrDirOld.AccessTime) {
		t.Errorf("atime was unchanged! %s", hdrDirNew.AccessTime)
	}
	if hdrDirNew.ModTime.Equal(hdrDirOld.ModTime) {
		t.Errorf("mtime was unchanged! %s", hdrDirNew.ModTime)
	}
	if !hdrDirNew.ModTime.Equal(mtime) {
		t.Errorf("mtime was not change to correct value. expected=%q got=%q", mtime, hdrDirNew.ModTime)
	}
	if !hdrDirNew.AccessTime.Equal(atime) {
		t.Errorf("atime was not change to correct value. expected=%q got=%q", atime, hdrDirNew.AccessTime)
	}

	// Do the same for a symlink.
	atime = testutils.Unix(18127518, 12421122)
	mtime = testutils.Unix(15245123, 19912991)

	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrOld, _ := tar.FileInfoHeader(fi, "")
	if err := Lutimes(filepath.Join(dir, "some", "parent", "directories", "link2"), atime, mtime); err != nil {
		t.Errorf("unexpected error from unpriv.lutimes: %s", err)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrNew, _ := tar.FileInfoHeader(fi, "")

	if hdrNew.AccessTime.Equal(hdrOld.AccessTime) {
		t.Errorf("atime was unchanged! %s", hdrNew.AccessTime)
	}
	if hdrNew.ModTime.Equal(hdrOld.ModTime) {
		t.Errorf("mtime was unchanged! %s", hdrNew.ModTime)
	}
	if !hdrNew.ModTime.Equal(mtime) {
		t.Errorf("mtime was not change to correct value. expected=%q got=%q", mtime, hdrNew.ModTime)
	}
	if !hdrNew.AccessTime.Equal(atime) {
		t.Errorf("atime was not change to correct value. expected=%q got=%q", atime, hdrNew.AccessTime)
	}

	// Make sure that the parent was not changed by Lutimes.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected error from unpriv.lstat: %s", err)
	}
	hdrDirNew2, _ := tar.FileInfoHeader(fi, "")

	if !hdrDirNew2.AccessTime.Equal(hdrDirNew.AccessTime) {
		t.Errorf("atime was changed! expected=%q got=%q", hdrDirNew.AccessTime, hdrDirNew2.AccessTime)
	}
	if !hdrDirNew2.ModTime.Equal(hdrDirNew.ModTime) {
		t.Errorf("mtime was changed! expected=%q got=%q", hdrDirNew.ModTime, hdrDirNew2.ModTime)
	}

	// Check that the parents were unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "parent", "directories"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "parent"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestRemove")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "some", "cousin", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "file2"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "cousin", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "file2"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "cousin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// Make sure that os.Remove fails.
	if err := os.Remove(filepath.Join(dir, "some", "parent", "directories", "file")); err == nil {
		t.Errorf("os.remove did not fail!")
	}

	// Now try removing all of the things.
	if err := Remove(filepath.Join(dir, "some", "parent", "directories", "file")); err != nil {
		t.Errorf("unexpected failure in unpriv.remove: %s", err)
	}
	if err := Remove(filepath.Join(dir, "some", "parent", "directories")); err != nil {
		t.Errorf("unexpected failure in unpriv.remove: %s", err)
	}
	if err := Remove(filepath.Join(dir, "some", "parent", "file2")); err != nil {
		t.Errorf("unexpected failure in unpriv.remove: %s", err)
	}
	if err := Remove(filepath.Join(dir, "some", "cousin", "directories")); err != nil {
		t.Errorf("unexpected failure in unpriv.remove: %s", err)
	}

	// Check that they are gone.
	if _, err := Lstat(filepath.Join(dir, "some", "parent", "directories")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected deleted directory to give ENOENT: %s", err)
	}
	if _, err := Lstat(filepath.Join(dir, "some", "cousin", "directories")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected deleted directory to give ENOENT: %s", err)
	}
	if _, err := Lstat(filepath.Join(dir, "some", "cousin", "directories")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected deleted file to give ENOENT: %s", err)
	}
	if _, err := Lstat(filepath.Join(dir, "some", "parent", "file2")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected deleted file to give ENOENT: %s", err)
	}
}

func TestRemoveAll(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestRemoveAll")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "cousin", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "file2"), fileContent, 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "cousin", "directories"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "cousin"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "file2"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// Make sure that os.RemoveAll fails.
	if err := os.RemoveAll(filepath.Join(dir, "some", "parent")); err == nil {
		t.Errorf("os.removeall did not fail!")
	}

	// Now try to removeall the entire tree.
	if err := RemoveAll(filepath.Join(dir, "some", "parent")); err != nil {
		t.Errorf("unexpected failure in unpriv.removeall: %s", err)
	}

	// Check that they are gone.
	if _, err := Lstat(filepath.Join(dir, "some", "parent")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("expected deleted directory to give ENOENT: %s", err)
	}
	if _, err := Lstat(filepath.Join(dir, "some")); err != nil {
		t.Errorf("expected parent of deleted directory to not have error: %s", err)
	}

	// Make sure that trying to remove the directory after it's gone still won't fail.
	if err := RemoveAll(filepath.Join(dir, "some", "parent")); err != nil {
		t.Errorf("unexpected failure in unpriv.removeall (after deletion): %s", err)
	}
}

func TestMkdir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestMkdir")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create no structure.
	if err := os.MkdirAll(filepath.Join(dir, "some"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// Make some subdirectories.
	if err := Mkdir(filepath.Join(dir, "some", "child"), 0); err != nil {
		t.Fatal(err)
	}
	if err := Mkdir(filepath.Join(dir, "some", "other-child"), 0); err != nil {
		t.Fatal(err)
	}
	if err := Mkdir(filepath.Join(dir, "some", "child", "dir"), 0); err != nil {
		t.Fatal(err)
	}

	// Check that they all have chmod(0).
	var fi os.FileInfo

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "other-child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "child", "dir"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "child"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "other-child"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "child", "dir"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestMkdirAll(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestMkdirAll")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create no structure.
	if err := os.MkdirAll(filepath.Join(dir, "some"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}

	// Make some subdirectories.
	if err := MkdirAll(filepath.Join(dir, "some", "child"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "other-child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}

	// Check that they all have chmod(0).
	var fi os.FileInfo

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "other-child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "other-child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "other-child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "other-child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}

	// Make sure that os.Lstat still fails.
	fi, err = os.Lstat(filepath.Join(dir, "some", "child"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "other-child"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
	fi, err = os.Lstat(filepath.Join(dir, "some", "child", "dir"))
	if err == nil {
		t.Errorf("expected os.Lstat to give EPERM -- got no error!")
	} else if !errors.Is(err, os.ErrPermission) {
		t.Errorf("expected os.Lstat to give EPERM -- got %s", err)
	}
}

func TestMkdirAllMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestMkdirAllMissing")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create no structure, but with read access.
	if err := os.MkdirAll(filepath.Join(dir, "some"), 0755); err != nil {
		t.Fatal(err)
	}

	// Make some subdirectories.
	if err := MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}
	// Make sure that os.MkdirAll fails.
	if err := os.MkdirAll(filepath.Join(dir, "some", "serious", "hacks"), 0); err == nil {
		t.Fatalf("expected MkdirAll to error out")
	}

	// Check that they all have chmod(0).
	var fi os.FileInfo

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
}

// Makes sure that if a parent directory only has +rw (-x) permissions, things
// are handled correctly. This is modelled after fedora's root filesystem
// (specifically /var/log/anaconda/pre-anaconda-logs/lvmdump).
func TestMkdirRWPerm(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestMkdirRWPerm")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some small structure. This is modelled after /var/log/anaconda/pre-anaconda-logs/lvmdump.
	if err := os.MkdirAll(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs"), 0600); err != nil {
		t.Fatal(err)
	}

	// Now we have to try to create /var/log/anaconda/pre-anaconda-logs/lvmdump/config_diff.
	if fh, err := os.Create(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump", "config_diff")); err == nil {
		fh.Close()
		t.Fatalf("expected error when using os.create for lvmdump/config_diff!")
	}

	// Try to do it with unpriv.
	fh, err := Create(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump", "config_diff"))
	if err != nil {
		t.Fatalf("unexpected unpriv.create error: %s", err)
	}
	defer fh.Close()

	if n, err := fh.Write(fileContent); err != nil {
		t.Fatal(err)
	} else if n != len(fileContent) {
		t.Fatalf("incomplete write to config_diff")
	}

	// Make some subdirectories.
	if err := MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}
	if err := MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"), 0); err != nil {
		t.Fatal(err)
	}
	// Make sure that os.MkdirAll fails.
	if err := os.MkdirAll(filepath.Join(dir, "some", "serious", "hacks"), 0); err == nil {
		t.Fatalf("expected MkdirAll to error out")
	}

	// Check that they all have chmod(0).
	var fi os.FileInfo

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
}

// Makes sure that if a parent directory only has +rx (-w) permissions, things
// are handled correctly with Mkdir or Create.
func TestMkdirRPerm(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestMkdirRPerm")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	fileContent := []byte("some content")

	// Create some small structure.
	if err := os.MkdirAll(filepath.Join(dir, "var", "log"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "var", "log"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "var"), 0555); err != nil {
		t.Fatal(err)
	}

	if fh, err := os.Create(filepath.Join(dir, "var", "log", "anaconda")); err == nil {
		fh.Close()
		t.Fatalf("expected error when using os.create for lvmdump/config_diff!")
	}

	// Try to do it with unpriv.
	fh, err := Create(filepath.Join(dir, "var", "log", "anaconda"))
	if err != nil {
		t.Fatalf("unexpected unpriv.create error: %s", err)
	}
	if err := fh.Chmod(0); err != nil {
		t.Fatalf("unexpected unpriv.create.chmod error: %s", err)
	}
	defer fh.Close()

	if n, err := fh.Write(fileContent); err != nil {
		t.Fatal(err)
	} else if n != len(fileContent) {
		t.Fatalf("incomplete write to config_diff")
	}

	// Make some subdirectories.
	if err := MkdirAll(filepath.Join(dir, "var", "log", "anaconda2", "childdir"), 0); err != nil {
		t.Fatal(err)
	}

	// Check that they all have chmod(0).
	var fi os.FileInfo

	// Double check it was unchanged.
	fi, err = Lstat(filepath.Join(dir, "var", "log", "anaconda"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "var", "log", "anaconda2", "childdir"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "var", "log", "anaconda2"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "var", "log"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0555 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
	fi, err = Lstat(filepath.Join(dir, "var"))
	if err != nil {
		t.Errorf("unexpected unpriv.lstat error: %s", err)
	}
	if fi.Mode()&os.ModePerm != 0555 {
		t.Errorf("unexpected modeperm for path %s: %o", fi.Name(), fi.Mode()&os.ModePerm)
	}
}

func TestWalk(t *testing.T) {
	// There are two important things to make sure of here. That we actually
	// hit all of the paths (once), and that the fileinfo we get is the one we
	// expected.

	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir, err := ioutil.TempDir("", "umoci-unpriv.TestWalk")
	if err != nil {
		t.Fatal(err)
	}
	defer RemoveAll(dir)

	// Create some structure.
	if err := os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := ioutil.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), []byte("some content"), 0555); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0123); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some", "parent"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "some"), 0); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}

	// Walk over it.
	seen := map[string]int{}
	err = Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Don't expect errors.
		if err != nil {
			t.Errorf("unexpected error in walkfunc(%s): %v", path, err)
			return err
		}

		// Run Lstat first, and return an error if it would fail so Wrap "works".
		newFi, err := os.Lstat(path)
		if err != nil {
			return err
		}

		// Figure out the expected mode.
		expectedMode := os.FileMode(0xFFFFFFFF)
		switch path {
		case dir:
			expectedMode = 0755 | os.ModeDir
		case filepath.Join(dir, "some"),
			filepath.Join(dir, "some", "parent"):
			expectedMode = os.ModeDir
		case filepath.Join(dir, "some", "parent", "directories"):
			expectedMode = 0123 | os.ModeDir
		case filepath.Join(dir, "some", "parent", "directories", "file"):
			expectedMode = 0
		default:
			t.Errorf("saw unexpected path %s", path)
			return nil
		}

		// Check the mode.
		if info.Mode() != expectedMode {
			t.Errorf("got unexpected mode on %s: expected %o got %o", path, info.Mode(), expectedMode)
		}
		if !reflect.DeepEqual(info, newFi) {
			t.Errorf("got different info after lstat: before=%#v after=%#v", info, newFi)
		}

		// Update seen map.
		seen[path]++
		return nil
	})
	if err != nil {
		t.Errorf("unexpected walk error: %v", err)
	}

	// Check the seen map.
	for path, num := range seen {
		if num != 1 {
			t.Errorf("path %s seen an unexpected number of times %d (expected %d)", path, num, 1)
		}
	}
	if len(seen) != 5 {
		t.Errorf("saw an unexpected number of paths: len(%v) != %v", seen, 5)
	}
}
