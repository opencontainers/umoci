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

package layer

import (
	"archive/tar"
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// TODO: Test the parent directory metadata is kept the same when unpacking.
// TODO: Add tests for metadata and consistency.

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

		te := newTarExtractor(MapOptions{})
		if err := te.unpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue)); err != nil {
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

// TestUnpackEntryParentDir ensures that when unpackEntry hits a path that
// doesn't have its leading directories, we create all of the parent
// directories.
func TestUnpackEntryParentDir(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestUnpackEntryParentDir")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	rootfs := filepath.Join(dir, "rootfs")
	if err := os.Mkdir(rootfs, 0755); err != nil {
		t.Fatal(err)
	}

	ctrValue := []byte("creating parentdirs")

	// Create our header. We raw prepend the prefix because we are generating
	// invalid tar headers.
	hdr := &tar.Header{
		Name:       "a/b/c/file",
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Mode:       0644,
		Size:       int64(len(ctrValue)),
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}

	te := newTarExtractor(MapOptions{})
	if err := te.unpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue)); err != nil {
		t.Fatalf("unexpected unpackEntry error: %s", err)
	}

	ctrValueGot, err := ioutil.ReadFile(filepath.Join(rootfs, "a/b/c/file"))
	if err != nil {
		t.Fatalf("unexpected readfile error: %s", err)
	}

	if !bytes.Equal(ctrValue, ctrValueGot) {
		t.Errorf("ctr path was not updated: expected='%s' got='%s'", string(ctrValue), string(ctrValueGot))
	}
}

// TestUnpackEntryWhiteout checks whether whiteout handling is done correctly,
// as well as ensuring that the metadata of the parent is maintained.
func TestUnpackEntryWhiteout(t *testing.T) {
	// TODO: Modify this to use subtests once Go 1.7 is in enough places.
	func(t *testing.T) {
		for _, test := range []struct {
			name string
			path string
			dir  bool // TODO: Switch to Typeflag
		}{
			{"FileInRoot", "rootpath", false},
			{"HiddenFileInRoot", ".hiddenroot", false},
			{"FileInSubdir", "some/path/file", false},
			{"HiddenFileInSubdir", "another/path/.hiddenfile", false},
			{"DirInRoot", "rootpath", true},
			{"HiddenDirInRoot", ".hiddenroot", true},
			{"DirInSubdir", "some/path/dir", true},
			{"HiddenDirInSubdir", "another/path/.hiddendir", true},
		} {
			t.Logf("running Test%s", test.name)
			testMtime := time.Unix(123, 456)
			testAtime := time.Unix(789, 111)

			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntryWhiteout")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			rawDir, rawFile := filepath.Split(test.path)
			wh := filepath.Join(rawDir, whPrefix+rawFile)

			// Create the parent directory.
			if err := os.MkdirAll(filepath.Join(dir, rawDir), 0755); err != nil {
				t.Fatal(err)
			}

			// Create the path itself.
			if test.dir {
				if err := os.Mkdir(filepath.Join(dir, test.path), 0755); err != nil {
					t.Fatal(err)
				}
				// Make some subfiles and directories.
				if err := ioutil.WriteFile(filepath.Join(dir, test.path, "file1"), []byte("some value"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, test.path, "file2"), []byte("some value"), 0644); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(dir, test.path, ".subdir"), 0755); err != nil {
					t.Fatal(err)
				}
				if err := ioutil.WriteFile(filepath.Join(dir, test.path, ".subdir", "file3"), []byte("some value"), 0644); err != nil {
					t.Fatal(err)
				}
			} else {
				if err := ioutil.WriteFile(filepath.Join(dir, test.path), []byte("some value"), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Set the modified time of the directory itself.
			if err := os.Chtimes(filepath.Join(dir, rawDir), testAtime, testMtime); err != nil {
				t.Fatal(err)
			}

			// Whiteout the path.
			hdr := &tar.Header{
				Name:     wh,
				Typeflag: tar.TypeReg,
			}

			te := newTarExtractor(MapOptions{})
			if err := te.unpackEntry(dir, hdr, nil); err != nil {
				t.Fatalf("unexpected error in unpackEntry: %s", err)
			}

			// Make sure that the path is gone.
			if _, err := os.Lstat(filepath.Join(dir, test.path)); !os.IsNotExist(err) {
				if err != nil {
					t.Fatalf("unexpected error checking whiteout out path: %s", err)
				}
				t.Errorf("path was not removed by whiteout: %s", test.path)
			}

			// Make sure the parent directory wasn't modified.
			if fi, err := os.Lstat(filepath.Join(dir, rawDir)); err != nil {
				t.Fatalf("error checking parent directory of whiteout: %s", err)
			} else {
				hdr, err := tar.FileInfoHeader(fi, "")
				if err != nil {
					t.Fatalf("error generating header from fileinfo of parent directory of whiteout: %s", err)
				}

				if !hdr.ModTime.Equal(testMtime) {
					t.Errorf("mtime of parent directory changed after whiteout: got='%s' expected='%s'", hdr.ModTime, testMtime)
				}
				if !hdr.AccessTime.Equal(testAtime) {
					t.Errorf("atime of parent directory changed after whiteout: got='%s' expected='%s'", hdr.ModTime, testAtime)
				}
			}
		}
	}(t)
}

// TestUnpackHardlink makes sure that hardlinks are correctly unpacked in all
// cases. In particular when it comes to hardlinks to symlinks.
func TestUnpackHardlink(t *testing.T) {
	// Create the files we're going to play with.
	dir, err := ioutil.TempDir("", "umoci-TestUnpackHardlink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	var (
		hdr *tar.Header

		ctrValue  = []byte("some content we won't check")
		regFile   = "regular"
		symFile   = "link"
		hardFileA = "hard link"
		hardFileB = "hard link to symlink"
	)

	te := newTarExtractor(MapOptions{})

	// Regular file.
	hdr = &tar.Header{
		Name:       regFile,
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Mode:       0644,
		Size:       int64(len(ctrValue)),
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}
	if err := te.unpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
		t.Fatalf("regular: unexpected unpackEntry error: %s", err)
	}

	// Hardlink to regFile.
	hdr = &tar.Header{
		Name:     hardFileA,
		Typeflag: tar.TypeLink,
		Linkname: filepath.Join("/", regFile),
		// These should **not** be applied.
		Uid: os.Getuid() + 1337,
		Gid: os.Getgid() + 2020,
	}
	if err := te.unpackEntry(dir, hdr, nil); err != nil {
		t.Fatalf("hardlinkA: unexpected unpackEntry error: %s", err)
	}

	// Symlink to regFile.
	hdr = &tar.Header{
		Name:     symFile,
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		Typeflag: tar.TypeSymlink,
		Linkname: filepath.Join("../../../", regFile),
	}
	if err := te.unpackEntry(dir, hdr, nil); err != nil {
		t.Fatalf("symlink: unexpected unpackEntry error: %s", err)
	}

	// Hardlink to symlink.
	hdr = &tar.Header{
		Name:     hardFileB,
		Typeflag: tar.TypeLink,
		Linkname: filepath.Join("../../../", symFile),
		// These should **really not** be applied.
		Uid: os.Getuid() + 1337,
		Gid: os.Getgid() + 2020,
	}
	if err := te.unpackEntry(dir, hdr, nil); err != nil {
		t.Fatalf("hardlinkB: unexpected unpackEntry error: %s", err)
	}

	// Quickly make sure that the contents are as expected.
	ctrValueGot, err := ioutil.ReadFile(filepath.Join(dir, regFile))
	if err != nil {
		t.Fatalf("regular file was not created: %s", err)
	}
	if !bytes.Equal(ctrValueGot, ctrValue) {
		t.Fatalf("regular file did not have expected values: expected=%s got=%s", ctrValue, ctrValueGot)
	}

	// Now we have to check the inode numbers.
	var regFi, symFi, hardAFi, hardBFi unix.Stat_t

	if err := unix.Lstat(filepath.Join(dir, regFile), &regFi); err != nil {
		t.Fatalf("could not stat regular file: %s", err)
	}
	if err := unix.Lstat(filepath.Join(dir, symFile), &symFi); err != nil {
		t.Fatalf("could not stat symlink: %s", err)
	}
	if err := unix.Lstat(filepath.Join(dir, hardFileA), &hardAFi); err != nil {
		t.Fatalf("could not stat hardlinkA: %s", err)
	}
	if err := unix.Lstat(filepath.Join(dir, hardFileB), &hardBFi); err != nil {
		t.Fatalf("could not stat hardlinkB: %s", err)
	}

	// This test only runs on Linux anyway.

	if regFi.Ino == symFi.Ino {
		t.Errorf("regular and symlink have the same inode! ino=%d", regFi.Ino)
	}
	if hardAFi.Ino == hardBFi.Ino {
		t.Errorf("both hardlinks have the same inode! ino=%d", hardAFi.Ino)
	}
	if hardAFi.Ino != regFi.Ino {
		t.Errorf("hardlink to regular has a different inode: reg=%d hard=%d", regFi.Ino, hardAFi.Ino)
	}
	if hardBFi.Ino != symFi.Ino {
		t.Errorf("hardlink to symlink has a different inode: sym=%d hard=%d", symFi.Ino, hardBFi.Ino)
	}

	// Double-check readlink.
	linknameA, err := os.Readlink(filepath.Join(dir, symFile))
	if err != nil {
		t.Errorf("unexpected error reading symlink: %s", err)
	}
	linknameB, err := os.Readlink(filepath.Join(dir, hardFileB))
	if err != nil {
		t.Errorf("unexpected error reading hardlink to symlink: %s", err)
	}
	if linknameA != linknameB {
		t.Errorf("hardlink to symlink doesn't match linkname: link=%s hard=%s", linknameA, linknameB)
	}

	// Make sure that uid and gid don't apply to hardlinks.
	if int(regFi.Uid) != os.Getuid() {
		t.Errorf("regular file: uid was changed by hardlink unpack: expected=%d got=%d", os.Getuid(), regFi.Uid)
	}
	if int(regFi.Gid) != os.Getgid() {
		t.Errorf("regular file: gid was changed by hardlink unpack: expected=%d got=%d", os.Getgid(), regFi.Gid)
	}
	if int(symFi.Uid) != os.Getuid() {
		t.Errorf("symlink: uid was changed by hardlink unpack: expected=%d got=%d", os.Getuid(), symFi.Uid)
	}
	if int(symFi.Gid) != os.Getgid() {
		t.Errorf("symlink: gid was changed by hardlink unpack: expected=%d got=%d", os.Getgid(), symFi.Gid)
	}
}

// TestUnpackEntryMap checks that the mapOptions handling works.
func TestUnpackEntryMap(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Log("mapOptions tests only work with root privileges")
		t.Skip()
	}

	// TODO: Modify this to use subtests once Go 1.7 is in enough places.
	func(t *testing.T) {
		for _, test := range []struct {
			uidMap rspec.LinuxIDMapping
			gidMap rspec.LinuxIDMapping
		}{
			{rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100}, rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100}},
			{rspec.LinuxIDMapping{HostID: uint32(os.Getuid()), ContainerID: 0, Size: 100}, rspec.LinuxIDMapping{HostID: uint32(os.Getgid()), ContainerID: 0, Size: 100}},
			{rspec.LinuxIDMapping{HostID: uint32(os.Getuid() + 100), ContainerID: 0, Size: 100}, rspec.LinuxIDMapping{HostID: uint32(os.Getgid() + 200), ContainerID: 0, Size: 100}},
		} {
			// Create the files we're going to play with.
			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntryMap")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			t.Logf("running with uid=%#v gid=%#v", test.uidMap, test.gidMap)

			var (
				hdrUID, hdrGID, uUID, uGID int
				hdr                        *tar.Header
				fi                         unix.Stat_t

				ctrValue = []byte("some content we won't check")
				regFile  = "regular"
				symFile  = "link"
				regDir   = " a directory"
				symDir   = "link-dir"
			)

			te := newTarExtractor(MapOptions{
				UIDMappings: []rspec.LinuxIDMapping{test.uidMap},
				GIDMappings: []rspec.LinuxIDMapping{test.gidMap},
			})

			// Regular file.
			hdrUID, hdrGID = 0, 0
			hdr = &tar.Header{
				Name:       regFile,
				Uid:        hdrUID,
				Gid:        hdrGID,
				Mode:       0644,
				Size:       int64(len(ctrValue)),
				Typeflag:   tar.TypeReg,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			if err := te.unpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regfile: unexpected unpackEntry error: %s", err)
			}

			if err := unix.Lstat(filepath.Join(dir, hdr.Name), &fi); err != nil {
				t.Errorf("failed to lstat %s: %s", hdr.Name, err)
			} else {
				uUID = int(fi.Uid)
				uGID = int(fi.Gid)
				if uUID != int(test.uidMap.HostID)+hdrUID {
					t.Errorf("file %s has the wrong uid mapping: got=%d expected=%d", hdr.Name, uUID, int(test.uidMap.HostID)+hdrUID)
				}
				if uGID != int(test.gidMap.HostID)+hdrGID {
					t.Errorf("file %s has the wrong gid mapping: got=%d expected=%d", hdr.Name, uGID, int(test.gidMap.HostID)+hdrGID)
				}
			}

			// Regular directory.
			hdrUID, hdrGID = 13, 42
			hdr = &tar.Header{
				Name:       regDir,
				Uid:        hdrUID,
				Gid:        hdrGID,
				Mode:       0755,
				Typeflag:   tar.TypeDir,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			if err := te.unpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected unpackEntry error: %s", err)
			}

			if err := unix.Lstat(filepath.Join(dir, hdr.Name), &fi); err != nil {
				t.Errorf("failed to lstat %s: %s", hdr.Name, err)
			} else {
				uUID = int(fi.Uid)
				uGID = int(fi.Gid)
				if uUID != int(test.uidMap.HostID)+hdrUID {
					t.Errorf("file %s has the wrong uid mapping: got=%d expected=%d", hdr.Name, uUID, int(test.uidMap.HostID)+hdrUID)
				}
				if uGID != int(test.gidMap.HostID)+hdrGID {
					t.Errorf("file %s has the wrong gid mapping: got=%d expected=%d", hdr.Name, uGID, int(test.gidMap.HostID)+hdrGID)
				}
			}

			// Symlink to file.
			hdrUID, hdrGID = 23, 22
			hdr = &tar.Header{
				Name:       symFile,
				Uid:        hdrUID,
				Gid:        hdrGID,
				Typeflag:   tar.TypeSymlink,
				Linkname:   regFile,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			if err := te.unpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected unpackEntry error: %s", err)
			}

			if err := unix.Lstat(filepath.Join(dir, hdr.Name), &fi); err != nil {
				t.Errorf("failed to lstat %s: %s", hdr.Name, err)
			} else {
				uUID = int(fi.Uid)
				uGID = int(fi.Gid)
				if uUID != int(test.uidMap.HostID)+hdrUID {
					t.Errorf("file %s has the wrong uid mapping: got=%d expected=%d", hdr.Name, uUID, int(test.uidMap.HostID)+hdrUID)
				}
				if uGID != int(test.gidMap.HostID)+hdrGID {
					t.Errorf("file %s has the wrong gid mapping: got=%d expected=%d", hdr.Name, uGID, int(test.gidMap.HostID)+hdrGID)
				}
			}

			// Symlink to director.
			hdrUID, hdrGID = 99, 88
			hdr = &tar.Header{
				Name:       symDir,
				Uid:        hdrUID,
				Gid:        hdrGID,
				Typeflag:   tar.TypeSymlink,
				Linkname:   regDir,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			if err := te.unpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected unpackEntry error: %s", err)
			}

			if err := unix.Lstat(filepath.Join(dir, hdr.Name), &fi); err != nil {
				t.Errorf("failed to lstat %s: %s", hdr.Name, err)
			} else {
				uUID = int(fi.Uid)
				uGID = int(fi.Gid)
				if uUID != int(test.uidMap.HostID)+hdrUID {
					t.Errorf("file %s has the wrong uid mapping: got=%d expected=%d", hdr.Name, uUID, int(test.uidMap.HostID)+hdrUID)
				}
				if uGID != int(test.gidMap.HostID)+hdrGID {
					t.Errorf("file %s has the wrong gid mapping: got=%d expected=%d", hdr.Name, uGID, int(test.gidMap.HostID)+hdrGID)
				}
			}
		}
	}(t)
}
