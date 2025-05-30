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

// Package unpriv provides rootless emulation of CAP_DAC_READ_SEARCH without
// the need for rootless user namespaces. This is necessary in general because
// it turns out that a lot of distributions have a rootfs with `chmod 000`
// directories that rely on root having CAP_DAC_READ_SEARCH to be normally
// accessible.
//
// Note that the implementation of CAP_DAC_READ_SEARCH requires write access to
// any normally-inaccessible components of paths.
//
// Users should use fseval.FsEval instead to allow programs to switch between
// fseval.Rootless and fseval.Default based on whether the program is
// privileged or not.
package unpriv

import (
	"archive/tar"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	securejoin "github.com/cyphar/filepath-securejoin"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/system"
)

// fiRestore restores the state given by an os.FileInfo instance at the given
// path by ensuring that an Lstat(path) will return as-close-to the same
// os.FileInfo.
func fiRestore(path string, fi os.FileInfo) error {
	// archive/tar handles the OS-specific syscall stuff required to get atime
	// and mtime information for a file.
	hdr, err := tar.FileInfoHeader(fi, "")
	if err != nil {
		return err
	}

	// Apply the relevant information from the FileInfo.
	if err := os.Chmod(path, fi.Mode()); err != nil {
		return fmt.Errorf("restore mode: %w", err)
	}
	if err := os.Chtimes(path, hdr.AccessTime, hdr.ModTime); err != nil {
		return fmt.Errorf("restore times: %w", err)
	}
	return nil
}

// splitpath splits the given path into each of the path components.
func splitpath(path string) []string {
	path = filepath.Clean(path)
	parts := strings.Split(path, string(os.PathSeparator))
	if filepath.IsAbs(path) {
		parts = append([]string{string(os.PathSeparator)}, parts...)
	}
	return parts
}

// WrapFunc is a function that can be passed to Wrap. It takes a path (and
// presumably operates on it -- since Wrap only ensures that the path given is
// resolvable) and returns some form of error.
type WrapFunc func(path string) error

// Wrap will wrap a given function, and call it in a context where all of the
// parent directories in the given path argument are such that the path can be
// resolved (you may need to make your own changes to the path to make it
// readable). Note that the provided function may be called several times, and
// if the error returned is such that !os.IsPermission(err), then no trickery
// will be performed. If fn returns an error, so will this function. All of the
// trickery is reverted when this function returns (which is when fn returns).
func Wrap(path string, fn WrapFunc) (Err error) {
	// FIXME: Should we be calling fn() here first?
	if err := fn(path); err == nil || !errors.Is(err, os.ErrPermission) {
		return err
	}

	// We need to chown all of the path components we don't have execute rights
	// to. Specifically these are the path components which are parents of path
	// components we cannot stat. However, we must make sure to not touch the
	// path itself.
	parts := splitpath(filepath.Dir(path))
	start := len(parts)
	for {
		current := filepath.Join(parts[:start]...)
		_, err := os.Lstat(current)
		if err == nil {
			// We've hit the first element we can chown.
			break
		}
		if !errors.Is(err, os.ErrPermission) {
			// This is a legitimate error.
			return fmt.Errorf("unpriv.wrap: lstat parent: %s: %w", current, err)
		}
		start--
	}
	// Chown from the top down.
	for i := start; i <= len(parts); i++ {
		current := filepath.Join(parts[:i]...)
		fi, err := os.Lstat(current)
		if err != nil {
			return fmt.Errorf("unpriv.wrap: lstat parent: %s: %w", current, err)
		}
		// Add +rwx permissions to directories. If we have the access to change
		// the mode at all then we are the user owner (not just a group owner).
		if err := os.Chmod(current, fi.Mode()|0o700); err != nil {
			return fmt.Errorf("unpriv.wrap: chmod parent: %s: %w", current, err)
		}
		defer funchelpers.VerifyError(&Err, func() error {
			return fiRestore(current, fi)
		})
	}

	// Everything is wrapped. Return from this nightmare.
	return fn(path)
}

// Open is a wrapper around os.Open which has been wrapped with unpriv.Wrap to
// make it possible to open paths even if you do not currently have read
// permission. Note that the returned file handle references a path that you do
// not have read access to (since all changes are reverted when this function
// returns), so attempts to do Readdir() or similar functions that require
// doing lstat(2) may fail.
func Open(path string) (*os.File, error) {
	var fh *os.File
	err := Wrap(path, func(path string) (Err error) {
		// Get information so we can revert it.
		fi, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("lstat file: %w", err)
		}

		if fi.Mode()&0o400 != 0o400 {
			// Add +r permissions to the file.
			if err := os.Chmod(path, fi.Mode()|0o400); err != nil {
				return fmt.Errorf("chmod +r: %w", err)
			}
			defer funchelpers.VerifyError(&Err, func() error {
				return fiRestore(path, fi)
			})
		}

		// Open the damn thing.
		fh, err = os.Open(path)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.open: %w", err)
	}
	return fh, nil
}

// Create is a wrapper around os.Create which has been wrapped with unpriv.Wrap
// to make it possible to create paths even if you do not currently have read
// permission. Note that the returned file handle references a path that you do
// not have read access to (since all changes are reverted when this function
// returns).
func Create(path string) (*os.File, error) {
	var fh *os.File
	err := Wrap(path, func(path string) error {
		var err error
		fh, err = os.Create(path)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.create: %w", err)
	}
	return fh, nil
}

// Readdir is a wrapper around (*os.File).Readdir which has been wrapper with
// unpriv.Wrap to make it possible to get []os.FileInfo for the set of children
// of the provided directory path. The interface for this is quite different to
// (*os.File).Readdir because we have to have a proper filesystem path in order
// to get the set of child FileInfos (because all of the child paths need to be
// resolveable).
func Readdir(path string) ([]os.FileInfo, error) {
	var infos []os.FileInfo
	err := Wrap(path, func(path string) (Err error) {
		// Get information so we can revert it.
		fi, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("lstat dir: %w", err)
		}

		// Add +rx permissions to the file.
		if err := os.Chmod(path, fi.Mode()|0o500); err != nil {
			return fmt.Errorf("chmod +rx: %w", err)
		}
		defer funchelpers.VerifyError(&Err, func() error {
			return fiRestore(path, fi)
		})

		// Open the damn thing.
		fh, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("opendir: %w", err)
		}
		defer funchelpers.VerifyClose(&Err, fh)

		// Get the set of dirents.
		infos, err = fh.Readdir(-1)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.readdir: %w", err)
	}
	return infos, nil
}

// Lstat is a wrapper around os.Lstat which has been wrapped with unpriv.Wrap
// to make it possible to get os.FileInfo about a path even if you do not
// currently have the required mode bits set to resolve the path. Note that you
// may not have resolve access after this function returns because all of the
// trickery is reverted by unpriv.Wrap.
func Lstat(path string) (os.FileInfo, error) {
	var fi os.FileInfo
	err := Wrap(path, func(path string) error {
		// Fairly simple.
		var err error
		fi, err = os.Lstat(path)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.lstat: %w", err)
	}
	return fi, nil
}

// Lstatx is like Lstat but uses unix.Lstat and returns unix.Stat_t instead.
func Lstatx(path string) (unix.Stat_t, error) {
	var s unix.Stat_t
	err := Wrap(path, func(path string) error {
		return unix.Lstat(path, &s)
	})
	if err != nil {
		return s, fmt.Errorf("unpriv.lstatx: %w", err)
	}
	return s, nil
}

// Readlink is a wrapper around os.Readlink which has been wrapped with
// unpriv.Wrap to make it possible to get the target of a symlink even if you
// do not currently have the required mode bits set to resolve the path. Note
// that you may not have resolve access after this function returns because all
// of this trickery is reverted by unpriv.Wrap.
func Readlink(path string) (string, error) {
	var target string
	err := Wrap(path, func(path string) error {
		// Fairly simple.
		var err error
		target, err = os.Readlink(path)
		return err
	})
	if err != nil {
		return "", fmt.Errorf("unpriv.readlink: %w", err)
	}
	return target, nil
}

// Symlink is a wrapper around os.Symlink which has been wrapped with
// unpriv.Wrap to make it possible to create a symlink even if you do not
// currently have the required access bits to create the symlink. Note that you
// may not have resolve access after this function returns because all of the
// trickery is reverted by unpriv.Wrap.
func Symlink(target, linkname string) error {
	err := Wrap(linkname, func(linkname string) error { return os.Symlink(target, linkname) })
	if err != nil {
		return fmt.Errorf("unpriv.symlink: %w", err)
	}
	return nil
}

// Link is a wrapper around unix.Link(..., 0) which has been wrapped with
// unpriv.Wrap to make it possible to create a hard link even if you do not
// currently have the required access bits to create the hard link. Note that
// you may not have resolve access after this function returns because all of
// the trickery is reverted by unpriv.Wrap.
func Link(target, linkname string) error {
	err := Wrap(linkname, func(linkname string) error {
		// We have to double-wrap this, because you need search access to the
		// linkname. This is safe because any common ancestors will be reverted
		// in reverse call stack order.
		err := Wrap(target, func(target string) error {
			// We need to explicitly pass 0 as a flag because POSIX allows the
			// default behaviour of link(2) when it comes to target being a
			// symlink to be implementation-defined. Only linkat(2) allows us
			// to guarantee the right behaviour.
			//  <https://pubs.opengroup.org/onlinepubs/9699919799/functions/link.html>
			return unix.Linkat(unix.AT_FDCWD, target, unix.AT_FDCWD, linkname, 0)
		})
		if err != nil {
			return fmt.Errorf("unpriv.wrap target: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unpriv.link: %w", err)
	}
	return nil
}

// Chmod is a wrapper around os.Chmod which has been wrapped with unpriv.Wrap
// to make it possible to change the permission bits of a path even if you do
// not currently have the required access bits to access the path.
func Chmod(path string, mode os.FileMode) error {
	err := Wrap(path, func(path string) error { return os.Chmod(path, mode) })
	if err != nil {
		return fmt.Errorf("unpriv.chmod: %w", err)
	}
	return nil
}

// Chtimes is a wrapper around os.Chtimes which has been wrapped with
// unpriv.Wrap to make it possible to change the modified times of a path even
// if you do not currently have the required access bits to access the path.
func Chtimes(path string, atime, mtime time.Time) error {
	err := Wrap(path, func(path string) error { return os.Chtimes(path, atime, mtime) })
	if err != nil {
		return fmt.Errorf("unpriv.chtimes: %w", err)
	}
	return nil
}

// Lutimes is a wrapper around system.Lutimes which has been wrapped with
// unpriv.Wrap to make it possible to change the modified times of a path even
// if you do no currently have the required access bits to access the path.
func Lutimes(path string, atime, mtime time.Time) error {
	err := Wrap(path, func(path string) error { return system.Lutimes(path, atime, mtime) })
	if err != nil {
		return fmt.Errorf("unpriv.lutimes: %w", err)
	}
	return nil
}

// Remove is a wrapper around os.Remove which has been wrapped with unpriv.Wrap
// to make it possible to remove a path even if you do not currently have the
// required access bits to modify or resolve the path.
func Remove(path string) error {
	err := Wrap(path, os.Remove)
	if err != nil {
		return fmt.Errorf("unpriv.remove: %w", err)
	}
	return nil
}

// foreachSubpath executes WrapFunc for each child of the given path (not
// including the path itself). If path is not a directory, then WrapFunc will
// not be called and no error will be returned. This should be called within a
// context where path has already been made resolveable, however the . If WrapFunc returns an
// error, the first error is returned and iteration is halted.
func foreachSubpath(path string, wrapFn WrapFunc) (Err error) {
	// Is the path a directory?
	fi, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !fi.IsDir() {
		return nil
	}

	// Open the directory.
	fd, err := Open(path)
	if err != nil {
		return err
	}
	defer funchelpers.VerifyClose(&Err, fd)

	// We need to change the mode to Readdirnames. We don't need to worry about
	// permissions because we're already in a context with filepath.Dir(path)
	// is at least a+rx. However, because we are calling wrapFn we need to
	// restore the original mode immediately.
	if err := os.Chmod(path, fi.Mode()|0o444); err != nil {
		return fmt.Errorf("chmod +r to readdir: %w", err)
	}
	defer funchelpers.VerifyError(&Err, func() error {
		return fiRestore(path, fi)
	})

	names, err := fd.Readdirnames(-1)
	if err != nil {
		return err
	}

	// Make iteration order consistent.
	sort.Strings(names)

	// Call on all the sub-directories. We run it in a Wrap context to ensure
	// that the path we pass is resolveable when executed.
	for _, name := range names {
		subpath := filepath.Join(path, name)
		if err := Wrap(subpath, wrapFn); err != nil {
			return err
		}
	}
	return nil
}

// RemoveAll is similar to os.RemoveAll but with all of the internal functions
// wrapped with unpriv.Wrap to make it possible to remove a path (even if it
// has child paths) even if you do not currently have enough access bits.
func RemoveAll(path string) error {
	err := Wrap(path, func(path string) error {
		// If remove works, we're done.
		err := os.Remove(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}

		// Is this a directory?
		fi, serr := os.Lstat(path)
		if serr != nil {
			// Use securejoin's IsNotExist to handle ENOTDIR sanely.
			if !securejoin.IsNotExist(serr) {
				return fmt.Errorf("lstat: %w", serr)
			}
			return nil
		}
		// Return error from remove if it's not a directory.
		if !fi.IsDir() {
			return fmt.Errorf("remove non-directory: %w", err)
		}
		err = nil
		err1 := foreachSubpath(path, func(subpath string) error {
			err2 := RemoveAll(subpath)
			if err == nil {
				err = err2
			}
			return nil
		})
		if err1 != nil {
			// We must have hit a race, but we don't care.
			if errors.Is(err1, os.ErrNotExist) {
				err1 = nil
			}
			return fmt.Errorf("foreach subpath: %w", err1)
		}

		// Remove the directory. This should now work.
		err1 = os.Remove(path)
		if err1 == nil || errors.Is(err1, os.ErrNotExist) {
			return nil
		}
		if err == nil {
			err = err1
		}
		return fmt.Errorf("remove: %w", err)
	})
	if err != nil {
		return fmt.Errorf("unpriv.removeall: %w", err)
	}
	return nil
}

// Mkdir is a wrapper around os.Mkdir which has been wrapped with unpriv.Wrap
// to make it possible to remove a path even if you do not currently have the
// required access bits to modify or resolve the path.
func Mkdir(path string, perm os.FileMode) error {
	err := Wrap(path, func(path string) error { return os.Mkdir(path, perm) })
	if err != nil {
		return fmt.Errorf("unpriv.mkdir: %w", err)
	}
	return nil
}

// MkdirAll is similar to os.MkdirAll but in order to implement it properly all
// of the internal functions were wrapped with unpriv.Wrap to make it possible
// to create a path even if you do not currently have enough access bits.
func MkdirAll(path string, perm os.FileMode) error {
	err := Wrap(path, func(path string) error {
		// Check whether the path already exists.
		fi, err := os.Stat(path)
		if err == nil {
			if fi.IsDir() {
				return nil
			}
			return &os.PathError{Op: "mkdir", Path: path, Err: unix.ENOTDIR}
		}

		// Create parent.
		parent := filepath.Dir(path)
		if parent != "." && parent != "/" {
			err = MkdirAll(parent, perm)
			if err != nil {
				return err
			}
		}

		// Parent exists, now we can create the path.
		err = os.Mkdir(path, perm)
		if err != nil {
			// Handle "foo/.".
			fi, err1 := os.Lstat(path)
			if err1 == nil && fi.IsDir() {
				return nil
			}
			return err
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("unpriv.mkdirall: %w", err)
	}
	return nil
}

// Mknod is a wrapper around unix.Mknod which has been wrapped with unpriv.Wrap
// to make it possible to remove a path even if you do not currently have the
// required access bits to modify or resolve the path.
func Mknod(path string, mode os.FileMode, dev uint64) error {
	err := Wrap(path, func(path string) error { return system.Mknod(path, uint32(mode), dev) })
	if err != nil {
		return fmt.Errorf("unpriv.mknod: %w", err)
	}
	return nil
}

// Llistxattr is a wrapper around system.Llistxattr which has been wrapped with
// unpriv.Wrap to make it possible to remove a path even if you do not
// currently have the required access bits to resolve the path.
func Llistxattr(path string) ([]string, error) {
	var xattrs []string
	err := Wrap(path, func(path string) error {
		var err error
		xattrs, err = system.Llistxattr(path)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.llistxattr: %w", err)
	}
	return xattrs, nil
}

// Lremovexattr is a wrapper around system.Lremovexattr which has been wrapped
// with unpriv.Wrap to make it possible to remove a path even if you do not
// currently have the required access bits to resolve the path.
func Lremovexattr(path, name string) error {
	err := Wrap(path, func(path string) error { return unix.Lremovexattr(path, name) })
	if err != nil {
		return fmt.Errorf("unpriv.lremovexattr: %w", err)
	}
	return nil
}

// Lsetxattr is a wrapper around system.Lsetxattr which has been wrapped
// with unpriv.Wrap to make it possible to set a path even if you do not
// currently have the required access bits to resolve the path.
func Lsetxattr(path, name string, value []byte, flags int) error {
	err := Wrap(path, func(path string) error { return unix.Lsetxattr(path, name, value, flags) })
	if err != nil {
		return fmt.Errorf("unpriv.lsetxattr: %w", err)
	}
	return nil
}

// Lgetxattr is a wrapper around system.Lgetxattr which has been wrapped
// with unpriv.Wrap to make it possible to get a path even if you do not
// currently have the required access bits to resolve the path.
func Lgetxattr(path, name string) ([]byte, error) {
	var value []byte
	err := Wrap(path, func(path string) error {
		var err error
		value, err = system.Lgetxattr(path, name)
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("unpriv.lgetxattr: %w", err)
	}
	return value, nil
}

// Lclearxattrs is a wrapper around system.Lclearxattrs which has been wrapped
// with unpriv.Wrap to make it possible to get a path even if you do not
// currently have the required access bits to resolve the path.
func Lclearxattrs(path string, skipFn func(xattrName string) bool) error {
	err := Wrap(path, func(path string) error { return system.Lclearxattrs(path, skipFn) })
	if err != nil {
		return fmt.Errorf("unpriv.lclearxattrs: %w", err)
	}
	return nil
}

// walk is the inner implementation of Walk.
func walk(path string, info os.FileInfo, walkFn filepath.WalkFunc) error {
	// Always run walkFn first. If we're not a directory there's no children to
	// iterate over and so we bail even if there wasn't an error.
	err := walkFn(path, info, nil)
	if !info.IsDir() || err != nil {
		return err
	}

	// Now just execute walkFn over each subpath.
	// TODO: We should handle the Readdirnames failing case that stdlib does.
	return foreachSubpath(path, func(subpath string) error {
		info, err := Lstat(subpath)
		if err != nil {
			// If it doesn't exist, just pass it directly to walkFn.
			if err := walkFn(subpath, info, err); err != nil {
				// To match stdlib, SkipDir assumes a non-existent path is a
				// directory and so SkipDir just skips that path.
				if errors.Is(err, filepath.SkipDir) {
					return err
				}
			}
		} else {
			if err := walk(subpath, info, walkFn); err != nil {
				// If this entry is a directory then SkipDir will just skip
				// this entry and continue walking the current directory, but
				// otherwise we need to skip the whole directory. This matches
				// the stdlib behaviour.
				if !(info.IsDir() && errors.Is(err, filepath.SkipDir)) { //nolint:staticcheck // QF1001: this form is easier to understand
					return err
				}
			}
		}
		return nil
	})
}

// Walk is a reimplementation of filepath.Walk, wrapping all of the relevant
// function calls with Wrap, allowing you to walk over a tree even in the face
// of multiple nested cases where paths are not normally accessible. The
// os.FileInfo passed to walkFn is the "pristine" version (as opposed to the
// currently-on-disk version that may have been temporarily modified by Wrap).
func Walk(root string, walkFn filepath.WalkFunc) error {
	return Wrap(root, func(root string) error {
		info, err := Lstat(root)
		if err != nil {
			err = walkFn(root, nil, err)
		} else {
			err = walk(root, info, walkFn)
		}
		if err == nil || errors.Is(err, filepath.SkipDir) {
			return nil
		}
		return fmt.Errorf("unpriv.walk: %w", err)
	})
}
