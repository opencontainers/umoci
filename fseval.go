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

	"github.com/vbatts/go-mtree"
)

// Ensure that mtree.FsEval is implemented by FsEval.
var _ mtree.FsEval = DefaultFsEval
var _ mtree.FsEval = RootlessFsEval

// FsEval is a super-interface that implements everything required for
// mtree.FsEval as well as including all of the imporant os.* wrapper functions
// needed for "oci/layers".tarExtractor.
type FsEval interface {
	// Open is a wrapper around unpriv.Open.
	Open(path string) (*os.File, error)

	// Create is a wrapper around unpriv.Create.
	Create(path string) (*os.File, error)

	// Readdir is a wrapper around unpriv.Readdir.
	Readdir(path string) ([]os.FileInfo, error)

	// Lstat is a wrapper around unpriv.Lstat.
	Lstat(path string) (os.FileInfo, error)

	// Readlink is a wrapper around unpriv.Readlink.
	Readlink(path string) (string, error)

	// Symlink is a wrapper around unpriv.Symlink.
	Symlink(linkname, path string) error

	// Link is a wrapper around unpriv.Link.
	Link(linkname, path string) error

	// Chmod is a wrapper around unpriv.Chmod.
	Chmod(path string, mode os.FileMode) error

	// Lutimes is a wrapper around unpriv.Lutimes.
	Lutimes(path string, atime, mtime time.Time) error

	// Remove is a wrapper around unpriv.Remove.
	Remove(path string) error

	// RemoveAll is a wrapper around unpriv.RemoveAll.
	RemoveAll(path string) error

	// Mkdir is a wrapper around unpriv.Mkdir.
	Mkdir(path string, perm os.FileMode) error

	// MkdirAll is a wrapper around unpriv.MkdirAll.
	MkdirAll(path string, perm os.FileMode) error

	// KeywordFunc returns a wrapper around the given mtree.KeywordFunc.
	KeywordFunc(fn mtree.KeywordFunc) mtree.KeywordFunc
}
