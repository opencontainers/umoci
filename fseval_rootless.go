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
	"io"
	"os"
	"time"

	"github.com/cyphar/umoci/pkg/system"
	"github.com/cyphar/umoci/pkg/unpriv"
	"github.com/vbatts/go-mtree"
)

// RootlessFsEval is an FsEval implementation that uses "umoci/pkg/unpriv".*
// functions in order to provide the ability for unprivileged users (those
// without CAP_DAC_OVERRIDE and CAP_DAC_READ_SEARCH) to evaluate parts of a
// filesystem that they own. Note that by necessity this requires modifying the
// filesystem (and thus will not work on read-only filesystems).
var RootlessFsEval FsEval = unprivFsEval(0)

// unprivFsEval is a hack to be able to make RootlessFsEval a const.
type unprivFsEval int

// Open is a wrapper around unpriv.Open.
func (fs unprivFsEval) Open(path string) (*os.File, error) {
	return unpriv.Open(path)
}

// Create is a wrapper around unpriv.Create.
func (fs unprivFsEval) Create(path string) (*os.File, error) {
	return unpriv.Create(path)
}

// Readdir is a wrapper around unpriv.Readdir.
func (fs unprivFsEval) Readdir(path string) ([]os.FileInfo, error) {
	return unpriv.Readdir(path)
}

// Lstat is a wrapper around unpriv.Lstat.
func (fs unprivFsEval) Lstat(path string) (os.FileInfo, error) {
	return unpriv.Lstat(path)
}

// Readlink is a wrapper around unpriv.Readlink.
func (fs unprivFsEval) Readlink(path string) (string, error) {
	return unpriv.Readlink(path)
}

// Symlink is a wrapper around unpriv.Symlink.
func (fs unprivFsEval) Symlink(linkname, path string) error {
	return unpriv.Symlink(linkname, path)
}

// Link is a wrapper around unpriv.Link.
func (fs unprivFsEval) Link(linkname, path string) error {
	return unpriv.Link(linkname, path)
}

// Chmod is a wrapper around unpriv.Chmod.
func (fs unprivFsEval) Chmod(path string, mode os.FileMode) error {
	return unpriv.Chmod(path, mode)
}

// Lutimes is a wrapper around unpriv.Lutimes.
func (fs unprivFsEval) Lutimes(path string, atime, mtime time.Time) error {
	return unpriv.Lutimes(path, atime, mtime)
}

// Remove is a wrapper around unpriv.Remove.
func (fs unprivFsEval) Remove(path string) error {
	return unpriv.Remove(path)
}

// RemoveAll is a wrapper around unpriv.RemoveAll.
func (fs unprivFsEval) RemoveAll(path string) error {
	return unpriv.RemoveAll(path)
}

// Mkdir is a wrapper around unpriv.Mkdir.
func (fs unprivFsEval) Mkdir(path string, perm os.FileMode) error {
	return unpriv.Mkdir(path, perm)
}

// Mknod is equivalent to unpriv.Mknod.
func (fs unprivFsEval) Mknod(path string, mode os.FileMode, dev system.Dev_t) error {
	return unpriv.Mknod(path, mode, dev)
}

// MkdirAll is a wrapper around unpriv.MkdirAll.
func (fs unprivFsEval) MkdirAll(path string, perm os.FileMode) error {
	return unpriv.MkdirAll(path, perm)
}

// KeywordFunc returns a wrapper around the given mtree.KeywordFunc.
func (fs unprivFsEval) KeywordFunc(fn mtree.KeywordFunc) mtree.KeywordFunc {
	return func(path string, info os.FileInfo, r io.Reader) (mtree.KeyVal, error) {
		var kv mtree.KeyVal
		err := unpriv.Wrap(path, func(path string) error {
			var err error
			kv, err = fn(path, info, r)
			return err
		})
		return kv, err
	}
}
