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

package layer

import (
	"archive/tar"
	"fmt"
	"os"
	"syscall"

	"github.com/cyphar/umoci/system"
)

// These values come from new_decode_dev() inside <linux/kdev_t.h>.
func major(device uint64) uint64 {
	return (device & 0xfff00) >> 8
}

// These values come from new_decode_dev() inside <linux/kdev_t.h>.
func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

func updateHeader(hdr *tar.Header, fi os.FileInfo) error {
	s, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return fmt.Errorf("failed to cast fileinfo to *syscall.stat_t")
	}

	// Currently the Go stdlib doesn't fill in the major/minor numbers of
	// devices, so we have to do it manually.
	if s.Mode&syscall.S_IFBLK == syscall.S_IFBLK || s.Mode&syscall.S_IFCHR == syscall.S_IFCHR {
		hdr.Devmajor = int64(system.Majordev(system.Dev_t(s.Rdev)))
		hdr.Devminor = int64(system.Minordev(system.Dev_t(s.Rdev)))
	}

	return nil
}

func getInode(fi os.FileInfo) (uint64, error) {
	s, ok := fi.Sys().(*syscall.Stat_t)
	if !ok {
		return 0, fmt.Errorf("failed to cast fileinfo to *syscall.stat_t")
	}
	return s.Ino, nil
}
