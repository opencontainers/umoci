// +build linux

/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2020 Cisco Inc.
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
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"golang.org/x/sys/unix"
)

func TestUnpackEntryOverlayFSWhiteout(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestOverlayFSWhiteout")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	err = unix.Mknod(filepath.Join(dir, "test"), syscall.S_IFCHR|0666, int(unix.Mkdev(0, 0)))
	if err != nil {
		if os.IsPermission(err) {
			t.Skip("skipping overlayfs test on kernel < 5.8")
		}

		t.Fatalf("couldn't mknod: %v", err)
	}

	headers := []pseudoHdr{
		{"file", "", tar.TypeReg, false},
		{whPrefix + "file", "", tar.TypeReg, false},
	}

	canSetTrustedXattrs := os.Geteuid() == 0

	if canSetTrustedXattrs {
		headers = append(headers, []pseudoHdr{
			{"dir", "", tar.TypeDir, false},
			{"dir/fileindir", "dir", tar.TypeReg, false},
			{"dir/" + whOpaque, "dir", tar.TypeReg, false},
		}...)
	}

	unpackOptions := UnpackOptions{
		MapOptions: MapOptions{
			Rootless: os.Geteuid() != 0,
		},
		WhiteoutMode: OverlayFSWhiteout,
	}

	te := NewTarExtractor(unpackOptions)

	for _, ph := range headers {
		hdr, rdr := fromPseudoHdr(ph)
		if err := te.UnpackEntry(dir, hdr, rdr); err != nil {
			t.Errorf("UnpackEntry %s failed: %v", hdr.Name, err)
		}
	}

	fi, err := os.Stat(filepath.Join(dir, "file"))
	if err != nil {
		t.Fatalf("failed to stat `file`: %v", err)
	}

	sysStat := fi.Sys().(*syscall.Stat_t)
	if unix.Major(sysStat.Rdev) != 0 || unix.Minor(sysStat.Rdev) != 0 {
		t.Fatalf("extract didn't make overlay whiteout %v", sysStat.Dev)
	}

	if canSetTrustedXattrs {
		value := make([]byte, 10)
		n, err := unix.Getxattr(filepath.Join(dir, "dir"), "trusted.overlay.opaque", value)
		if err != nil {
			t.Fatalf("failed to get overlay opaque attr: %v", err)
		}

		if string(value[:n]) != "y" {
			t.Fatalf("bad opaque xattr: %v", string(value[:n]))
		}
	}
}
