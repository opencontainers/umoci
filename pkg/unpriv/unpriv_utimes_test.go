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
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/testhelpers"
)

func TestLutimesFile(t *testing.T) {
	var fiOld, fiNew unix.Stat_t

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	path := filepath.Join(dir, "some file")

	err = os.WriteFile(path, []byte("some contents"), 0o755)
	require.NoError(t, err)

	atime := testhelpers.Unix(125812851, 128518257)
	mtime := testhelpers.Unix(257172893, 995216512)

	err = unix.Lstat(path, &fiOld)
	require.NoError(t, err)

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	err = unix.Lstat(path, &fiNew)
	require.NoError(t, err)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected=%q got=%q old=%q", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected=%q got=%q old=%q", mtime, mtimeNew, mtimeOld)
	}
}

func TestLutimesDirectory(t *testing.T) {
	var fiOld, fiNew unix.Stat_t

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	path := filepath.Join(dir, " a directory  ")

	err = os.Mkdir(path, 0o755)
	require.NoError(t, err)

	atime := testhelpers.Unix(128551231, 273285257)
	mtime := testhelpers.Unix(185726393, 752135712)

	err = unix.Lstat(path, &fiOld)
	require.NoError(t, err)

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	err = unix.Lstat(path, &fiNew)
	require.NoError(t, err)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected=%q got=%q old=%q", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected=%q got=%q old=%q", mtime, mtimeNew, mtimeOld)
	}
}

func TestLutimesSymlink(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck

	path := filepath.Join(dir, " !! symlink here")

	err = os.Symlink(".", path)
	require.NoError(t, err)

	atime := testhelpers.Unix(128551231, 273285257)
	mtime := testhelpers.Unix(185726393, 752135712)

	err = unix.Lstat(path, &fiOld)
	require.NoError(t, err)
	err = unix.Lstat(dir, &fiParentOld)
	require.NoError(t, err)

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	err = unix.Lstat(path, &fiNew)
	require.NoError(t, err)
	err = unix.Lstat(dir, &fiParentNew)
	require.NoError(t, err)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected=%q got=%q old=%q", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected=%q got=%q old=%q", mtime, mtimeNew, mtimeOld)
	}

	// Make sure that the parent directory was unchanged.
	atimeParentOld := time.Unix(fiParentOld.Atim.Unix())
	mtimeParentOld := time.Unix(fiParentOld.Mtim.Unix())
	atimeParentNew := time.Unix(fiParentNew.Atim.Unix())
	mtimeParentNew := time.Unix(fiParentNew.Mtim.Unix())

	if !atimeParentOld.Equal(atimeParentNew) {
		t.Errorf("parent directory atime was changed! old=%q new=%q", atimeParentOld, atimeParentNew)
	}
	if !mtimeParentOld.Equal(mtimeParentNew) {
		t.Errorf("parent directory mtime was changed! old=%q new=%q", mtimeParentOld, mtimeParentNew)
	}
}

func TestLutimesRelative(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := os.MkdirTemp(dir, "inner") //nolint:usetesting // this tempdir is inside t.TempDir and needs special RemoveAll handling
	require.NoError(t, err)
	defer RemoveAll(dir) //nolint:errcheck
	t.Chdir(dir)

	path := filepath.Join("some parent", " !! symlink here")

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = os.Symlink(".", path)
	require.NoError(t, err)

	atime := testhelpers.Unix(134858232, 258921237)
	mtime := testhelpers.Unix(171257291, 425815288)

	err = unix.Lstat(path, &fiOld)
	require.NoError(t, err)
	err = unix.Lstat(".", &fiParentOld)
	require.NoError(t, err)

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	err = unix.Lstat(path, &fiNew)
	require.NoError(t, err)
	err = unix.Lstat(".", &fiParentNew)
	require.NoError(t, err)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected=%q got=%q old=%q", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected=%q got=%q old=%q", mtime, mtimeNew, mtimeOld)
	}

	// Make sure that the parent directory was unchanged.
	atimeParentOld := time.Unix(fiParentOld.Atim.Unix())
	mtimeParentOld := time.Unix(fiParentOld.Mtim.Unix())
	atimeParentNew := time.Unix(fiParentNew.Atim.Unix())
	mtimeParentNew := time.Unix(fiParentNew.Mtim.Unix())

	if !atimeParentOld.Equal(atimeParentNew) {
		t.Errorf("parent directory atime was changed! old=%q new=%q", atimeParentOld, atimeParentNew)
	}
	if !mtimeParentOld.Equal(mtimeParentNew) {
		t.Errorf("parent directory mtime was changed! old=%q new=%q", mtimeParentOld, mtimeParentNew)
	}
}
