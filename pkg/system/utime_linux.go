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
	"fmt"
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

	// We can't use AT_FDCWD here. The reason is really stupid, and is because
	// Go's RawSyscall requires uintptr arguments. But AT_FDCWD is defined to
	// be a negative value. Go's own syscall.* implementations are not held to
	// this restriction due to how they're compiled, but we have to instead
	// fake our own AT_FDCWD by opening our current directory.
	dirfd, err := syscall.Open(".", _O_PATH|syscall.O_DIRECTORY|syscall.O_NOFOLLOW, 0)
	if err != nil {
		// This should really never be reached.
		return fmt.Errorf("lutimes: opening .: %s", err)
	}
	defer syscall.Close(dirfd)

	// The interface for this is really, really silly.
	_, _, errno := syscall.RawSyscall6(syscall.SYS_UTIMENSAT, // int utimensat(
		uintptr(dirfd),                     // int dirfd,
		uintptr(assertPtrFromString(path)), // char *pathname,
		uintptr(unsafe.Pointer(&times[0])), // struct timespec times[2],
		uintptr(_AT_SYMLINK_NOFOLLOW),      // int flags);
		0, 0)
	if errno != 0 {
		return fmt.Errorf("lutimes %s: %s", path, errno)
	}
	return nil
}
