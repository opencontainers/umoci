/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017, 2018, 2018 SUSE LLC.
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
	"testing"

	"golang.org/x/sys/unix"
)

// Exhaustive test for Tarmode mapping.
func TestTarmode(t *testing.T) {
	for _, test := range []struct {
		typeflag byte
		mode     uint32
	}{
		{tar.TypeReg, 0},
		{tar.TypeSymlink, unix.S_IFLNK},
		{tar.TypeChar, unix.S_IFCHR},
		{tar.TypeBlock, unix.S_IFBLK},
		{tar.TypeFifo, unix.S_IFIFO},
		{tar.TypeDir, unix.S_IFDIR},
	} {
		mode := Tarmode(test.typeflag)
		if mode != test.mode {
			t.Errorf("got unexpected mode %x with tar typeflag %x, expected %x", mode, test.typeflag, test.mode)
		}
	}
}
