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

package umoci

import (
	"os"
	"time"

	"github.com/cyphar/umoci/pkg/system"
	"github.com/vbatts/go-mtree"
)

// DefaultFsEval is the "identity" form of FsEval. In particular, it does not
// do any trickery and calls directly to the relevant os.* functions (and does
// not wrap KeywordFunc). This should be used by default, because there are no
// weird side-effects.
var DefaultFsEval FsEval = osFsEval(0)

// unprivFsEval is a hack to be able to make DefaultFsEval a const.
type osFsEval int

// Open is a wrapper around unpriv.Open.
func (fs osFsEval) Open(path string) (*os.File, error) {
	return os.Open(path)
}

// Create is a wrapper around unpriv.Create.
func (fs osFsEval) Create(path string) (*os.File, error) {
	return os.Create(path)
}

// Readdir is a wrapper around unpriv.Readdir.
func (fs osFsEval) Readdir(path string) ([]os.FileInfo, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return fh.Readdir(-1)
}

// Lstat is a wrapper around unpriv.Lstat.
func (fs osFsEval) Lstat(path string) (os.FileInfo, error) {
	return os.Lstat(path)
}

// Readlink is a wrapper around unpriv.Readlink.
func (fs osFsEval) Readlink(path string) (string, error) {
	return os.Readlink(path)
}

// Symlink is a wrapper around unpriv.Symlink.
func (fs osFsEval) Symlink(linkname, path string) error {
	return os.Symlink(linkname, path)
}

// Link is a wrapper around unpriv.Link.
func (fs osFsEval) Link(linkname, path string) error {
	return os.Link(linkname, path)
}

// Chmod is a wrapper around unpriv.Chmod.
func (fs osFsEval) Chmod(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

// Lutimes is a wrapper around unpriv.Lutimes.
func (fs osFsEval) Lutimes(path string, atime, mtime time.Time) error {
	return system.Lutimes(path, atime, mtime)
}

// Remove is a wrapper around unpriv.Remove.
func (fs osFsEval) Remove(path string) error {
	return os.Remove(path)
}

// RemoveAll is a wrapper around unpriv.RemoveAll.
func (fs osFsEval) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Mkdir is a wrapper around unpriv.Mkdir.
func (fs osFsEval) Mkdir(path string, perm os.FileMode) error {
	return os.Mkdir(path, perm)
}

// Mknod is equivalent to system.Mknod.
func (fs osFsEval) Mknod(path string, mode os.FileMode, dev system.Dev_t) error {
	return system.Mknod(path, mode, dev)
}

// MkdirAll is a wrapper around unpriv.MkdirAll.
func (fs osFsEval) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

// KeywordFunc returns a wrapper around the given mtree.KeywordFunc.
func (fs osFsEval) KeywordFunc(fn mtree.KeywordFunc) mtree.KeywordFunc {
	return fn
}
