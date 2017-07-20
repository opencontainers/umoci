/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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
	"archive/tar"
	"os"

	"golang.org/x/sys/unix"
)

// Dev_t represents a dev_t structure.
type Dev_t uint64

// Tarmode takes a Typeflag (from a tar.Header for example) and returns the
// corresponding os.Filemode bit. Unknown typeflags are treated like regular
// files.
func Tarmode(typeflag byte) uint32 {
	switch typeflag {
	case tar.TypeSymlink:
		return unix.S_IFLNK
	case tar.TypeChar:
		return unix.S_IFCHR
	case tar.TypeBlock:
		return unix.S_IFBLK
	case tar.TypeFifo:
		return unix.S_IFIFO
	case tar.TypeDir:
		return unix.S_IFDIR
	}
	return 0
}

// Makedev produces a dev_t from the individual major and minor numbers,
// similar to makedev(3).
func Makedev(major, minor uint64) Dev_t {
	// These values come from new_envode_dev inside <linux/kdev_t.h>.
	return Dev_t((minor & 0xff) | (major << 8) | ((minor &^ 0xff) << 12))
}

// Majordev returns the major device number given a dev_t, similar to major(3).
func Majordev(device Dev_t) uint64 {
	// These values come from new_decode_dev() inside <linux/kdev_t.h>.
	return uint64((device & 0xfff00) >> 8)
}

// Minordev returns the minor device number given a dev_t, similar to minor(3).
func Minordev(device Dev_t) uint64 {
	// These values come from new_decode_dev() inside <linux/kdev_t.h>.
	return uint64((device & 0xff) | ((device >> 12) & 0xfff00))
}

// Mknod is a wrapper around mknod(2).
func Mknod(path string, mode os.FileMode, dev Dev_t) error {
	return unix.Mknod(path, uint32(mode), int(dev))
}
