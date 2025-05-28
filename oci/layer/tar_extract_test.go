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

package layer

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/testhelpers"
	"github.com/opencontainers/umoci/pkg/fseval"
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
	err := os.WriteFile(filepath.Join(dir, file), hostValue, 0o644)
	require.NoError(t, err)

	// Create our header. We raw prepend the prefix because we are generating
	// invalid tar headers.
	hdr := &tar.Header{
		Name:       prefix + "/" + filepath.Base(file),
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Mode:       0o644,
		Size:       int64(len(ctrValue)),
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}

	te := NewTarExtractor(nil)
	err = te.UnpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue))
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	hostValueGot, err := os.ReadFile(filepath.Join(dir, file))
	require.NoError(t, err, "read host file")

	ctrValueGot, err := os.ReadFile(filepath.Join(rootfs, file))
	require.NoError(t, err, "read ctr file")

	assert.Equal(t, ctrValue, ctrValueGot, "ctr path was not updated")
	assert.Equal(t, hostValue, hostValueGot, "HOST PATH WAS CHANGED! THIS IS A PATH ESCAPE!")
}

func assertIsDirectory(t *testing.T, fsEval fseval.FsEval, path string) {
	stat, err := fsEval.Lstatx(path)
	require.NoError(t, err, "stat path")
	assert.EqualValuesf(t, unix.S_IFDIR, stat.Mode&unix.S_IFMT, "%q should be a directory (S_IFDIR)", path)
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
			dir := t.TempDir()

			rootfs := filepath.Join(dir, "rootfs")
			err := os.Mkdir(rootfs, 0o755)
			require.NoError(t, err, "mkdir rootfs")

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
			dir := t.TempDir()

			rootfs := filepath.Join(dir, "rootfs")
			err := os.Mkdir(rootfs, 0o755)
			require.NoError(t, err, "mkdir rootfs")

			// Create the symlink.
			err = os.Symlink(test.prefix, filepath.Join(rootfs, "link"))
			require.NoError(t, err, "make symlink")

			testUnpackEntrySanitiseHelper(t, dir, filepath.Join("/", test.prefix, "file"), "link")
		})
	}
}

// TestUnpackEntryNonDirParent makes sure that extracting a subpath underneath
// a non-directory will result in no errors and the directory should instead
// https://github.com/opencontainers/umoci/issues/546
func TestUnpackEntryNonDirParent(t *testing.T) {
	dir := t.TempDir()

	dentries := []tarDentry{
		{path: "nondir", ftype: tar.TypeReg},
		{path: "nondir/foo/bar/baz", ftype: tar.TypeReg},
	}

	unpackOptions := UnpackOptions{
		OnDiskFormat: DirRootfs{
			MapOptions: MapOptions{
				Rootless: os.Geteuid() != 0,
			},
		},
	}

	te := NewTarExtractor(&unpackOptions)

	for _, de := range dentries {
		hdr, rdr := tarFromDentry(de)
		err := te.UnpackEntry(dir, hdr, rdr)
		require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
	}

	assertIsDirectory(t, te.fsEval, filepath.Join(dir, "nondir"))
	assertIsDirectory(t, te.fsEval, filepath.Join(dir, "nondir/foo"))
	assertIsDirectory(t, te.fsEval, filepath.Join(dir, "nondir/foo/bar"))
	assert.FileExists(t, filepath.Join(dir, "nondir/foo/bar/baz"))
}

// TestUnpackEntryParentDir ensures that when UnpackEntry hits a path that
// doesn't have its leading directories, we create all of the parent
// directories.
func TestUnpackEntryParentDir(t *testing.T) {
	rootfs := filepath.Join(t.TempDir(), "rootfs")
	err := os.Mkdir(rootfs, 0o755)
	require.NoError(t, err, "mkdir rootfs")

	ctrValue := []byte("creating parentdirs")

	// Create our header. We raw prepend the prefix because we are generating
	// invalid tar headers.
	hdr := &tar.Header{
		Name:       "a/b/c/file",
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Mode:       0o644,
		Size:       int64(len(ctrValue)),
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}

	te := NewTarExtractor(nil)

	err = te.UnpackEntry(rootfs, hdr, bytes.NewBuffer(ctrValue))
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	ctrValueGot, err := os.ReadFile(filepath.Join(rootfs, "a/b/c/file"))
	require.NoError(t, err, "read ctr file")
	assert.Equal(t, ctrValue, ctrValueGot, "ctr path was not updated")
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
			testMtime := testhelpers.Unix(123, 456)
			testAtime := testhelpers.Unix(789, 111)

			dir := t.TempDir()

			rawDir, rawFile := filepath.Split(test.path)
			wh := filepath.Join(rawDir, whPrefix+rawFile)

			// Create the parent directory.
			err := os.MkdirAll(filepath.Join(dir, rawDir), 0o755)
			require.NoError(t, err, "mkdir parent directory")

			// Create the path itself.
			if test.dir {
				err := os.Mkdir(filepath.Join(dir, test.path), 0o755)
				require.NoError(t, err)

				// Make some subfiles and directories.
				err = os.WriteFile(filepath.Join(dir, test.path, "file1"), []byte("some value"), 0o644)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(dir, test.path, "file2"), []byte("some value"), 0o644)
				require.NoError(t, err)

				err = os.Mkdir(filepath.Join(dir, test.path, ".subdir"), 0o755)
				require.NoError(t, err)

				err = os.WriteFile(filepath.Join(dir, test.path, ".subdir", "file3"), []byte("some value"), 0o644)
				require.NoError(t, err)
			} else {
				err := os.WriteFile(filepath.Join(dir, test.path), []byte("some value"), 0o644)
				require.NoError(t, err)
			}

			// Set the modified time of the directory itself.
			err = os.Chtimes(filepath.Join(dir, rawDir), testAtime, testMtime)
			require.NoError(t, err)

			// Whiteout the path.
			hdr := &tar.Header{
				Name:     wh,
				Typeflag: tar.TypeReg,
			}

			te := NewTarExtractor(nil)
			err = te.UnpackEntry(dir, hdr, nil)
			require.NoErrorf(t, err, "UnpackEntry %s whiteout", hdr.Name)

			// Make sure that the path is gone.
			_, err = os.Lstat(filepath.Join(dir, test.path))
			assert.ErrorIs(t, err, os.ErrNotExist, "whiteout should have removed path") //nolint:testifylint // assert.*Error* makes more sense

			// Make sure the parent directory wasn't modified.
			fi, err := os.Lstat(filepath.Join(dir, rawDir))
			require.NoError(t, err, "stat parent directory")

			hdr, err = tar.FileInfoHeader(fi, "")
			require.NoError(t, err, "convert parent directory to tar header")
			assert.Equal(t, testMtime, hdr.ModTime, "mtime of parent directory")
			assert.Equal(t, testAtime, hdr.AccessTime, "atime of parent directory")
		})
	}
}

// TestUnpackOpaqueWhiteout checks whether *opaque* whiteout handling is done
// correctly, as well as ensuring that the metadata of the parent is
// maintained -- and that upperdir entries are handled.
func TestUnpackOpaqueWhiteout(t *testing.T) {
	type layeredTarDentry struct {
		tarDentry
		upper, shouldSurvive bool
	}

	for _, test := range []struct {
		name     string
		dentries []layeredTarDentry
	}{
		{"EmptyDir", nil},
		{"OneLevel", []layeredTarDentry{
			{tarDentry{path: "file", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "link", ftype: tar.TypeSymlink, linkname: ".."}, true, true},
			{tarDentry{path: "badlink", ftype: tar.TypeSymlink, linkname: "./nothing"}, true, true},
			{tarDentry{path: "fifo", ftype: tar.TypeFifo}, false, false},
		}},
		{"OneLevelNoUpper", []layeredTarDentry{
			{tarDentry{path: "file", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "link", ftype: tar.TypeSymlink, linkname: ".."}, false, false},
			{tarDentry{path: "badlink", ftype: tar.TypeSymlink, linkname: "./nothing"}, false, false},
			{tarDentry{path: "fifo", ftype: tar.TypeFifo}, false, false},
		}},
		{"TwoLevel", []layeredTarDentry{
			{tarDentry{path: "file", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "link", ftype: tar.TypeSymlink, linkname: ".."}, false, false},
			{tarDentry{path: "badlink", ftype: tar.TypeSymlink, linkname: "./nothing"}, false, false},
			{tarDentry{path: "dir", ftype: tar.TypeDir}, true, true},
			{tarDentry{path: "dir/file", ftype: tar.TypeRegA}, true, true}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "dir/link", ftype: tar.TypeSymlink, linkname: "../badlink"}, false, false},
			{tarDentry{path: "dir/verybadlink", ftype: tar.TypeSymlink, linkname: "../../../../../../../../../../../../etc/shadow"}, true, true},
			{tarDentry{path: "dir/verybadlink2", ftype: tar.TypeSymlink, linkname: "/../../../../../../../../../../../../etc/shadow"}, false, false},
		}},
		{"TwoLevelNoUpper", []layeredTarDentry{
			{tarDentry{path: "file", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "link", ftype: tar.TypeSymlink, linkname: ".."}, false, false},
			{tarDentry{path: "badlink", ftype: tar.TypeSymlink, linkname: "./nothing"}, false, false},
			{tarDentry{path: "dir", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "dir/file", ftype: tar.TypeRegA}, false, false}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "dir/link", ftype: tar.TypeSymlink, linkname: "../badlink"}, false, false},
			{tarDentry{path: "dir/verybadlink", ftype: tar.TypeSymlink, linkname: "../../../../../../../../../../../../etc/shadow"}, false, false},
			{tarDentry{path: "dir/verybadlink2", ftype: tar.TypeSymlink, linkname: "/../../../../../../../../../../../../etc/shadow"}, false, false},
		}},
		{"MultiLevel", []layeredTarDentry{
			{tarDentry{path: "level1_file", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "level1_link", ftype: tar.TypeSymlink, linkname: ".."}, false, false},
			{tarDentry{path: "level1a", ftype: tar.TypeDir}, true, true},
			{tarDentry{path: "level1a/level2_file", ftype: tar.TypeRegA}, false, false}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "level1a/level2_link", ftype: tar.TypeSymlink, linkname: "../../../"}, true, true},
			{tarDentry{path: "level1a/level2a", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2a/level3_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2a/level3_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2b", ftype: tar.TypeDir}, true, true},
			{tarDentry{path: "level1a/level2b/level3_fileA", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "level1a/level2b/level3_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2b/level3", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3/level4", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3/level4", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3_fileA", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "level1b", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1b/level2_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1b/level2_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1b/level2", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1b/level2/level3_file", ftype: tar.TypeReg}, false, false},
		}},
		{"MultiLevelNoUpper", []layeredTarDentry{
			{tarDentry{path: "level1_file", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1_link", ftype: tar.TypeSymlink, linkname: ".."}, false, false},
			{tarDentry{path: "level1a", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2_file", ftype: tar.TypeRegA}, false, false}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "level1a/level2_link", ftype: tar.TypeSymlink, linkname: "../../../"}, false, false},
			{tarDentry{path: "level1a/level2a", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2a/level3_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2a/level3_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2b", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2b/level3_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1a/level2b/level3", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3/level4", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3/level4", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1a/level2b/level3_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1b", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1b/level2_fileA", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1b/level2_fileB", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "level1b/level2", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "level1b/level2/level3_file", ftype: tar.TypeReg}, false, false},
		}},
		{"MissingUpperAncestor", []layeredTarDentry{
			// Even if the parent directories are not listed as being in an
			// upper, they need to still exist for the subpath to exist.
			{tarDentry{path: "some", ftype: tar.TypeDir}, false, true},
			{tarDentry{path: "some/dir", ftype: tar.TypeDir}, false, true},
			{tarDentry{path: "some/dir/somewhere", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "another", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "another/dir", ftype: tar.TypeDir}, false, false},
			{tarDentry{path: "another/dir/somewhere", ftype: tar.TypeReg}, false, false},
		}},
		{"UpperWhiteout", []layeredTarDentry{
			{tarDentry{path: whPrefix + "fileB", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "fileA", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "fileB", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "fileC", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: whPrefix + "fileA", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: whPrefix + "fileC", ftype: tar.TypeReg}, true, true},
		}},
		// XXX: What umoci should do here is not really defined by the
		//      spec. In particular, whether you need a whiteout for every
		//      sub-path or just the path itself is not well-defined. This
		//      code assumes that you *do not*.
		{"UpperDirWhiteout", []layeredTarDentry{
			{tarDentry{path: whPrefix + "dir2", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "file", ftype: tar.TypeReg}, false, false},
			{tarDentry{path: "dir1", ftype: tar.TypeDir}, true, true},
			{tarDentry{path: "dir1/file", ftype: tar.TypeRegA}, true, true}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "dir1/link", ftype: tar.TypeSymlink, linkname: "../badlink"}, false, false},
			{tarDentry{path: "dir1/verybadlink", ftype: tar.TypeSymlink, linkname: "../../../../../../../../../../../../etc/shadow"}, true, true},
			{tarDentry{path: "dir1/verybadlink2", ftype: tar.TypeSymlink, linkname: "/../../../../../../../../../../../../etc/shadow"}, false, false},
			{tarDentry{path: whPrefix + "dir1", ftype: tar.TypeReg}, true, true},
			{tarDentry{path: "dir2", ftype: tar.TypeDir}, true, true},
			{tarDentry{path: "dir2/file", ftype: tar.TypeRegA}, true, true}, //nolint:staticcheck // SA1019: TypeRegA is deprecated but for compatibility we need to support it
			{tarDentry{path: "dir2/link", ftype: tar.TypeSymlink, linkname: "../badlink"}, false, false},
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			unpackOptions := UnpackOptions{
				OnDiskFormat: DirRootfs{
					MapOptions: MapOptions{
						Rootless: os.Geteuid() != 0,
					},
				},
			}

			dir := t.TempDir()

			// We do all whiteouts in a subdirectory.
			whiteoutDir := "test-subdir"
			whiteoutRoot := filepath.Join(dir, whiteoutDir)
			err := os.MkdirAll(whiteoutRoot, 0o755)
			require.NoError(t, err, "mkdir whiteout root")

			// Track if we have upper entries.
			numUpper := 0

			// First we apply the non-upper files in a new TarExtractor.
			te := NewTarExtractor(&unpackOptions)
			for _, de := range test.dentries {
				// Skip upper.
				if de.upper {
					numUpper++
					continue
				}
				hdr, rdr := tarFromDentry(de.tarDentry)
				hdr.Name = filepath.Join(whiteoutDir, hdr.Name)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s lower", hdr.Name)
			}

			// Now we apply the upper files in another TarExtractor.
			te = NewTarExtractor(&unpackOptions)
			for _, de := range test.dentries {
				// Skip non-upper.
				if !de.upper {
					continue
				}
				hdr, rdr := tarFromDentry(de.tarDentry)
				hdr.Name = filepath.Join(whiteoutDir, hdr.Name)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s upper", hdr.Name)
			}

			// And now apply a whiteout for the whiteoutRoot.
			whHdr := &tar.Header{
				Name:     filepath.Join(whiteoutDir, whOpaque),
				Typeflag: tar.TypeReg,
			}
			err = te.UnpackEntry(dir, whHdr, nil)
			require.NoErrorf(t, err, "UnpackEntry %s whiteout", whiteoutRoot)

			// Now we double-check it worked. If the file was in "upper" it
			// should have survived. If it was in lower it shouldn't. We don't
			// bother checking the contents here.
			for _, de := range test.dentries {
				// If there's an explicit whiteout in the headers we ignore it
				// here, since it won't be on the filesystem.
				if strings.HasPrefix(filepath.Base(de.path), whPrefix) {
					t.Logf("ignoring whiteout entry %q during post-check", de.path)
					continue
				}

				fullPath := filepath.Join(whiteoutRoot, de.path)
				_, err := te.fsEval.Lstat(fullPath)
				if de.shouldSurvive {
					assert.NoError(t, err, "upper layer file should have survived") //nolint:testifylint // assert.*Error* makes more sense
				} else {
					assert.ErrorIs(t, err, os.ErrNotExist, "lower layer file should have been removed") //nolint:testifylint // assert.*Error* makes more sense
				}
			}

			// Make sure the whiteoutRoot still exists.
			fi, err := te.fsEval.Lstat(whiteoutRoot)
			require.NoError(t, err, "whiteout root should still exist")
			assert.True(t, fi.IsDir(), "whiteout root should still be a directory")

			// Check that the directory is empty if there's no uppers.
			if numUpper == 0 {
				fd, err := os.Open(whiteoutRoot)
				require.NoError(t, err, "open whiteout root")

				names, err := fd.Readdirnames(-1)
				require.NoError(t, err, "readdir whiteout root")

				assert.Empty(t, names, "whiteout root should be empty if no uppers")
			}
		})
	}
}

// TestUnpackHardlink makes sure that hardlinks are correctly unpacked in all
// cases. In particular when it comes to hardlinks to symlinks.
func TestUnpackHardlink(t *testing.T) {
	// Create the files we're going to play with.
	dir := t.TempDir()

	var (
		hdr *tar.Header
		err error

		ctrValue  = []byte("some content we won't check")
		regFile   = "regular"
		symFile   = "link"
		hardFileA = "hard link"
		hardFileB = "hard link to symlink"
	)

	te := NewTarExtractor(nil)

	// Regular file.
	hdr = &tar.Header{
		Name:       regFile,
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		Mode:       0o644,
		Size:       int64(len(ctrValue)),
		Typeflag:   tar.TypeReg,
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}
	err = te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue))
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	// Hardlink to regFile.
	hdr = &tar.Header{
		Name:     hardFileA,
		Typeflag: tar.TypeLink,
		Linkname: filepath.Join("/", regFile),
		// These should **not** be applied.
		Uid: os.Getuid() + 1337,
		Gid: os.Getgid() + 2020,
	}
	err = te.UnpackEntry(dir, hdr, nil)
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	// Symlink to regFile.
	hdr = &tar.Header{
		Name:     symFile,
		Uid:      os.Getuid(),
		Gid:      os.Getgid(),
		Typeflag: tar.TypeSymlink,
		Linkname: filepath.Join("../../../", regFile),
	}
	err = te.UnpackEntry(dir, hdr, nil)
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	// Hardlink to symlink.
	hdr = &tar.Header{
		Name:     hardFileB,
		Typeflag: tar.TypeLink,
		Linkname: filepath.Join("../../../", symFile),
		// These should **really not** be applied.
		Uid: os.Getuid() + 1337,
		Gid: os.Getgid() + 2020,
	}
	err = te.UnpackEntry(dir, hdr, nil)
	require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

	// Make sure that the contents are as expected.
	ctrValueGot, err := os.ReadFile(filepath.Join(dir, regFile))
	require.NoError(t, err, "read regular file contents")
	assert.Equal(t, ctrValue, ctrValueGot, "regular file contents should match tar entry")

	// Now we have to check the inode numbers.
	var regFi, symFi, hardAFi, hardBFi unix.Stat_t

	err = unix.Lstat(filepath.Join(dir, regFile), &regFi)
	require.NoError(t, err, "lstat regular file")
	err = unix.Lstat(filepath.Join(dir, symFile), &symFi)
	require.NoError(t, err, "lstat symlink")
	err = unix.Lstat(filepath.Join(dir, hardFileA), &hardAFi)
	require.NoError(t, err, "lstat hardlink to regular file")
	err = unix.Lstat(filepath.Join(dir, hardFileB), &hardBFi)
	require.NoError(t, err, "lstat hardlink to symlink")

	// Check inode numbers of regular hardlink.
	assert.NotEqual(t, regFi.Ino, symFi.Ino, "regular and symlink should have different inode numbers")
	assert.Equal(t, regFi.Ino, hardAFi.Ino, "hardlink should have the same inode number as the original file")

	// Check inode numbers of hardlink-to-symlink.
	assert.NotEqual(t, regFi.Ino, hardBFi.Ino, "hardlink to symlink should not have the same inode number as the original file")
	assert.NotEqual(t, hardAFi.Ino, hardBFi.Ino, "hardlink to symlink should not have the same inode number as regular hardlink")
	assert.Equal(t, symFi.Ino, hardBFi.Ino, "hardlink to symlink should have same inode number as the symlink")

	// Double-check readlink.
	linknameA, err := os.Readlink(filepath.Join(dir, symFile))
	require.NoError(t, err, "readlink symlink")
	linknameB, err := os.Readlink(filepath.Join(dir, hardFileB))
	require.NoError(t, err, "readlink symlink to hardlink")
	assert.Equal(t, linknameA, linknameB, "hardlink to symlink should have same readlink data")

	// Make sure that uid and gid don't apply to hardlinks.
	assert.EqualValues(t, os.Getuid(), regFi.Uid, "regular file uid should not be changed by hardlink unpack")
	assert.EqualValues(t, os.Getgid(), regFi.Gid, "regular file gid should not be changed by hardlink unpack")
	assert.EqualValues(t, os.Getuid(), symFi.Uid, "symlink uid should not be changed by hardlink unpack")
	assert.EqualValues(t, os.Getgid(), symFi.Gid, "symlink gid should not be changed by hardlink unpack")
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
		{
			"IdentityRoot",
			rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: 0, ContainerID: 0, Size: 100},
		},
		{
			"MapSelfToRoot",
			rspec.LinuxIDMapping{HostID: uint32(os.Getuid()), ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: uint32(os.Getgid()), ContainerID: 0, Size: 100},
		},
		{
			"MapOtherToRoot",
			rspec.LinuxIDMapping{HostID: uint32(os.Getuid() + 100), ContainerID: 0, Size: 100},
			rspec.LinuxIDMapping{HostID: uint32(os.Getgid() + 200), ContainerID: 0, Size: 100},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Create the files we're going to play with.
			dir := t.TempDir()

			var (
				hdrUid, hdrGid int //nolint:revive // Uid/Gid preferred
				hdr            *tar.Header
				fi             unix.Stat_t
				err            error

				ctrValue = []byte("some content we won't check")
				regFile  = "regular"
				symFile  = "link"
				regDir   = " a directory"
				symDir   = "link-dir"
			)

			te := NewTarExtractor(&UnpackOptions{
				OnDiskFormat: DirRootfs{
					MapOptions: MapOptions{
						UIDMappings: []rspec.LinuxIDMapping{test.uidMap},
						GIDMappings: []rspec.LinuxIDMapping{test.gidMap},
					},
				},
			})

			// Regular file.
			hdrUid, hdrGid = 0, 0
			hdr = &tar.Header{
				Name:       regFile,
				Uid:        hdrUid,
				Gid:        hdrGid,
				Mode:       0o644,
				Size:       int64(len(ctrValue)),
				Typeflag:   tar.TypeReg,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			err = te.UnpackEntry(dir, hdr, bytes.NewBuffer(ctrValue))
			require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

			err = unix.Lstat(filepath.Join(dir, hdr.Name), &fi)
			require.NoErrorf(t, err, "lstat %s", hdr.Name)
			assert.EqualValuesf(t, int(test.uidMap.HostID)+hdrUid, fi.Uid, "file %s uid mapping", hdr.Name)
			assert.EqualValuesf(t, int(test.gidMap.HostID)+hdrGid, fi.Gid, "file %s gid mapping", hdr.Name)

			// Regular directory.
			hdrUid, hdrGid = 13, 42
			hdr = &tar.Header{
				Name:       regDir,
				Uid:        hdrUid,
				Gid:        hdrGid,
				Mode:       0o755,
				Typeflag:   tar.TypeDir,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			err = te.UnpackEntry(dir, hdr, nil)
			require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

			err = unix.Lstat(filepath.Join(dir, hdr.Name), &fi)
			require.NoErrorf(t, err, "lstat %s", hdr.Name)
			assert.EqualValuesf(t, int(test.uidMap.HostID)+hdrUid, fi.Uid, "file %s uid mapping", hdr.Name)
			assert.EqualValuesf(t, int(test.gidMap.HostID)+hdrGid, fi.Gid, "file %s gid mapping", hdr.Name)

			// Symlink to file.
			hdrUid, hdrGid = 23, 22
			hdr = &tar.Header{
				Name:       symFile,
				Uid:        hdrUid,
				Gid:        hdrGid,
				Typeflag:   tar.TypeSymlink,
				Linkname:   regFile,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			err = te.UnpackEntry(dir, hdr, nil)
			require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

			err = unix.Lstat(filepath.Join(dir, hdr.Name), &fi)
			require.NoErrorf(t, err, "lstat %s", hdr.Name)
			assert.EqualValuesf(t, int(test.uidMap.HostID)+hdrUid, fi.Uid, "file %s uid mapping", hdr.Name)
			assert.EqualValuesf(t, int(test.gidMap.HostID)+hdrGid, fi.Gid, "file %s gid mapping", hdr.Name)

			// Symlink to directory.
			hdrUid, hdrGid = 99, 88
			hdr = &tar.Header{
				Name:       symDir,
				Uid:        hdrUid,
				Gid:        hdrGid,
				Typeflag:   tar.TypeSymlink,
				Linkname:   regDir,
				ModTime:    time.Now(),
				AccessTime: time.Now(),
				ChangeTime: time.Now(),
			}
			err = te.UnpackEntry(dir, hdr, nil)
			require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

			err = unix.Lstat(filepath.Join(dir, hdr.Name), &fi)
			require.NoErrorf(t, err, "lstat %s", hdr.Name)
			assert.EqualValuesf(t, int(test.uidMap.HostID)+hdrUid, fi.Uid, "file %s uid mapping", hdr.Name)
			assert.EqualValuesf(t, int(test.gidMap.HostID)+hdrGid, fi.Gid, "file %s gid mapping", hdr.Name)
		})
	}
}

func TestIsDirlink(t *testing.T) {
	dir := t.TempDir()

	err := os.Mkdir(filepath.Join(dir, "test_dir"), 0o755)
	require.NoError(t, err)

	file, err := os.Create(filepath.Join(dir, "test_file"))
	require.NoError(t, err)
	_ = file.Close()

	err = os.Symlink("test_dir", filepath.Join(dir, "link"))
	require.NoError(t, err)

	te := NewTarExtractor(nil)
	// Basic symlink usage.
	dirlink, err := te.isDirlink(dir, filepath.Join(dir, "link"))
	require.NoError(t, err, "isDirlink symlink to directory")
	assert.True(t, dirlink, "symlink to directory is a dirlink")

	// "Read" a non-existent link.
	_, err = te.isDirlink(dir, filepath.Join(dir, "doesnt-exist"))
	assert.Error(t, err, "isDirlink non-existent path") //nolint:testifylint // assert.*Error* makes more sense

	// "Read" a directory.
	_, err = te.isDirlink(dir, filepath.Join(dir, "test_dir"))
	assert.Error(t, err, "isDirlink directory") //nolint:testifylint // assert.*Error* makes more sense

	// "Read" a file.
	_, err = te.isDirlink(dir, filepath.Join(dir, "test_file"))
	assert.Error(t, err, "isDirlink file") //nolint:testifylint // assert.*Error* makes more sense

	// Break the symlink.
	err = os.Remove(filepath.Join(dir, "test_dir"))
	require.NoError(t, err, "delete symlink target")

	dirlink, err = te.isDirlink(dir, filepath.Join(dir, "link"))
	require.NoError(t, err, "isDirlink broken symlink to directory")
	assert.False(t, dirlink, "broken symlink to directory is not a dirlink")

	// Point the symlink to a file.
	err = os.Remove(filepath.Join(dir, "link"))
	require.NoError(t, err)
	err = os.Symlink("test_file", filepath.Join(dir, "link"))
	require.NoError(t, err)

	dirlink, err = te.isDirlink(dir, filepath.Join(dir, "link"))
	require.NoError(t, err, "isDirlink symlink to file")
	assert.False(t, dirlink, "symlink to file is not a dirlink")
}
