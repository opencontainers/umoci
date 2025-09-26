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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/pkg/fseval"
)

// tarGenerator is a helper for generating layer diff tars. It should be noted
// that when using tarGenerator.Add{Path,Whiteout} it is recommended to do it
// in lexicographic order.
type tarGenerator struct {
	tw *tar.Writer

	// onDiskFmt is the on-disk format that was used for the already-extracted
	// directory we are operating on. [OverlayfsRootfs] will cause
	// overlayfs-style whiteouts to be converted to OCI-style whiteouts.
	onDiskFmt OnDiskFormat

	// Hardlink mapping.
	inodes map[uint64]string

	// fsEval is an fseval.FsEval used for extraction.
	fsEval fseval.FsEval

	// sourceDateEpoch, if set, clamps file timestamps in the generated tar
	// to not be newer than this time (follows tar --clamp-mtime behavior).
	sourceDateEpoch *time.Time

	// XXX: Should we add a safety check to make sure we don't generate two of
	//      the same path in a tar archive? This is not permitted by the spec.
}

// newTarGenerator creates a new tarGenerator using the provided writer as the
// output writer.
func newTarGenerator(w io.Writer, opt *RepackOptions) *tarGenerator {
	opt = opt.fill()

	fsEval := fseval.Default
	if opt.MapOptions().Rootless {
		fsEval = fseval.Rootless
	}

	return &tarGenerator{
		tw:              tar.NewWriter(w),
		onDiskFmt:       opt.OnDiskFormat,
		inodes:          map[uint64]string{},
		fsEval:          fsEval,
		sourceDateEpoch: opt.SourceDateEpoch,
	}
}

// normalise converts the provided pathname to a POSIX-compliant pathname. It also will provide an error if a path looks unsafe.
func normalise(rawPath string, isDir bool) (string, error) {
	// Clean up the path.
	path := CleanPath(rawPath)

	// Nothing to do.
	if path == "." {
		return ".", nil
	}

	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, "/")
	}

	// Check that the path is "safe", meaning that it doesn't resolve outside
	// of the tar archive. While this might seem paranoid, it is a legitimate
	// concern.
	if "/"+path != filepath.Join("/", path) {
		return "", fmt.Errorf("escape warning: generated path is outside tar root: %s", rawPath)
	}

	// With some other tar formats, you needed to have a '/' at the end of a
	// pathname in order to state that it is a directory. While this is no
	// longer necessary, some older tooling may assume that.
	if isDir {
		path += "/"
	}

	return path, nil
}

// AddFile adds a file from the filesystem to the tar archive. It copies all of
// the relevant stat information about the file, and also attempts to track
// hardlinks. This should be functionally equivalent to adding entries with GNU
// tar.
func (tg *tarGenerator) AddFile(name, path string) (Err error) {
	fi, err := tg.fsEval.Lstat(path)
	if err != nil {
		return fmt.Errorf("add file lstat: %w", err)
	}

	linkname := ""
	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkname, err = tg.fsEval.Readlink(path); err != nil {
			return fmt.Errorf("add file readlink: %w", err)
		}
	}

	hdr, err := tar.FileInfoHeader(fi, linkname)
	if err != nil {
		return fmt.Errorf("convert fi to hdr: %w", err)
	}

	// Apply SOURCE_DATE_EPOCH timestamp clamping if set. Note that we only
	// clamp timestamps that are newer than SOURCE_DATE_EPOCH (a-la GNU tar's
	// --clamp-mtime behaviour).
	if tg.sourceDateEpoch != nil {
		sourceDateEpoch := *tg.sourceDateEpoch
		if !hdr.ModTime.IsZero() && hdr.ModTime.After(sourceDateEpoch) {
			hdr.ModTime = sourceDateEpoch
		}
		// NOTE: atime and ctime are currently no-ops because we use the
		// default archive/tar.Writer settings.
		if !hdr.AccessTime.IsZero() && hdr.AccessTime.After(sourceDateEpoch) {
			hdr.AccessTime = sourceDateEpoch
		}
		if !hdr.ChangeTime.IsZero() && hdr.ChangeTime.After(sourceDateEpoch) {
			hdr.ChangeTime = sourceDateEpoch
		}
	}
	hdr.Xattrs = map[string]string{} //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
	// Usually incorrect for containers and was added in Go 1.10 causing
	// changes to our output on a compiler bump...
	hdr.Uname = ""
	hdr.Gname = ""
	// archive/tar will round timestamps to their nearest second, while
	// tar_time (for our mtree validation) will truncate timestamps. For
	// consistency, explicitly truncate them.
	hdr.ModTime = hdr.ModTime.Truncate(time.Second)
	hdr.AccessTime = hdr.AccessTime.Truncate(time.Second)
	hdr.ChangeTime = hdr.ChangeTime.Truncate(time.Second)

	name, err = normalise(name, fi.IsDir())
	if err != nil {
		return fmt.Errorf("normalise path: %w", err)
	}
	hdr.Name = name

	// Make sure that we don't include any files with the name ".wh.". This
	// will almost certainly confuse some users (unfortunately) but there's
	// nothing we can do to store such files on-disk.
	if strings.HasPrefix(filepath.Base(name), whPrefix) {
		return fmt.Errorf("invalid path has whiteout prefix %q: %s", whPrefix, name)
	}

	// FIXME: Do we need to ensure that the parent paths have all been added to
	//        the archive? I haven't found any tar specification that makes
	//        this mandatory, but I have a feeling that some people might rely
	//        on it. The issue with implementing it is that we'd have to get
	//        the FileInfo about the directory from somewhere (and we don't
	//        want to waste space by adding an entry that will be overwritten
	//        later).

	// Different systems have different special things they need to set within
	// a tar header. For example, device numbers are quite important to be set
	// by us.
	statx, err := tg.fsEval.Lstatx(path)
	if err != nil {
		return fmt.Errorf("lstatx %q: %w", path, err)
	}
	updateHeader(hdr, statx)

	// Set up xattrs externally to updateHeader because the function signature
	// would look really dumb otherwise.
	// XXX: This should probably be moved to a function in tar_unix.go.
	xattrs, err := tg.fsEval.Llistxattr(path)
	if err != nil {
		if !errors.Is(err, unix.EOPNOTSUPP) {
			return fmt.Errorf("get xattr list: %w", err)
		}
		xattrs = []string{}
	}
	for _, xattr := range xattrs {
		// Some xattrs need to be skipped for sanity reasons, such as
		// security.selinux, because they are very much host-specific and
		// carrying them to other hosts would be a really bad idea. Other
		// xattrs need to be remapped (such as escaped trusted.overlay.* xattrs
		// when in overlayfs mode) to have correct values.
		mappedName := xattr
		if filter, isSpecial := getXattrFilter(xattr); isSpecial {
			if newName := filter.ToTar(tg.onDiskFmt, xattr); newName == "" {
				log.Debugf("xattr{%s} skipping the inclusion of xattr %q in generated tar archive", hdr.Name, xattr)
				continue
			} else if newName != xattr {
				mappedName = newName
				log.Debugf("xattr{%s} remapping xattr %q to %q in generated tar archive", hdr.Name, xattr, mappedName)
			}
		}
		// TODO: We should translate all v3 capabilities into root-owned
		//       capabilities here. But we don't have Go code for that yet
		//       (we'd need to use libcap to parse it).
		value, err := tg.fsEval.Lgetxattr(path, xattr)
		if err != nil {
			// Ignore xattrs we were unable to read or if the filesystem is
			// refusing to provide information about them. Note that rather
			// than getting a permission error when reading a trusted.* xattr
			// as an unprivileged user, you actually get ENODATA.
			log.Debugf("xattr{%s} failure reading xattr %q: %v", hdr.Name, xattr, err)
			// TODO: Should we use errors.As?
			if !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENODATA) {
				return fmt.Errorf("get xattr %q: %w", xattr, err)
			}
		}
		// https://golang.org/issues/20698 -- We don't just error out here
		// because it's not _really_ a fatal error. Currently it's unclear
		// whether the stdlib will correctly handle reading or disable writing
		// of these PAX headers so we have to track this ourselves.
		if len(value) <= 0 {
			log.Warnf("xattr{%s} ignoring empty-valued xattr %q: disallowed by PAX standard", hdr.Name, xattr)
			continue
		}
		// Note that Go strings can actually be arbitrary byte sequences, so
		// this conversion (while it might look a bit wrong) is actually fine.
		hdr.Xattrs[mappedName] = string(value) //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
	}

	// Not all systems have the concept of an inode, but I'm not in the mood to
	// handle this in a way that makes anything other than GNU/Linux happy
	// right now. Handle hardlinks.
	if oldpath, ok := tg.inodes[statx.Ino]; ok {
		// We just hit a hardlink, so we just have to change the header.
		hdr.Typeflag = tar.TypeLink
		hdr.Linkname = oldpath
		hdr.Size = 0
	} else {
		tg.inodes[statx.Ino] = name
	}

	// Apply any header mappings.
	if err := mapHeader(hdr, tg.onDiskFmt.Map()); err != nil {
		return fmt.Errorf("map header: %w", err)
	}
	if err := tg.tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("write header: %w", err)
	}

	// Write the contents of regular files.
	if hdr.Typeflag == tar.TypeReg {
		fh, err := tg.fsEval.Open(path)
		if err != nil {
			return fmt.Errorf("open file: %w", err)
		}
		defer funchelpers.VerifyClose(&Err, fh)

		n, err := system.Copy(tg.tw, fh)
		if err != nil {
			return fmt.Errorf("copy to layer: %w", err)
		}
		if n != hdr.Size {
			return fmt.Errorf("copy to layer: %w", io.ErrShortWrite)
		}
	}

	return nil
}

// whPrefix is the whiteout prefix, which is used to signify "special" files in
// an OCI image layer archive. An expanded filesystem image cannot contain
// files that have a basename starting with this prefix.
const whPrefix = ".wh."

// whOpaque is the *full* basename of a special file which indicates that all
// siblings in a directory are to be dropped in the "lower" layer.
const whOpaque = whPrefix + whPrefix + ".opq"

// addWhiteout adds a whiteout file for the given name inside the tar archive.
// It's not recommended to add a file with AddFile and then white it out. If
// you specify opaque, then the whiteout created is an opaque whiteout *for the
// directory path* given.
func (tg *tarGenerator) addWhiteout(name string, opaque bool) error {
	name, err := normalise(name, false)
	if err != nil {
		return fmt.Errorf("normalise path: %w", err)
	}

	// Disallow having a whiteout of a whiteout, purely for our own sanity.
	dir, file := filepath.Split(name)
	if strings.HasPrefix(file, whPrefix) {
		return fmt.Errorf("invalid path has whiteout prefix %q: %s", whPrefix, name)
	}

	// Figure out the whiteout name.
	whiteout := filepath.Join(dir, whPrefix+file)
	if opaque {
		whiteout = filepath.Join(name, whOpaque)
	}

	// Add a dummy header for the whiteout file.
	if err := tg.tw.WriteHeader(&tar.Header{Name: whiteout, Size: 0}); err != nil {
		return fmt.Errorf("write whiteout header: %w", err)
	}
	return nil
}

// AddWhiteout creates a whiteout for the provided path.
func (tg *tarGenerator) AddWhiteout(name string) error {
	return tg.addWhiteout(name, false)
}

// AddOpaqueWhiteout creates a whiteout for the provided path.
func (tg *tarGenerator) AddOpaqueWhiteout(name string) error {
	return tg.addWhiteout(name, true)
}
