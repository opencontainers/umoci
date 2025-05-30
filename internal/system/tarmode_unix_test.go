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

package system

import (
	"archive/tar"
	"testing"

	"github.com/stretchr/testify/assert"
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
		assert.Equalf(t, test.mode, mode, "tar typeflag %v not converted to mode properly", test.typeflag)
	}
}
