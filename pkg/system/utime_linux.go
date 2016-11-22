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
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// Lutimes is a wrapper around utimensat(2), with the AT_SYMLINK_NOFOLLOW flag
// set, to allow changing the time of a symlink rather than the file it points
// to.
func Lutimes(path string, atime, mtime time.Time) error {
	var times [2]syscall.Timespec
	times[0] = syscall.NsecToTimespec(atime.UnixNano())
	times[1] = syscall.NsecToTimespec(mtime.UnixNano())

	// Open the parent directory.
	dirFile, err := os.OpenFile(filepath.Dir(path), syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_DIRECTORY, 0)
	if err != nil {
		return err
	}
	defer dirFile.Close()

	// The interface for this is really, really silly.
	_, _, errno := syscall.RawSyscall6(syscall.SYS_UTIMENSAT, // int utimensat(
		uintptr(dirFile.Fd()),              // int dirfd,
		uintptr(assertPtrFromString(path)), // char *pathname,
		uintptr(unsafe.Pointer(&times[0])), // struct timespec times[2],
		uintptr(_AT_SYMLINK_NOFOLLOW),      // int flags);
		0, 0)
	if errno != 0 {
		return &os.PathError{Op: "lutimes", Path: path, Err: errno}
	}
	return nil
}
