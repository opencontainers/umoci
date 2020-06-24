// +build linux

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

package testutils

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

// MakeReadOnly makes the given path read-only (by bind-mounting it as "ro").
// TODO: This should be done through an interface restriction in the test
//       (which is then backed up by the readonly mount if necessary). The fact
//       this test is necessary is a sign that we need a better split up of the
//       CAS interface.
func MakeReadOnly(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Skip("readonly tests only work with root privileges")
	}

	t.Logf("mounting %s as readonly", path)

	if err := unix.Mount(path, path, "", unix.MS_BIND|unix.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
	if err := unix.Mount("none", path, "", unix.MS_BIND|unix.MS_REMOUNT|unix.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
}

// MakeReadWrite undos the effect of MakeReadOnly.
func MakeReadWrite(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Skip("readonly tests only work with root privileges")
	}

	t.Logf("mounting %s as readwrite", path)

	if err := unix.Unmount(path, unix.MNT_DETACH); err != nil {
		t.Fatalf("unmount %s: %s", path, err)
	}
}
