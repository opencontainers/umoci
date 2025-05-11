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

package fseval

import (
	"os"
	"path/filepath"
	"time"

	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/funchelpers"
	"github.com/opencontainers/umoci/pkg/system"
)

// Default is the "identity" form of FsEval. In particular, it does not do any
// trickery and calls directly to the relevant os.* functions (and does not
// wrap KeywordFunc). This should be used by default, because there are no
// weird side-effects.
var Default FsEval = osFsEval(0)

// osFsEval is a hack to be able to make DefaultFsEval a const.
type osFsEval int

// Open is equivalent to os.Open.
func (fs osFsEval) Open(path string) (*os.File, error) {
	return os.Open(path)
}

// Create is equivalent to os.Create.
func (fs osFsEval) Create(path string) (*os.File, error) {
	return os.Create(path)
}

// Readdir is equivalent to os.Readdir.
func (fs osFsEval) Readdir(path string) (_ []os.FileInfo, Err error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer funchelpers.VerifyClose(&Err, fh)
	return fh.Readdir(-1)
}

// Lstat is equivalent to os.Lstat.
func (fs osFsEval) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

// Lstatx is equivalent to unix.Lstat.
func (fs osFsEval) Lstatx(path string) (unix.Stat_t, error) {
	var s unix.Stat_t
	err := unix.Lstat(path, &s)
	return s, err
}

// Readlink is equivalent to os.Readlink.
func (fs osFsEval) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

// Symlink is equivalent to os.Symlink.
func (fs osFsEval) Symlink(target, linkname string) error {
	return os.Symlink(target, linkname)
}

// Link is equivalent to unix.Link(..., ~AT_SYMLINK_FOLLOW).
func (fs osFsEval) Link(target, linkname string) error {
	// We need to explicitly pass 0 as a flag because POSIX allows the default
	// behaviour of link(2) when it comes to target being a symlink to be
	// implementation-defined. Only linkat(2) allows us to guarantee the right
	// behaviour.
	//  <https://pubs.opengroup.org/onlinepubs/9699919799/functions/link.html>
	return unix.Linkat(unix.AT_FDCWD, target, unix.AT_FDCWD, linkname, 0)
}

// Chmod is equivalent to os.Chmod.
func (fs osFsEval) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// Lutimes is equivalent to os.Lutimes.
func (fs osFsEval) Lutimes(path string, atime, mtime time.Time) error {
	return system.Lutimes(path, atime, mtime)
}

// RemoveAll is equivalent to os.RemoveAll.
func (fs osFsEval) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Mknod is equivalent to unix.Mknod.
func (fs osFsEval) Mknod(path string, mode os.FileMode, dev uint64) error {
	return system.Mknod(path, uint32(mode), dev)
}

// MkdirAll is equivalent to os.MkdirAll.
func (fs osFsEval) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Llistxattr is equivalent to system.Llistxattr.
func (fs osFsEval) Llistxattr(path string) ([]string, error) {
	return system.Llistxattr(path)
}

// Lremovexattr is equivalent to system.Lremovexattr.
func (fs osFsEval) Lremovexattr(path, name string) error {
	return unix.Lremovexattr(path, name)
}

// Lsetxattr is equivalent to system.Lsetxattr.
func (fs osFsEval) Lsetxattr(path, name string, value []byte, flags int) error {
	return unix.Lsetxattr(path, name, value, flags)
}

// Lgetxattr is equivalent to system.Lgetxattr.
func (fs osFsEval) Lgetxattr(path string, name string) ([]byte, error) {
	return system.Lgetxattr(path, name)
}

// Lclearxattrs is equivalent to system.Lclearxattrs.
func (fs osFsEval) Lclearxattrs(path string, skipFn func(xattrName string) bool) error {
	return system.Lclearxattrs(path, skipFn)
}

// KeywordFunc returns a wrapper around the given mtree.KeywordFunc.
func (fs osFsEval) KeywordFunc(fn mtree.KeywordFunc) mtree.KeywordFunc {
	return fn
}

// Walk is equivalent to filepath.Walk.
func (fs osFsEval) Walk(root string, fn filepath.WalkFunc) error {
	return filepath.Walk(root, fn)
}
