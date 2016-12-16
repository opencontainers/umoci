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
	"bytes"
	"os"
	"syscall"
	"unsafe"

	"github.com/pkg/errors"
)

// Llistxattr is a wrapper around llistxattr(2).
func Llistxattr(path string) ([]string, error) {
	bufsize, _, err := syscall.RawSyscall(syscall.SYS_LLISTXATTR, //. int llistxattr(
		uintptr(assertPtrFromString(path)), // char *path,
		0, // char *list,
		0) // size_t size);
	if err != 0 {
		return nil, errors.Wrap(err, "llistxattr: get bufsize")
	}

	if bufsize == 0 {
		return []string{}, nil
	}

	buffer := make([]byte, bufsize)
	n, _, err := syscall.RawSyscall(syscall.SYS_LLISTXATTR, // int llistxattr(
		uintptr(assertPtrFromString(path)),  // char *path,
		uintptr(unsafe.Pointer(&buffer[0])), // char *list,
		uintptr(bufsize))                    // size_t size);
	if err == syscall.ERANGE || n != bufsize {
		return nil, errors.Errorf("llistxattr: get buffer: xattr set changed")
	} else if err != 0 {
		return nil, errors.Wrap(err, "llistxattr: get buffer")
	}

	var xattrs []string
	for _, name := range bytes.Split(buffer, []byte{'\x00'}) {
		// "" is not a valid xattr (weirdly you get ERANGE -- not EINVAL -- if
		// you try to touch it). So just skip it.
		if len(name) == 0 {
			continue
		}
		xattrs = append(xattrs, string(name))
	}
	return xattrs, nil
}

// Lremovexattr is a wrapper around lremovexattr(2).
func Lremovexattr(path, name string) error {
	_, _, err := syscall.RawSyscall(syscall.SYS_LREMOVEXATTR, // int lremovexattr(
		uintptr(assertPtrFromString(path)), //.   char *path
		uintptr(assertPtrFromString(name)), //.   char *name);
		0)
	if err != 0 {
		return errors.Wrapf(err, "lremovexattr(%s, %s)", path, name)
	}
	return nil
}

// Lsetxattr is a wrapper around lsetxattr(2).
func Lsetxattr(path, name string, value []byte, flags int) error {
	_, _, err := syscall.RawSyscall6(syscall.SYS_LSETXATTR, // int lsetxattr(
		uintptr(assertPtrFromString(path)), //.   char *path,
		uintptr(assertPtrFromString(name)), //.   char *name,
		uintptr(unsafe.Pointer(&value[0])), //.   void *value,
		uintptr(len(value)),                //.   size_t size,
		uintptr(flags),                     //.   int flags);
		0)
	if err != 0 {
		return errors.Wrapf(err, "lsetxattr(%s, %s, %s, %d): %s", path, name, value, flags)
	}
	return nil
}

// Lgetxattr is a wrapper around lgetxattr(2).
func Lgetxattr(path string, name string) ([]byte, error) {
	bufsize, _, err := syscall.RawSyscall6(syscall.SYS_LGETXATTR, //. int lgetxattr(
		uintptr(assertPtrFromString(path)), // char *path,
		uintptr(assertPtrFromString(name)), // char *name,
		0, // void *value,
		0, // size_t size);
		0, 0)
	if err != 0 {
		return nil, errors.Wrap(err, "lgetxattr: get bufsize")
	}

	if bufsize == 0 {
		return []byte{}, nil
	}

	buffer := make([]byte, bufsize)
	n, _, err := syscall.RawSyscall6(syscall.SYS_LGETXATTR, // int lgetxattr(
		uintptr(assertPtrFromString(path)),  // char *path,
		uintptr(assertPtrFromString(name)),  // char *name,
		uintptr(unsafe.Pointer(&buffer[0])), // void *value,
		uintptr(bufsize),                    // size_t size);
		0, 0)
	if err == syscall.ERANGE || n != bufsize {
		return nil, errors.Errorf("lgetxattr: get buffer: xattr set changed")
	} else if err != 0 {
		return nil, errors.Wrap(err, "lgetxattr: get buffer")
	}

	return buffer, nil
}

// Lclearxattrs is a wrapper around Llistxattr and Lremovexattr, which attempts
// to remove all xattrs from a given file.
func Lclearxattrs(path string) error {
	names, err := Llistxattr(path)
	if err != nil {
		return errors.Wrap(err, "lclearxattrs: get list")
	}
	for _, name := range names {
		if err := Lremovexattr(path, name); err != nil {
			// Ignore permission errors, because hitting a permission error
			// means that it's a security.* xattr label or something similar.
			if os.IsPermission(errors.Cause(err)) {
				continue
			}
			return errors.Wrap(err, "lclearxattrs: remove xattr")
		}
	}
	return nil
}
