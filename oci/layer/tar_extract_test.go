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

package layer

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/umoci/pkg/testutils"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// TODO: Test the parent directory metadata is kept the same when unpacking.
// TODO: Add tests for metadata and consistency.

// testUnpackEntrySanitiseHelper is a basic helper to check that a tar header
// with the given prefix will resolve to the same path without it during
// unpacking. The "unsafe" version should resolve to the parent directory
// (which will be checked). The rootfs is assumed to be <dir>/rootfs.
func testUnpackEntrySanitiseHelper(t *testing.T, dir, file, prefix string) {
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

	te := NewTarExtractor(UnpackOptions{})
	if err := te.UnpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue)); err != nil {
		t.Fatalf("unexpected UnpackEntry error: %s", err)
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

// TestUnpackEntrySanitiseScoping makes sure that path sanitisation is done
// safely with regards to /../../ prefixes in invalid tar archives.
func TestUnpackEntrySanitiseScoping(t *testing.T) {
	for _, test := range []struct {
		name   string
		prefix string
	}{
		{"GarbagePrefix", "/.."},
		{"DotDotPrefix", ".."},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntrySanitiseScoping")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			rootfs := filepath.Join(dir, "rootfs")
			if err := os.Mkdir(rootfs, 0755); err != nil {
				t.Fatal(err)
			}

			testUnpackEntrySanitiseHelper(t, dir, filepath.Join("/", test.prefix, "file"), test.prefix)
		})
	}
}

// TestUnpackEntrySymlinkScoping makes sure that path sanitisation is done
// safely with regards to symlinks path components set to /.. and similar
// prefixes in invalid tar archives (a regular tar archive won't contain stuff
// like that).
func TestUnpackEntrySymlinkScoping(t *testing.T) {
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
		t.Run(test.name, func(t *testing.T) {
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

			testUnpackEntrySanitiseHelper(t, dir, filepath.Join("/", test.prefix, "file"), "link")
		})
	}
}

// TestUnpackEntryParentDir ensures that when UnpackEntry hits a path that
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

	te := NewTarExtractor(UnpackOptions{})
	if err := te.UnpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue)); err != nil {
		t.Fatalf("unexpected UnpackEntry error: %s", err)
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
		t.Run(test.name, func(t *testing.T) {
			testMtime := testutils.Unix(123, 456)
			testAtime := testutils.Unix(789, 111)

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

			te := NewTarExtractor(UnpackOptions{})
			if err := te.UnpackEntry(dir, hdr, nil); err != nil {
				t.Fatalf("unexpected error in UnpackEntry: %s", err)
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
		})
	}
}

type pseudoHdr struct {
	path     string
	linkname string
	typeflag byte
	upper    bool
}

func fromPseudoHdr(ph pseudoHdr) (*tar.Header, io.Reader) {
	var r io.Reader
	var size int64
	if ph.typeflag == tar.TypeReg || ph.typeflag == tar.TypeRegA {
		size = 256 * 1024
		r = &io.LimitedReader{
			R: rand.Reader,
			N: size,
		}
	}

	mode := os.FileMode(0777)
	if ph.typeflag == tar.TypeDir {
		mode |= os.ModeDir
	}

	return &tar.Header{
		Name:       ph.path,
		Linkname:   ph.linkname,
		Typeflag:   ph.typeflag,
		Mode:       int64(mode),
		Size:       size,
		ModTime:    testutils.Unix(1210393, 4528036),
		AccessTime: testutils.Unix(7892829, 2341211),
		ChangeTime: testutils.Unix(8731293, 8218947),
	}, r
}

// TestUnpackOpaqueWhiteout checks whether *opaque* whiteout handling is done
// correctly, as well as ensuring that the metadata of the parent is
// maintained -- and that upperdir entries are handled.
func TestUnpackOpaqueWhiteout(t *testing.T) {
	for _, test := range []struct {
		name          string
		ignoreExist   bool // ignore if extra upper files exist
		pseudoHeaders []pseudoHdr
	}{
		{"EmptyDir", false, nil},
		{"OneLevel", false, []pseudoHdr{
			{"file", "", tar.TypeReg, false},
			{"link", "..", tar.TypeSymlink, true},
			{"badlink", "./nothing", tar.TypeSymlink, true},
			{"fifo", "", tar.TypeFifo, false},
		}},
		{"OneLevelNoUpper", false, []pseudoHdr{
			{"file", "", tar.TypeReg, false},
			{"link", "..", tar.TypeSymlink, false},
			{"badlink", "./nothing", tar.TypeSymlink, false},
			{"fifo", "", tar.TypeFifo, false},
		}},
		{"TwoLevel", false, []pseudoHdr{
			{"file", "", tar.TypeReg, true},
			{"link", "..", tar.TypeSymlink, false},
			{"badlink", "./nothing", tar.TypeSymlink, false},
			{"dir", "", tar.TypeDir, true},
			{"dir/file", "", tar.TypeRegA, true},
			{"dir/link", "../badlink", tar.TypeSymlink, false},
			{"dir/verybadlink", "../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, true},
			{"dir/verybadlink2", "/../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, false},
		}},
		{"TwoLevelNoUpper", false, []pseudoHdr{
			{"file", "", tar.TypeReg, false},
			{"link", "..", tar.TypeSymlink, false},
			{"badlink", "./nothing", tar.TypeSymlink, false},
			{"dir", "", tar.TypeDir, false},
			{"dir/file", "", tar.TypeRegA, false},
			{"dir/link", "../badlink", tar.TypeSymlink, false},
			{"dir/verybadlink", "../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, false},
			{"dir/verybadlink2", "/../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, false},
		}},
		{"MultiLevel", false, []pseudoHdr{
			{"level1_file", "", tar.TypeReg, true},
			{"level1_link", "..", tar.TypeSymlink, false},
			{"level1a", "", tar.TypeDir, true},
			{"level1a/level2_file", "", tar.TypeRegA, false},
			{"level1a/level2_link", "../../../", tar.TypeSymlink, true},
			{"level1a/level2a", "", tar.TypeDir, false},
			{"level1a/level2a/level3_fileA", "", tar.TypeReg, false},
			{"level1a/level2a/level3_fileB", "", tar.TypeReg, false},
			{"level1a/level2b", "", tar.TypeDir, true},
			{"level1a/level2b/level3_fileA", "", tar.TypeReg, true},
			{"level1a/level2b/level3_fileB", "", tar.TypeReg, false},
			{"level1a/level2b/level3", "", tar.TypeDir, false},
			{"level1a/level2b/level3/level4", "", tar.TypeDir, false},
			{"level1a/level2b/level3/level4", "", tar.TypeDir, false},
			{"level1a/level2b/level3_fileA", "", tar.TypeReg, true},
			{"level1b", "", tar.TypeDir, false},
			{"level1b/level2_fileA", "", tar.TypeReg, false},
			{"level1b/level2_fileB", "", tar.TypeReg, false},
			{"level1b/level2", "", tar.TypeDir, false},
			{"level1b/level2/level3_file", "", tar.TypeReg, false},
		}},
		{"MultiLevelNoUpper", false, []pseudoHdr{
			{"level1_file", "", tar.TypeReg, false},
			{"level1_link", "..", tar.TypeSymlink, false},
			{"level1a", "", tar.TypeDir, false},
			{"level1a/level2_file", "", tar.TypeRegA, false},
			{"level1a/level2_link", "../../../", tar.TypeSymlink, false},
			{"level1a/level2a", "", tar.TypeDir, false},
			{"level1a/level2a/level3_fileA", "", tar.TypeReg, false},
			{"level1a/level2a/level3_fileB", "", tar.TypeReg, false},
			{"level1a/level2b", "", tar.TypeDir, false},
			{"level1a/level2b/level3_fileA", "", tar.TypeReg, false},
			{"level1a/level2b/level3_fileB", "", tar.TypeReg, false},
			{"level1a/level2b/level3", "", tar.TypeDir, false},
			{"level1a/level2b/level3/level4", "", tar.TypeDir, false},
			{"level1a/level2b/level3/level4", "", tar.TypeDir, false},
			{"level1a/level2b/level3_fileA", "", tar.TypeReg, false},
			{"level1b", "", tar.TypeDir, false},
			{"level1b/level2_fileA", "", tar.TypeReg, false},
			{"level1b/level2_fileB", "", tar.TypeReg, false},
			{"level1b/level2", "", tar.TypeDir, false},
			{"level1b/level2/level3_file", "", tar.TypeReg, false},
		}},
		{"MissingUpperAncestor", true, []pseudoHdr{
			{"some", "", tar.TypeDir, false},
			{"some/dir", "", tar.TypeDir, false},
			{"some/dir/somewhere", "", tar.TypeReg, true},
			{"another", "", tar.TypeDir, false},
			{"another/dir", "", tar.TypeDir, false},
			{"another/dir/somewhere", "", tar.TypeReg, false},
		}},
		{"UpperWhiteout", false, []pseudoHdr{
			{whPrefix + "fileB", "", tar.TypeReg, true},
			{"fileA", "", tar.TypeReg, true},
			{"fileB", "", tar.TypeReg, true},
			{"fileC", "", tar.TypeReg, false},
			{whPrefix + "fileA", "", tar.TypeReg, true},
			{whPrefix + "fileC", "", tar.TypeReg, true},
		}},
		// XXX: What umoci should do here is not really defined by the
		//      spec. In particular, whether you need a whiteout for every
		//      sub-path or just the path itself is not well-defined. This
		//      code assumes that you *do not*.
		{"UpperDirWhiteout", false, []pseudoHdr{
			{whPrefix + "dir2", "", tar.TypeReg, true},
			{"file", "", tar.TypeReg, false},
			{"dir1", "", tar.TypeDir, true},
			{"dir1/file", "", tar.TypeRegA, true},
			{"dir1/link", "../badlink", tar.TypeSymlink, false},
			{"dir1/verybadlink", "../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, true},
			{"dir1/verybadlink2", "/../../../../../../../../../../../../etc/shadow", tar.TypeSymlink, false},
			{whPrefix + "dir1", "", tar.TypeReg, true},
			{"dir2", "", tar.TypeDir, true},
			{"dir2/file", "", tar.TypeRegA, true},
			{"dir2/link", "../badlink", tar.TypeSymlink, false},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			unpackOptions := UnpackOptions{
				MapOptions: MapOptions{
					Rootless: os.Geteuid() != 0,
				},
			}

			dir, err := ioutil.TempDir("", "umoci-TestUnpackOpaqueWhiteout")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

			// We do all whiteouts in a subdirectory.
			whiteoutDir := "test-subdir"
			whiteoutRoot := filepath.Join(dir, whiteoutDir)
			if err := os.MkdirAll(whiteoutRoot, 0755); err != nil {
				t.Fatal(err)
			}

			// Track if we have upper entries.
			numUpper := 0

			// First we apply the non-upper files in a new TarExtractor.
			te := NewTarExtractor(unpackOptions)
			for _, ph := range test.pseudoHeaders {
				// Skip upper.
				if ph.upper {
					numUpper++
					continue
				}
				hdr, rdr := fromPseudoHdr(ph)
				hdr.Name = filepath.Join(whiteoutDir, hdr.Name)
				if err := te.UnpackEntry(dir, hdr, rdr); err != nil {
					t.Errorf("UnpackEntry %s failed: %v", hdr.Name, err)
				}
			}

			// Now we apply the upper files in another TarExtractor.
			te = NewTarExtractor(unpackOptions)
			for _, ph := range test.pseudoHeaders {
				// Skip non-upper.
				if !ph.upper {
					continue
				}
				hdr, rdr := fromPseudoHdr(ph)
				hdr.Name = filepath.Join(whiteoutDir, hdr.Name)
				if err := te.UnpackEntry(dir, hdr, rdr); err != nil {
					t.Errorf("UnpackEntry %s failed: %v", hdr.Name, err)
				}
			}

			// And now apply a whiteout for the whiteoutRoot.
			whHdr := &tar.Header{
				Name:     filepath.Join(whiteoutDir, whOpaque),
				Typeflag: tar.TypeReg,
			}
			if err := te.UnpackEntry(dir, whHdr, nil); err != nil {
				t.Fatalf("unpack whiteout %s failed: %v", whiteoutRoot, err)
			}

			// Now we double-check it worked. If the file was in "upper" it
			// should have survived. If it was in lower it shouldn't. We don't
			// bother checking the contents here.
			for _, ph := range test.pseudoHeaders {
				// If there's an explicit whiteout in the headers we ignore it
				// here, since it won't be on the filesystem.
				if strings.HasPrefix(filepath.Base(ph.path), whPrefix) {
					t.Logf("ignoring whiteout entry %q during post-check", ph.path)
					continue
				}

				fullPath := filepath.Join(whiteoutRoot, ph.path)
				_, err := te.fsEval.Lstat(fullPath)
				if err != nil && !os.IsNotExist(errors.Cause(err)) {
					t.Errorf("unexpected lstat error of %s: %v", ph.path, err)
				} else if ph.upper && err != nil {
					t.Errorf("expected upper %s to exist: got %v", ph.path, err)
				} else if !ph.upper && err == nil {
					if !test.ignoreExist {
						t.Errorf("expected lower %s to not exist", ph.path)
					}
				}
			}

			// Make sure the whiteoutRoot still exists.
			if fi, err := te.fsEval.Lstat(whiteoutRoot); err != nil {
				if os.IsNotExist(errors.Cause(err)) {
					t.Errorf("expected whiteout root to still exist: %v", err)
				} else {
					t.Errorf("unexpected error in lstat of whiteout root: %v", err)
				}
			} else if !fi.IsDir() {
				t.Errorf("expected whiteout root to still be a directory")
			}

			// Check that the directory is empty if there's no uppers.
			if numUpper == 0 {
				if fd, err := os.Open(whiteoutRoot); err != nil {
					t.Errorf("unexpected error opening whiteoutRoot: %v", err)
				} else if names, err := fd.Readdirnames(-1); err != nil {
					t.Errorf("unexpected error reading dirnames: %v", err)
				} else if len(names) != 0 {
					t.Errorf("expected empty opaque'd dir: got %v", names)
				}
			}
		})
	}
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

		// On MacOS, this might not work.
		hardlinkToSymlinkSupported = true

		ctrValue  = []byte("some content we won't check")
		regFile   = "regular"
		symFile   = "link"
		hardFileA = "hard link"
		hardFileB = "hard link to symlink"
	)

	te := NewTarExtractor(UnpackOptions{})

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
	if err := te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
		t.Fatalf("regular: unexpected UnpackEntry error: %s", err)
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
	if err := te.UnpackEntry(dir, hdr, nil); err != nil {
		t.Fatalf("hardlinkA: unexpected UnpackEntry error: %s", err)
	}

	// Symlink to regFile.
	hdr = &tar.Header{
		Name:     symFile,
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		Typeflag: tar.TypeSymlink,
		Linkname: filepath.Join("../../../", regFile),
	}
	if err := te.UnpackEntry(dir, hdr, nil); err != nil {
		t.Fatalf("symlink: unexpected UnpackEntry error: %s", err)
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
	if err := te.UnpackEntry(dir, hdr, nil); err != nil {
		// On Travis' setup, hardlinks to symlinks are not permitted under
		// MacOS. That's fine.
		if runtime.GOOS == "darwin" && errors.Is(err, unix.ENOTSUP) {
			hardlinkToSymlinkSupported = false
			t.Logf("hardlinks to symlinks unsupported -- skipping that part of the test")
		} else {
			t.Fatalf("hardlinkB: unexpected UnpackEntry error: %s", err)
		}
	}

	// Make sure that the contents are as expected.
	ctrValueGot, err := ioutil.ReadFile(filepath.Join(dir, regFile))
	if err != nil {
		t.Fatalf("regular file was not created: %s", err)
	}
	if !bytes.Equal(ctrValueGot, ctrValue) {
		t.Fatalf("regular file did not have expected values: expected=%s got=%s", ctrValue, ctrValueGot)
	}

	// Now we have to check the inode numbers.
	var regFi, symFi, hardAFi unix.Stat_t

	if err := unix.Lstat(filepath.Join(dir, regFile), &regFi); err != nil {
		t.Fatalf("could not stat regular file: %s", err)
	}
	if err := unix.Lstat(filepath.Join(dir, symFile), &symFi); err != nil {
		t.Fatalf("could not stat symlink: %s", err)
	}
	if err := unix.Lstat(filepath.Join(dir, hardFileA), &hardAFi); err != nil {
		t.Fatalf("could not stat hardlinkA: %s", err)
	}

	if regFi.Ino == symFi.Ino {
		t.Errorf("regular and symlink have the same inode! ino=%d", regFi.Ino)
	}
	if hardAFi.Ino != regFi.Ino {
		t.Errorf("hardlink to regular has a different inode: reg=%d hard=%d", regFi.Ino, hardAFi.Ino)
	}

	if hardlinkToSymlinkSupported {
		var hardBFi unix.Stat_t

		if err := unix.Lstat(filepath.Join(dir, hardFileB), &hardBFi); err != nil {
			t.Fatalf("could not stat hardlinkB: %s", err)
		}

		// Check inode numbers of hardlink-to-symlink.
		if hardAFi.Ino == hardBFi.Ino {
			t.Errorf("both hardlinks have the same inode! ino=%d", hardAFi.Ino)
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
		t.Skip("mapOptions tests only work with root privileges")
	}

	for _, test := range []struct {
		name   string
		uidMap rspec.LinuxIDMapping
		gidMap rspec.LinuxIDMapping
	}{
		{"IdentityRoot",
			rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100}},
		{"MapSelfToRoot",
			rspec.LinuxIDMapping{HostID: uint32(os.Getuid()), ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: uint32(os.Getgid()), ContainerID: 0, Size: 100}},
		{"MapOtherToRoot",
			rspec.LinuxIDMapping{HostID: uint32(os.Getuid() + 100), ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: uint32(os.Getgid() + 200), ContainerID: 0, Size: 100}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Create the files we're going to play with.
			dir, err := ioutil.TempDir("", "umoci-TestUnpackEntryMap")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(dir)

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

			te := NewTarExtractor(UnpackOptions{MapOptions: MapOptions{
				UIDMappings: []rspec.LinuxIDMapping{test.uidMap},
				GIDMappings: []rspec.LinuxIDMapping{test.gidMap},
			}})

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
			if err := te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regfile: unexpected UnpackEntry error: %s", err)
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
			if err := te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected UnpackEntry error: %s", err)
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
			if err := te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected UnpackEntry error: %s", err)
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
			if err := te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue)); err != nil {
				t.Fatalf("regdir: unexpected UnpackEntry error: %s", err)
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
		})
	}
}

func TestIsDirlink(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci-TestDirLink")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := os.Mkdir(filepath.Join(dir, "test_dir"), 0755); err != nil {
		t.Fatal(err)
	}
	if file, err := os.Create(filepath.Join(dir, "test_file")); err != nil {
		t.Fatal(err)
	} else {
		file.Close()
	}
	if err := os.Symlink("test_dir", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	te := NewTarExtractor(UnpackOptions{})
	// Basic symlink usage.
	if dirlink, err := te.isDirlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Errorf("symlink failed: %v", err)
	} else if !dirlink {
		t.Errorf("dirlink test failed")
	}

	// "Read" a non-existent link.
	if _, err := te.isDirlink(dir, filepath.Join(dir, "doesnt-exist")); err == nil {
		t.Errorf("read non-existent dirlink")
	}
	// "Read" a directory.
	if _, err := te.isDirlink(dir, filepath.Join(dir, "test_dir")); err == nil {
		t.Errorf("read non-link dirlink")
	}
	// "Read" a file.
	if _, err := te.isDirlink(dir, filepath.Join(dir, "test_file")); err == nil {
		t.Errorf("read non-link dirlink")
	}

	// Break the symlink.
	if err := os.Remove(filepath.Join(dir, "test_dir")); err != nil {
		t.Fatal(err)
	}
	if dirlink, err := te.isDirlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Errorf("broken symlink failed: %v", err)
	} else if dirlink {
		t.Errorf("broken dirlink test failed")
	}

	// Point the symlink to a file.
	if err := os.Remove(filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("test_file", filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}
	if dirlink, err := te.isDirlink(dir, filepath.Join(dir, "link")); err != nil {
		t.Errorf("file symlink failed: %v", err)
	} else if dirlink {
		t.Errorf("file dirlink test failed")
	}
}
