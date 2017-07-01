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

package casext

import (
	"os"
	"syscall"
	"testing"
)

// readonly makes the given path read-only (by bind-mounting it as "ro").
// TODO: This should be done through an interface restriction in the test
//       (which is then backed up by the readonly mount if necessary). The fact
//       this test is necessary is a sign that we need a better split up of the
//       CAS interface.
// Copied from oci/cas/drivers/dir/dir_test.go.
func readonly(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Log("readonly tests only work with root privileges")
		t.Skip()
	}

	t.Logf("mounting %s as readonly", path)

	if err := syscall.Mount(path, path, "", syscall.MS_BIND|syscall.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
	if err := syscall.Mount("none", path, "", syscall.MS_BIND|syscall.MS_REMOUNT|syscall.MS_RDONLY, ""); err != nil {
		t.Fatalf("mount %s as ro: %s", path, err)
	}
}

// readwrite undoes the effect of readonly.
// Copied from oci/cas/drivers/dir/dir_test.go.
func readwrite(t *testing.T, path string) {
	if os.Geteuid() != 0 {
		t.Log("readonly tests only work with root privileges")
		t.Skip()
	}

	if err := syscall.Unmount(path, syscall.MNT_DETACH); err != nil {
		t.Fatalf("unmount %s: %s", path, err)
	}
}
