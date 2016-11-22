/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016 SUSE LLC.
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

package system

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestLutimesFile(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-system.TestLutimesFile")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, "some file")

	if err := ioutil.WriteFile(path, []byte("some contents"), 0755); err != nil {
		t.Fatal(err)
	}

	atime := time.Unix(125812851, 128518257)
	mtime := time.Unix(257172893, 995216512)

	fiOld, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	fiNew, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	atimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Mtim.Unix())
	atimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected='%s' got='%s' old='%s'", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected='%s' got='%s' old='%s'", mtime, mtimeNew, mtimeOld)
	}
}

func TestLutimesDirectory(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-system.TestLutimesDirectory")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, " a directory  ")

	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal(err)
	}

	atime := time.Unix(128551231, 273285257)
	mtime := time.Unix(185726393, 752135712)

	fiOld, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	fiNew, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}

	atimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Mtim.Unix())
	atimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected='%s' got='%s' old='%s'", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected='%s' got='%s' old='%s'", mtime, mtimeNew, mtimeOld)
	}
}

func TestLutimesSymlink(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-system.TestLutimesSymlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	path := filepath.Join(dir, " !! symlink here")

	if err := os.Symlink(".", path); err != nil {
		t.Fatal(err)
	}

	atime := time.Unix(128551231, 273285257)
	mtime := time.Unix(185726393, 752135712)

	fiOld, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	fiParentOld, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}

	if err := Lutimes(path, atime, mtime); err != nil {
		t.Errorf("unexpected error with system.lutimes: %s", err)
	}

	fiNew, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	fiParentNew, err := os.Lstat(dir)
	if err != nil {
		t.Fatal(err)
	}

	atimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeOld := time.Unix(fiOld.Sys().(*syscall.Stat_t).Mtim.Unix())
	atimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeNew := time.Unix(fiNew.Sys().(*syscall.Stat_t).Mtim.Unix())

	if atimeOld.Equal(atimeNew) {
		t.Errorf("atime was not changed at all!")
	}
	if !atimeNew.Equal(atime) {
		t.Errorf("atime was not changed to expected value: expected='%s' got='%s' old='%s'", atime, atimeNew, atimeOld)
	}
	if mtimeOld.Equal(mtimeNew) {
		t.Errorf("mtime was not changed at all!")
	}
	if !mtimeNew.Equal(mtime) {
		t.Errorf("mtime was not changed: expected='%s' got='%s' old='%s'", mtime, mtimeNew, mtimeOld)
	}

	// Make sure that the parent directory was unchanged.
	atimeParentOld := time.Unix(fiParentOld.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeParentOld := time.Unix(fiParentOld.Sys().(*syscall.Stat_t).Mtim.Unix())
	atimeParentNew := time.Unix(fiParentNew.Sys().(*syscall.Stat_t).Atim.Unix())
	mtimeParentNew := time.Unix(fiParentNew.Sys().(*syscall.Stat_t).Mtim.Unix())

	if !atimeParentOld.Equal(atimeParentNew) {
		t.Errorf("parent directory atime was changed! old='%s' new='%s'", atimeParentOld, atimeParentNew)
	}
	if !mtimeParentOld.Equal(mtimeParentNew) {
		t.Errorf("parent directory mtime was changed! old='%s' new='%s'", mtimeParentOld, mtimeParentNew)
	}
}
