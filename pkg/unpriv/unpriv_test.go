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
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal/testhelpers"
)

func TestWrapNoTricks(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Make sure that no error is returned an no trickery is done if fn() works
	// the first time. This is important to make sure that we're not doing
	// dodgy stuff if unnecessary.
	err = Wrap(filepath.Join(dir, "nonexistant", "path"), func(path string) error { //nolint:revive // unused-parameter doesn't make sense for this test
		return nil
	})
	require.NoError(t, err, "wrap should not return error in simple case")

	// Now make sure that Wrap doesn't mess with any directories in the same case.
	err = os.MkdirAll(filepath.Join(dir, "parent", "directory"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "parent"), 0)
	require.NoError(t, err)
	err = Wrap(filepath.Join(dir, "parent", "directory"), func(path string) error { //nolint:revive // unused-parameter doesn't make sense for this test
		return nil
	})
	require.NoError(t, err, "wrap should not return error in simple case")
}

// assert that the given path has 0o000 permissions and is thus inaccessible.
func assertInaccessibleMode(t *testing.T, path string) os.FileInfo {
	fi, err := Lstat(path)
	if assert.NoErrorf(t, err, "checking %q is inaccesssible", path) {
		assert.Zero(t, fi.Mode()&os.ModePerm, "permissions on inaccessible path should be 0o000")
	}
	return fi
}

// assert that the file has the specified file mode.
func assertFileMode(t *testing.T, path string, mode os.FileMode) os.FileInfo {
	fi, err := Lstat(path)
	if assert.NoErrorf(t, err, "checking %q is accesssible", path) {
		assert.Equalf(t, mode, fi.Mode()&os.ModePerm, "incorrect permissions on %q", path)
	}
	return fi
}

// assert that the file does not exist.
func assertNotExist(t *testing.T, path string) {
	_, err := Lstat(path)
	assert.ErrorIsf(t, err, os.ErrNotExist, "expected path %q to not exist", path) //nolint:testifylint // assert.*Error* makes more sense
}

// assert that the file does exist.
func assertExist(t *testing.T, path string) {
	_, err := Lstat(path)
	require.NoErrorf(t, err, "expected path %q to exist", path)
}

func TestLstat(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), []byte("some content"), 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Check that the mode was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file"))

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file"))

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestReadlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.Symlink("some path", filepath.Join(dir, "some", "parent", "directories", "link1"))
	require.NoError(t, err)
	err = os.Symlink("..", filepath.Join(dir, "some", "parent", "directories", "link2"))
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Check that the links can be read.
	if linkname, err := Readlink(filepath.Join(dir, "some", "parent", "directories", "link1")); assert.NoError(t, err) {
		assert.Equal(t, "some path", linkname, "incorrect symlink target")
	}
	if linkname, err := Readlink(filepath.Join(dir, "some", "parent", "directories", "link2")); assert.NoError(t, err) {
		assert.Equal(t, "..", linkname, "incorrect symlink target")
	}

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link1"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestSymlink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// unpriv.Symlink.
	err = Symlink("some path", filepath.Join(dir, "some", "parent", "directories", "link1"))
	require.NoError(t, err)
	err = Symlink("..", filepath.Join(dir, "some", "parent", "directories", "link2"))
	require.NoError(t, err)

	// Check that the links can be read.
	if linkname, err := Readlink(filepath.Join(dir, "some", "parent", "directories", "link1")); assert.NoError(t, err) {
		assert.Equal(t, "some path", linkname, "incorrect symlink target")
	}
	if linkname, err := Readlink(filepath.Join(dir, "some", "parent", "directories", "link2")); assert.NoError(t, err) {
		assert.Equal(t, "..", linkname, "incorrect symlink target")
	}

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link1"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestOpen(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "file"), []byte("parent"), 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "file"), []byte("some"), 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "file"), []byte("dir"), 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "file"))
	require.NoError(t, err, "unpriv open")
	defer fh.Close() //nolint:errcheck

	// Check that the mode was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file"))

	// Check using fh.Stat.
	if fi, err := fh.Stat(); assert.NoErrorf(t, err, "checking %q is inaccesssible", fh.Name()) {
		assert.Zero(t, fi.Mode()&os.ModePerm, "permissions on inaccessible path should be 0o000")
	}

	// Read the file contents.
	gotContent, err := io.ReadAll(fh)
	require.NoError(t, err)
	assert.Equal(t, fileContent, gotContent, "unpriv open content should match actual file contents")

	// Now change the mode using fh.Chmod.
	err = fh.Chmod(0o755)
	require.NoError(t, err)

	// Double check it was changed.
	if fi, err := Lstat(filepath.Join(dir, "some", "parent", "directories", "file")); assert.NoErrorf(t, err, "checking %q is accesssible", fh.Name()) {
		assert.EqualValues(t, 0o755, fi.Mode()&os.ModePerm, "permissions on accessible path should be 0o755")
	}

	// Change it back.
	err = fh.Chmod(0)
	require.NoError(t, err)

	// Double check it was changed.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file"))

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestReaddir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file1"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file2"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file3"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Mkdir(filepath.Join(dir, "some", "parent", "directories", "dir"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file1"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file2"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file3"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "dir"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Make sure that the naive Open+Readdir will fail.
	fh, err := Open(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck

	_, err = fh.Readdir(-1)
	assert.Error(t, err, "unwrapped readdir of an unpriv open should fail") //nolint:testifylint // assert.*Error* makes more sense

	// Check that Readdir() only returns the relevant results.
	infos, err := Readdir(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	assert.Len(t, infos, 4, "unpriv readdir should give the right number of results")
	for _, info := range infos {
		assert.Zerof(t, info.Mode()&os.ModePerm, "unexpected permissions for path %q", info.Name())
	}

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense

	// Make sure that the naive Open.Readdir will still fail.
	fh, err = Open(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err, "unpriv open")
	defer fh.Close() //nolint:errcheck

	_, err = fh.Readdir(-1)
	assert.Error(t, err, "unwrapped readdir of an unpriv open should still fail") //nolint:testifylint // assert.*Error* makes more sense
}

func TestWrapWrite(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	err = Wrap(filepath.Join(dir, "some", "parent", "directories", "lolpath"), func(path string) error {
		return os.WriteFile(path, fileContent, 0o755)
	})
	require.NoError(t, err, "unwrap wrap WriteFile")

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "lolpath"))
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck

	// Read the file contents.
	if gotContent, err := io.ReadAll(fh); assert.NoError(t, err) {
		assert.Equal(t, fileContent, gotContent, "file content should match original content")
	}

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestLink(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	fh, err := Open(filepath.Join(dir, "some", "parent", "directories", "file"))
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck

	// Read the file contents.
	if gotContent, err := io.ReadAll(fh); assert.NoError(t, err) {
		assert.Equal(t, fileContent, gotContent, "file content should match original content")
	}

	// Make new links.
	err = Link(filepath.Join(dir, "some", "parent", "directories", "file"), filepath.Join(dir, "some", "parent", "directories", "file2"))
	require.NoError(t, err)
	err = Link(filepath.Join(dir, "some", "parent", "directories", "file"), filepath.Join(dir, "some", "parent", "file2"))
	require.NoError(t, err)

	// Check the contents.
	fh1, err := Open(filepath.Join(dir, "some", "parent", "directories", "file2"))
	require.NoError(t, err)
	defer fh1.Close() //nolint:errcheck
	if gotContent, err := io.ReadAll(fh1); assert.NoError(t, err) {
		assert.Equal(t, fileContent, gotContent, "file content through link1 should match original content")
	}

	// And the other link.
	fh2, err := Open(filepath.Join(dir, "some", "parent", "file2"))
	require.NoError(t, err)
	defer fh2.Close() //nolint:errcheck
	if gotContent, err := io.ReadAll(fh2); assert.NoError(t, err) {
		assert.Equal(t, fileContent, gotContent, "file content through link2 should match original content")
	}

	// Double check it was unchanged.
	fi1 := assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file"))
	fi2 := assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories", "file2"))
	fi3 := assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "file2"))

	// Check that the files are the same.
	assert.True(t, os.SameFile(fi1, fi2), "link1 and original file should be the same")
	assert.True(t, os.SameFile(fi1, fi3), "link2 and original file should be the same")
	assert.True(t, os.SameFile(fi2, fi3), "link1 and link2 should be the same")

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestChtimes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Get the atime and mtime of one of the paths.
	fiOld, err := Lstat(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	hdrOld, _ := tar.FileInfoHeader(fiOld, "")

	// Modify the times.
	atime := testhelpers.Unix(12345678, 12421512)
	mtime := testhelpers.Unix(11245631, 13373321)
	err = Chtimes(filepath.Join(dir, "some", "parent", "directories"), atime, mtime)
	require.NoError(t, err)

	// Get the new atime and mtime.
	fiNew, err := Lstat(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	hdrNew, _ := tar.FileInfoHeader(fiNew, "")

	assert.NotEqual(t, hdrOld.AccessTime, hdrNew.AccessTime, "atime should have changed after chtimes")
	assert.Equal(t, atime, hdrNew.AccessTime, "atime should match value given to chtimes")
	assert.NotEqual(t, hdrOld.ModTime, hdrNew.ModTime, "mtime should have changed after chtimes")
	assert.Equal(t, mtime, hdrNew.ModTime, "mtime should match value given to chtimes")

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestLutimes(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Symlink(".", filepath.Join(dir, "some", "parent", "directories", "link2"))
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Get the atime and mtime of one of the paths.
	fiDirOld, err := Lstat(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	hdrDirOld, _ := tar.FileInfoHeader(fiDirOld, "")

	// Modify the times.
	atime := testhelpers.Unix(12345678, 12421512)
	mtime := testhelpers.Unix(11245631, 13373321)
	err = Lutimes(filepath.Join(dir, "some", "parent", "directories"), atime, mtime)
	require.NoError(t, err)

	// Get the new atime and mtime.
	fiDirNew, err := Lstat(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	hdrDirNew, _ := tar.FileInfoHeader(fiDirNew, "")

	assert.NotEqual(t, hdrDirOld.AccessTime, hdrDirNew.AccessTime, "atime should have changed after lutimes")
	assert.Equal(t, atime, hdrDirNew.AccessTime, "atime should match value given to lutimes")
	assert.NotEqual(t, hdrDirOld.ModTime, hdrDirNew.ModTime, "mtime should have changed after lutimes")
	assert.Equal(t, mtime, hdrDirNew.ModTime, "mtime should match value given to lutimes")

	// Do the same for a symlink.
	atime = testhelpers.Unix(18127518, 12421122)
	mtime = testhelpers.Unix(15245123, 19912991)

	fiOld, err := Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	require.NoError(t, err)
	hdrOld, _ := tar.FileInfoHeader(fiOld, "")

	err = Lutimes(filepath.Join(dir, "some", "parent", "directories", "link2"), atime, mtime)
	require.NoError(t, err)

	fiNew, err := Lstat(filepath.Join(dir, "some", "parent", "directories", "link2"))
	require.NoError(t, err)
	hdrNew, _ := tar.FileInfoHeader(fiNew, "")

	assert.NotEqual(t, hdrOld.AccessTime, hdrNew.AccessTime, "atime should have changed after lutimes")
	assert.Equal(t, atime, hdrNew.AccessTime, "atime should match value given to lutimes")
	assert.NotEqual(t, hdrOld.ModTime, hdrNew.ModTime, "mtime should have changed after lutimes")
	assert.Equal(t, mtime, hdrNew.ModTime, "mtime should match value given to lutimes")

	// Make sure that the parent was not changed by Lutimes.
	fiDirNew2, err := Lstat(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	hdrDirNew2, _ := tar.FileInfoHeader(fiDirNew2, "")

	assert.EqualExportedValues(t, hdrDirNew, hdrDirNew2, "parent directory state should not have been changed by wrapped Lutimes")

	// Check that the parents were unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent", "directories"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "parent"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "directories", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "parent", "file2"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestRemove(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(dir, "some", "cousin", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "file2"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "cousin", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "file2"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "cousin"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Make sure that os.Remove fails.
	err = os.Remove(filepath.Join(dir, "some", "parent", "directories", "file"))
	require.ErrorIs(t, err, os.ErrPermission, "os remove should fail with EACCES")

	// Now try removing all of the things.
	err = Remove(filepath.Join(dir, "some", "parent", "directories", "file"))
	require.NoError(t, err)
	err = Remove(filepath.Join(dir, "some", "parent", "directories"))
	require.NoError(t, err)
	err = Remove(filepath.Join(dir, "some", "parent", "file2"))
	require.NoError(t, err)
	err = Remove(filepath.Join(dir, "some", "cousin", "directories"))
	require.NoError(t, err)

	// Check that they are gone.
	assertNotExist(t, filepath.Join(dir, "some", "parent", "directories", "file"))
	assertNotExist(t, filepath.Join(dir, "some", "parent", "directories"))
	assertNotExist(t, filepath.Join(dir, "some", "cousin", "directories"))
	assertNotExist(t, filepath.Join(dir, "some", "parent", "file2"))
}

func TestRemoveAll(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "cousin", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "file2"), fileContent, 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "cousin", "directories"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "cousin"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "file2"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Make sure that os.RemoveAll fails.
	err = os.RemoveAll(filepath.Join(dir, "some", "parent"))
	require.ErrorIs(t, err, os.ErrPermission, "os removeall should fail")

	// Now try to removeall the entire tree.
	err = RemoveAll(filepath.Join(dir, "some", "parent"))
	require.NoError(t, err)

	// Check that they are gone.
	assertNotExist(t, filepath.Join(dir, "some", "parent"))
	assertExist(t, filepath.Join(dir, "some"))

	// Make sure that trying to remove the directory after it's gone still won't fail.
	err = RemoveAll(filepath.Join(dir, "some", "parent"))
	require.NoError(t, err, "removeall after path is gone should still succeed")
}

func TestMkdir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create no structure.
	err = os.MkdirAll(filepath.Join(dir, "some"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Make some subdirectories.
	err = Mkdir(filepath.Join(dir, "some", "child"), 0)
	require.NoError(t, err)
	err = Mkdir(filepath.Join(dir, "some", "other-child"), 0)
	require.NoError(t, err)
	err = Mkdir(filepath.Join(dir, "some", "child", "dir"), 0)
	require.NoError(t, err)

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "other-child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child", "dir"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "child"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "other-child"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "child", "dir"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestMkdirAll(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create no structure.
	err = os.MkdirAll(filepath.Join(dir, "some"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)

	// Make some subdirectories.
	err = MkdirAll(filepath.Join(dir, "some", "child"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "other-child", "with", "more", "children"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "child", "with", "more", "children"), 0)
	require.NoError(t, err)

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "child", "with", "more", "children"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "other-child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "other-child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "other-child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "other-child", "with", "more", "children"))
	assertInaccessibleMode(t, filepath.Join(dir, "some"))

	// Make sure that os.Lstat still fails.
	_, err = os.Lstat(filepath.Join(dir, "some", "child"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "other-child"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
	_, err = os.Lstat(filepath.Join(dir, "some", "child", "dir"))
	assert.ErrorIs(t, err, os.ErrPermission, "os lstat should fail with EACCES") //nolint:testifylint // assert.*Error* makes more sense
}

func TestMkdirAllMissing(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create no structure, but with read access.
	err = os.MkdirAll(filepath.Join(dir, "some"), 0o755)
	require.NoError(t, err)

	// Make some subdirectories.
	err = MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"), 0)
	require.NoError(t, err)
	// Make sure that os.MkdirAll fails.
	err = os.MkdirAll(filepath.Join(dir, "some", "serious", "hacks"), 0)
	require.ErrorIs(t, err, os.ErrPermission, "os.MkdirAll should fail in inaccessible path")

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"))
}

// Makes sure that if a parent directory only has +rw (-x) permissions, things
// are handled correctly. This is modelled after fedora's root filesystem
// (specifically /var/log/anaconda/pre-anaconda-logs/lvmdump).
func TestMkdirRWPerm(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some small structure. This is modelled after /var/log/anaconda/pre-anaconda-logs/lvmdump.
	err = os.MkdirAll(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs"), 0o600)
	require.NoError(t, err)

	// Make sure the os.Create fails with the path.
	_, err = os.Create(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump", "config_diff"))
	require.ErrorIs(t, err, os.ErrPermission, "os.Create should fail to create in an 0600 directory")

	// Try to do it with unpriv.
	fh, err := Create(filepath.Join(dir, "var", "log", "anaconda", "pre-anaconda-logs", "lvmdump", "config_diff"))
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck

	n, err := fh.Write(fileContent)
	require.NoError(t, err)
	require.Equal(t, len(fileContent), n, "incomplete write")

	// Make some subdirectories.
	err = MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"), 0)
	require.NoError(t, err)
	err = MkdirAll(filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"), 0)
	require.NoError(t, err)
	// Make sure that os.MkdirAll fails.
	err = os.MkdirAll(filepath.Join(dir, "some", "serious", "hacks"), 0)
	require.ErrorIs(t, err, os.ErrPermission, "os.MkdirAll should fail in inaccessible path")

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "a", "b", "c", "child", "with", "more", "children"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more"))
	assertInaccessibleMode(t, filepath.Join(dir, "some", "x", "y", "z", "other-child", "with", "more", "children"))
}

// Makes sure that if a parent directory only has +rx (-w) permissions, things
// are handled correctly with Mkdir or Create.
func TestMkdirRPerm(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	fileContent := []byte("some content")

	// Create some small structure.
	err = os.MkdirAll(filepath.Join(dir, "var", "log"), 0o755)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "var", "log"), 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "var"), 0o555)
	require.NoError(t, err)

	// Make sure the os.Create fails with the path.
	_, err = os.Create(filepath.Join(dir, "var", "log", "anaconda"))
	require.ErrorIs(t, err, os.ErrPermission, "os.Create should fail to create in an 0555 directory")

	// Try to do it with unpriv.
	fh, err := Create(filepath.Join(dir, "var", "log", "anaconda"))
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck
	err = fh.Chmod(0)
	require.NoError(t, err)

	n, err := fh.Write(fileContent)
	require.NoError(t, err)
	require.Equal(t, len(fileContent), n, "incomplete write")

	// Make some subdirectories.
	err = MkdirAll(filepath.Join(dir, "var", "log", "anaconda2", "childdir"), 0)
	require.NoError(t, err)

	// Double check it was unchanged.
	assertInaccessibleMode(t, filepath.Join(dir, "var", "log", "anaconda"))
	assertInaccessibleMode(t, filepath.Join(dir, "var", "log", "anaconda2", "childdir"))
	assertInaccessibleMode(t, filepath.Join(dir, "var", "log", "anaconda2"))
	assertFileMode(t, filepath.Join(dir, "var", "log"), 0o555)
	assertFileMode(t, filepath.Join(dir, "var"), 0o555)
}

func TestWalk(t *testing.T) {
	// There are two important things to make sure of here. That we actually
	// hit all of the paths (once), and that the fileinfo we get is the one we
	// expected.

	if os.Geteuid() == 0 {
		t.Skip("unpriv.* tests only work with non-root privileges")
	}

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	// Create some structure.
	err = os.MkdirAll(filepath.Join(dir, "some", "parent", "directories"), 0o755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "some", "parent", "directories", "file"), []byte("some content"), 0o555)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories", "file"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent", "directories"), 0o123)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some", "parent"), 0)
	require.NoError(t, err)
	err = os.Chmod(filepath.Join(dir, "some"), 0)
	require.NoError(t, err)
	err = os.Chmod(dir, 0o755)
	require.NoError(t, err)

	// Walk over it.
	seen := map[string]struct{}{}
	err = Walk(dir, func(path string, info os.FileInfo, err error) error {
		// Don't expect errors.
		if !assert.NoErrorf(t, err, "unexpected error in walkfunc(%q)", path) { //nolint:testifylint
			return err
		}

		// Run Lstat first, and return an error if it would fail so Wrap "works".
		newFi, err := os.Lstat(path)
		if err != nil {
			return err
		}

		// Figure out the expected mode.
		var expectedMode os.FileMode
		switch path {
		case dir:
			expectedMode = 0o755 | os.ModeDir
		case filepath.Join(dir, "some"),
			filepath.Join(dir, "some", "parent"):
			expectedMode = os.ModeDir
		case filepath.Join(dir, "some", "parent", "directories"):
			expectedMode = 0o123 | os.ModeDir
		case filepath.Join(dir, "some", "parent", "directories", "file"):
			expectedMode = 0
		default:
			t.Errorf("saw unexpected path %s", path)
			return nil
		}

		// Check the mode.
		assert.Equalf(t, expectedMode, info.Mode(), "unexpected file mode for %q", path)
		assert.EqualExportedValues(t, info, newFi, "should get the same FileInfo before and after lstat")

		// Update seen map.
		assert.NotContainsf(t, seen, path, "saw the path %q during the walk more than once", path)
		seen[path] = struct{}{}
		return nil
	})
	require.NoError(t, err)

	// Make sure we saw the right number of elements.
	assert.Len(t, seen, 5, "expected to see all subpaths during walk")
}
