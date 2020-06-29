/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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
)

// FsEval is a super-interface that implements everything required for
// mtree.FsEval as well as including all of the imporant os.* wrapper functions
// needed for "oci/layers".tarExtractor.
type FsEval interface {
	// We inherit all of the base methods from mtree.FsEval.
	mtree.FsEval

	// Create is equivalent to os.Create.
	Create(path string) (*os.File, error)

	// Lstatx is equivalent to unix.Lstat.
	Lstatx(path string) (unix.Stat_t, error)

	// Readlink is equivalent to os.Readlink.
	Readlink(path string) (string, error)

	// Symlink is equivalent to os.Symlink.
	Symlink(linkname, path string) error

	// Link is equivalent to os.Link.
	Link(linkname, path string) error

	// Chmod is equivalent to os.Chmod.
	Chmod(path string, mode os.FileMode) error

	// Lutimes is equivalent to os.Lutimes.
	Lutimes(path string, atime, mtime time.Time) error

	// RemoveAll is equivalent to os.RemoveAll.
	RemoveAll(path string) error

	// MkdirAll is equivalent to os.MkdirAll.
	MkdirAll(path string, perm os.FileMode) error

	// Mknod is equivalent to unix.Mknod.
	Mknod(path string, mode os.FileMode, dev uint64) error

	// Llistxattr is equivalent to system.Llistxattr
	Llistxattr(path string) ([]string, error)

	// Lremovexattr is equivalent to system.Lremovexattr
	Lremovexattr(path, name string) error

	// Lsetxattr is equivalent to system.Lsetxattr
	Lsetxattr(path, name string, value []byte, flags int) error

	// Lgetxattr is equivalent to system.Lgetxattr
	Lgetxattr(path string, name string) ([]byte, error)

	// Lclearxattrs is equivalent to system.Lclearxattrs
	Lclearxattrs(path string, except map[string]struct{}) error

	// Walk is equivalent to filepath.Walk.
	Walk(root string, fn filepath.WalkFunc) error
}
