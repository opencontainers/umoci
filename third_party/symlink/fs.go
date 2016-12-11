// Copyright 2012 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.BSD file.

// This code is a modified version of path/filepath/symlink.go from the Go standard library.

package symlink

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/pkg/errors"
)

// FsEval is a mock-friendly (and unpriv.*) friendly way of wrapping
// filesystem-related functions. Note that this code (and all code referencing
// it) comes from this fork and is not present in the upstream code.
type FsEval struct {
	Lstat    func(path string) (fi os.FileInfo, err error)
	Readlink func(path string) (linkname string, err error)
}

// FollowSymlinkInScope is a wrapper around evalSymlinksInScope that returns an
// absolute path. This function handles paths in a platform-agnostic manner.
func FollowSymlinkInScope(path, root string) (string, error) {
	return FsEval{}.FollowSymlinkInScope(path, root)
}

func (fs FsEval) FollowSymlinkInScope(path, root string) (string, error) {
	// Default is os.*.
	if fs.Lstat == nil || fs.Readlink == nil {
		fs.Lstat = os.Lstat
		fs.Readlink = os.Readlink
	}
	path, err := filepath.Abs(filepath.FromSlash(path))
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(filepath.FromSlash(root))
	if err != nil {
		return "", err
	}
	return fs.evalSymlinksInScope(path, root)
}

// isNotExist tells you if err is an error that implies that either the path
// accessed does not exist (or path components don't exist).
func isNotExist(err error) bool {
	if os.IsNotExist(errors.Cause(err)) {
		return true
	}

	// Check that it's not actually an ENOTDIR.
	perr, ok := errors.Cause(err).(*os.PathError)
	if !ok {
		return false
	}
	errno, ok := perr.Err.(syscall.Errno)
	if !ok {
		return false
	}
	return errno == syscall.ENOTDIR || errno == syscall.ENOENT
}

// evalSymlinksInScope will evaluate symlinks in `path` within a scope `root` and return
// a result guaranteed to be contained within the scope `root`, at the time of the call.
// Symlinks in `root` are not evaluated and left as-is.
// Errors encountered while attempting to evaluate symlinks in path will be returned.
// Non-existing paths are valid and do not constitute an error.
// `path` has to contain `root` as a prefix, or else an error will be returned.
// Trying to break out from `root` does not constitute an error.
//
// Example:
//   If /foo/bar -> /outside,
//   FollowSymlinkInScope("/foo/bar", "/foo") == "/foo/outside" instead of "/oustide"
//
// IMPORTANT: it is the caller's responsibility to call evalSymlinksInScope *after* relevant symlinks
// are created and not to create subsequently, additional symlinks that could potentially make a
// previously-safe path, unsafe. Example: if /foo/bar does not exist, evalSymlinksInScope("/foo/bar", "/foo")
// would return "/foo/bar". If one makes /foo/bar a symlink to /baz subsequently, then "/foo/bar" should
// no longer be considered safely contained in "/foo".
func (fs FsEval) evalSymlinksInScope(path, root string) (string, error) {
	root = filepath.Clean(root)
	if path == root {
		return path, nil
	}
	if !strings.HasPrefix(path, root) {
		return "", errors.Errorf("evalSymlinksInScope: %s is not in %s", path, root)
	}
	const maxIter = 255
	originalPath := path
	// given root of "/a" and path of "/a/b/../../c" we want path to be "/b/../../c"
	path = path[len(root):]
	if root == string(filepath.Separator) {
		path = string(filepath.Separator) + path
	}
	if !strings.HasPrefix(path, string(filepath.Separator)) {
		return "", errors.Errorf("evalSymlinksInScope: %s is not in %s", path, root)
	}
	path = filepath.Clean(path)
	// consume path by taking each frontmost path element,
	// expanding it if it's a symlink, and appending it to b
	var b bytes.Buffer
	// b here will always be considered to be the "current absolute path inside
	// root" when we append paths to it, we also append a slash and use
	// filepath.Clean after the loop to trim the trailing slash
	for n := 0; path != ""; n++ {
		if n > maxIter {
			return "", errors.Errorf("evalSymlinksInScope: too many links in %s", originalPath)
		}

		// find next path component, p
		i := strings.IndexRune(path, filepath.Separator)
		var p string
		if i == -1 {
			p, path = path, ""
		} else {
			p, path = path[:i], path[i+1:]
		}

		if p == "" {
			continue
		}

		// this takes a b.String() like "b/../" and a p like "c" and turns it
		// into "/b/../c" which then gets filepath.Cleaned into "/c" and then
		// root gets prepended and we Clean again (to remove any trailing slash
		// if the first Clean gave us just "/")
		cleanP := filepath.Clean(string(filepath.Separator) + b.String() + p)
		if cleanP == string(filepath.Separator) {
			// never Lstat "/" itself
			b.Reset()
			continue
		}
		fullP := filepath.Clean(root + cleanP)

		fi, err := fs.Lstat(fullP)
		if isNotExist(err) {
			// if p does not exist, accept it
			b.WriteString(p)
			b.WriteRune(filepath.Separator)
			continue
		}
		if err != nil {
			return "", err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			b.WriteString(p)
			b.WriteRune(filepath.Separator)
			continue
		}

		// it's a symlink, put it at the front of path
		dest, err := fs.Readlink(fullP)
		if err != nil {
			return "", errors.Wrap(err, "evalSymlinksInScope: read symlink components")
		}
		if filepath.IsAbs(dest) {
			b.Reset()
		}
		path = dest + string(filepath.Separator) + path
	}

	// see note above on "fullP := ..." for why this is double-cleaned and
	// what's happening here
	return filepath.Clean(root + filepath.Clean(string(filepath.Separator)+b.String())), nil
}

// EvalSymlinks returns the path name after the evaluation of any symbolic
// links.
// If path is relative the result will be relative to the current directory,
// unless one of the components is an absolute symbolic link.
// This version has been updated to support long paths prepended with `\\?\`.
func EvalSymlinks(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}
