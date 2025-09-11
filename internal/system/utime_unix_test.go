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

package system

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/testhelpers"
)

func TestLutimesFile(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()

	path := filepath.Join(dir, "some file")

	err := os.WriteFile(path, []byte("some contents"), 0o755)
	require.NoError(t, err)

	atime := testhelpers.Unix(125812851, 128518257)
	mtime := testhelpers.Unix(257172893, 995216512)

	err = unix.Lstat(path, &fiOld)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentOld)
	require.NoErrorf(t, err, "lstat %s", path)

	err = Lutimes(path, atime, mtime)
	require.NoErrorf(t, err, "lutimes %s", path)

	err = unix.Lstat(path, &fiNew)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentNew)
	require.NoErrorf(t, err, "lstat %s", path)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	assert.NotEqual(t, atimeOld, atimeNew, "atime should change after lutimes")
	assert.Equal(t, atime, atimeNew, "new atime should match requested atime")

	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())
	assert.NotEqual(t, mtimeOld, mtimeNew, "mtime should change after lutimes")
	assert.Equal(t, mtime, mtimeNew, "new mtime should match requested mtime")

	assert.Equal(t, fiParentOld, fiParentNew, "stat data of parent directory should be unchanged")
}

func TestLutimesDirectory(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()

	path := filepath.Join(dir, " a directory  ")

	err := os.Mkdir(path, 0o755)
	require.NoError(t, err)

	atime := testhelpers.Unix(128551231, 273285257)
	mtime := testhelpers.Unix(185726393, 752135712)

	err = unix.Lstat(path, &fiOld)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentOld)
	require.NoErrorf(t, err, "lstat %s", path)

	err = Lutimes(path, atime, mtime)
	require.NoErrorf(t, err, "lutimes %s", path)

	err = unix.Lstat(path, &fiNew)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentNew)
	require.NoErrorf(t, err, "lstat %s", path)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	assert.NotEqual(t, atimeOld, atimeNew, "atime should change after lutimes")
	assert.Equal(t, atime, atimeNew, "new atime should match requested atime")

	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())
	assert.NotEqual(t, mtimeOld, mtimeNew, "mtime should change after lutimes")
	assert.Equal(t, mtime, mtimeNew, "new mtime should match requested mtime")

	assert.Equal(t, fiParentOld, fiParentNew, "stat data of parent directory should be unchanged")
}

func TestLutimesSymlink(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()

	path := filepath.Join(dir, "  a symlink   ")

	err := os.Symlink(".", path)
	require.NoError(t, err)

	atime := testhelpers.Unix(128551231, 273285257)
	mtime := testhelpers.Unix(185726393, 752135712)

	err = unix.Lstat(path, &fiOld)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentOld)
	require.NoErrorf(t, err, "lstat %s", path)

	err = Lutimes(path, atime, mtime)
	require.NoErrorf(t, err, "lutimes %s", path)

	err = unix.Lstat(path, &fiNew)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentNew)
	require.NoErrorf(t, err, "lstat %s", path)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	assert.NotEqual(t, atimeOld, atimeNew, "atime should change after lutimes")
	assert.Equal(t, atime, atimeNew, "new atime should match requested atime")

	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())
	assert.NotEqual(t, mtimeOld, mtimeNew, "mtime should change after lutimes")
	assert.Equal(t, mtime, mtimeNew, "new mtime should match requested mtime")

	assert.Equal(t, fiParentOld, fiParentNew, "stat data of parent directory should be unchanged")
}

func TestLutimesRelative(t *testing.T) {
	var fiOld, fiParentOld, fiNew, fiParentNew unix.Stat_t

	dir := t.TempDir()
	t.Chdir(dir)

	path := filepath.Join("some parent", " !! symlink here")
	var err error

	err = os.MkdirAll(filepath.Dir(path), 0o755)
	require.NoError(t, err)
	err = os.Symlink(".", path)
	require.NoError(t, err)

	atime := testhelpers.Unix(134858232, 258921237)
	mtime := testhelpers.Unix(171257291, 425815288)

	err = unix.Lstat(path, &fiOld)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentOld)
	require.NoErrorf(t, err, "lstat %s", path)

	err = Lutimes(path, atime, mtime)
	require.NoErrorf(t, err, "lutimes %s", path)

	err = unix.Lstat(path, &fiNew)
	require.NoErrorf(t, err, "lstat %s", path)
	err = unix.Lstat(dir, &fiParentNew)
	require.NoErrorf(t, err, "lstat %s", path)

	atimeOld := time.Unix(fiOld.Atim.Unix())
	atimeNew := time.Unix(fiNew.Atim.Unix())
	assert.NotEqual(t, atimeOld, atimeNew, "atime should change after lutimes")
	assert.Equal(t, atime, atimeNew, "new atime should match requested atime")

	mtimeOld := time.Unix(fiOld.Mtim.Unix())
	mtimeNew := time.Unix(fiNew.Mtim.Unix())
	assert.NotEqual(t, mtimeOld, mtimeNew, "mtime should change after lutimes")
	assert.Equal(t, mtime, mtimeNew, "new mtime should match requested mtime")

	// Make sure that the parent directory was unchanged.
	assert.Equal(t, fiParentOld, fiParentNew, "stat data of parent directory should be unchanged")
}
