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
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TODO: Test the parent directory metadata is kept the same when unpacking.
// TODO: Add tests for metadata and consistency.
// TODO: Add whiteout path tests.

// testUnpackEntrySanitiseHelper is a basic helper to check that a tar header
// with the given prefix will resolve to the same path without it during
// unpacking. The "unsafe" version should resolve to the parent directory
// (which will be checked). The rootfs is assumed to be <dir>/rootfs.
func testUnpackEntrySanitiseHelper(t *testing.T, dir, file, prefix string) func(t *testing.T) {
	// We return a function so that we can pass it directly to t.Run(...).
	return func(t *testing.T) {
		hostValue := []byte("host content")
		ctrValue := []byte("container content")

		rootfs := filepath.Join(dir, "rootfs")

		// Create a host file that we want to make sure doesn't get overwrittern.
		if err := ioutil.WriteFile(filepath.Join(dir, "file"), hostValue, 0644); err != nil {
			t.Fatal(err)
		}

		// Create our header. We raw prepend the prefix because we are generating
		// invalid tar headers.
		hdr := &tar.Header{
			Name:       prefix + "/" + filepath.Base(file),
			Uid:        os.Getuid(),
			Gid:        os.Getgid(),
			Mode:       0644,
			Size:       int64(len(ctrValue)),
			Typeflag:   tar.TypeReg,
			ModTime:    time.Now(),
			AccessTime: time.Now(),
			ChangeTime: time.Now(),
		}

		if err := unpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue)); err != nil {
			t.Fatalf("unexpected unpackEntry error: %s", err)
		}

		hostValueGot, err := ioutil.ReadFile(filepath.Join(dir, "file"))
		if err != nil {
			t.Fatalf("unexpected readfile error on host: %s", err)
		}

		ctrValueGot, err := ioutil.ReadFile(filepath.Join(rootfs, "file"))
		if err != nil {
			t.Fatalf("unexpected readfile error in ctr: %s", err)
		}

		if !bytes.Equal(ctrValue, ctrValueGot) {
			t.Errorf("ctr path was not updated: expected='%s' got='%s'", string(ctrValue), string(ctrValueGot))
		}
		if !bytes.Equal(hostValue, hostValueGot) {
			t.Errorf("HOST PATH WAS CHANGED! THIS IS A PATH ESCAPE! expected='%s' got='%s'", string(hostValue), string(hostValueGot))
		}
	}
}

// TestUnpackEntrySanitiseScoping makes sure that path sanitisation is done
// safely with regards to /../../ prefixes in invalid tar archives.
func TestUnpackEntrySanitiseScoping(t *testing.T) {
	// TODO: Modify this to use subtests once Go 1.7 is in enough places.
	func(t *testing.T) {
		for _, test := range []struct {
			name   string
			prefix string
		}{
			{"GarbagePrefix", "/.."},
			{"DotDotPrefix", ".."},
		} {
			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntrySanitiseScoping")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			rootfs := filepath.Join(dir, "rootfs")
			if err := os.Mkdir(rootfs, 0755); err != nil {
				t.Fatal(err)
			}

			t.Logf("running Test%s", test.name)
			testUnpackEntrySanitiseHelper(t, dir, filepath.Join("/", test.prefix, "file"), test.prefix)(t)
		}
	}(t)
}

// TestUnpackEntrySymlinkScoping makes sure that path sanitisation is done
// safely with regards to symlinks path components set to /.. and similar
// prefixes in invalid tar archives (a regular tar archive won't contain stuff
// like that).
func TestUnpackEntrySymlinkScoping(t *testing.T) {
	// TODO: Modify this to use subtests once Go 1.7 is in enough places.
	func(t *testing.T) {
		for _, test := range []struct {
			name   string
			prefix string
		}{
			{"RootPrefix", "/"},
			{"GarbagePrefix1", "/../"},
			{"GarbagePrefix2", "/../../../../../../../../../../../../../../../"},
			{"GarbagePrefix3", "/./.././.././.././.././.././.././.././.././../"},
			{"DotDotPrefix", ".."},
		} {
			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntrySymlinkScoping")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			rootfs := filepath.Join(dir, "rootfs")
			if err := os.Mkdir(rootfs, 0755); err != nil {
				t.Fatal(err)
			}

			// Create the symlink.
			if err := os.Symlink(test.prefix, filepath.Join(rootfs, "link")); err != nil {
				t.Fatal(err)
			}

			t.Logf("running Test%s", test.name)
			testUnpackEntrySanitiseHelper(t, dir, filepath.Join("/", test.prefix, "file"), "link")(t)
		}
	}(t)
}
