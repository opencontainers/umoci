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
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/testutils"
)

func TestLutimesFile(t *testing.T) {
	var fiOld, fiNew unix.Stat_t

	dir := t.TempDir()

	// We need to delete the directory manually because the stdlib RemoveAll
	// will get permission errors with the way we structure the paths.
	dir, err := ioutil.TempDir(dir, "inner")
	require.NoError(t, err)
	defer RemoveAll(dir)

	path := filepath.Join(dir, "some file")

	if err := ioutil.WriteFile(path, []byte("some contents"), 0755); err != nil {
		t.Fatal(err)
	}

	atime := testutils.Unix(125812851, 128518257)
	mtime := testutils.Unix(257172893, 995216512)

	if err := unix.Lstat(path, &fiOld); err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	if err := unix.Lstat(path, &fiNew); err != nil {
		t.Fatal(err)
	}

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
	dir, err := ioutil.TempDir(dir, "inner")
	require.NoError(t, err)
	defer RemoveAll(dir)

	path := filepath.Join(dir, " a directory  ")

	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	atime := testutils.Unix(128551231, 273285257)
	mtime := testutils.Unix(185726393, 752135712)

	if err := unix.Lstat(path, &fiOld); err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	if err := unix.Lstat(path, &fiNew); err != nil {
		t.Fatal(err)
	}

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
	dir, err := ioutil.TempDir(dir, "inner")
	require.NoError(t, err)
	defer RemoveAll(dir)

	path := filepath.Join(dir, " !! symlink here")

	if err := os.Symlink(".", path); err != nil {
		t.Fatal(err)
	}

	atime := testutils.Unix(128551231, 273285257)
	mtime := testutils.Unix(185726393, 752135712)

	if err := unix.Lstat(path, &fiOld); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lstat(dir, &fiParentOld); err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	if err := unix.Lstat(path, &fiNew); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lstat(dir, &fiParentNew); err != nil {
		t.Fatal(err)
	}

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
	dir, err := ioutil.TempDir(dir, "inner")
	require.NoError(t, err)
	defer RemoveAll(dir)

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	os.Chdir(dir)
	defer os.Chdir(oldwd)

	path := filepath.Join("some parent", " !! symlink here")

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".", path); err != nil {
		t.Fatal(err)
	}

	atime := testutils.Unix(134858232, 258921237)
	mtime := testutils.Unix(171257291, 425815288)

	if err := unix.Lstat(path, &fiOld); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lstat(".", &fiParentOld); err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	if err := unix.Lstat(path, &fiNew); err != nil {
		t.Fatal(err)
	}
	if err := unix.Lstat(".", &fiParentNew); err != nil {
		t.Fatal(err)
	}

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
