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
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	securejoin "github.com/cyphar/filepath-securejoin"
	"github.com/moby/sys/userns"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/system"
)

// inUserNamespace is a cached return value of userns.RunningInUserNS(). We
// compute this once globally rather than for each unpack. It won't change (we
// would hope) after we check it the first time.
var inUserNamespace = userns.RunningInUserNS()

// TarExtractor represents a tar file to be extracted.
type TarExtractor struct {
	// mapOptions is the set of mapping options to use when extracting
	// filesystem layers.
	mapOptions MapOptions

	// partialRootless indicates whether "partial rootless" tricks should be
	// applied in our extraction. Rootless and userns execution have some
	// similar tricks necessary, but not all rootless tricks should be applied
	// when running in a userns -- hence the term "partial rootless" tricks.
	partialRootless bool

	// fsEval is an fseval.FsEval used for extraction.
	fsEval fseval.FsEval

	// upperPaths are paths that have either been extracted in the execution of
	// this TarExtractor or are ancestors of paths extracted. The purpose of
	// having this stored in-memory is to be able to handle opaque whiteouts as
	// well as some other possible ordering issues with malformed archives (the
	// downside of this approach is that it takes up memory -- we could switch
	// to a trie if necessary). These paths are relative to the tar root but
	// are fully symlink-expanded so no need to worry about that line noise.
	upperPaths map[string]struct{}

	// enotsupWarned is a flag set when we encounter the first ENOTSUP error
	// dealing with xattrs. This is used to ensure extraction to a destination
	// file system that does not support xattrs raises a single warning, rather
	// than a warning for every file, which can amount to 1000s of messages that
	// scroll a terminal, and may obscure other more important warnings.
	enotsupWarned bool

	// keepDirlinks is the corresponding flag from the UnpackOptions
	// supplied when this TarExtractor was constructed.
	keepDirlinks bool

	// whiteoutMode indicates how this TarExtractor will handle whiteouts.
	whiteoutMode WhiteoutMode
}

// NewTarExtractor creates a new TarExtractor.
func NewTarExtractor(opt UnpackOptions) *TarExtractor {
	fsEval := fseval.Default
	if opt.MapOptions.Rootless {
		fsEval = fseval.Rootless
	}

	return &TarExtractor{
		mapOptions:      opt.MapOptions,
		partialRootless: opt.MapOptions.Rootless || inUserNamespace,
		fsEval:          fsEval,
		upperPaths:      make(map[string]struct{}),
		enotsupWarned:   false,
		keepDirlinks:    opt.KeepDirlinks,
		whiteoutMode:    opt.WhiteoutMode,
	}
}

// restoreMetadata applies the state described in tar.Header to the filesystem
// at the given path. No sanity checking is done of the tar.Header's pathname
// or other information. In addition, no mapping is done of the header.
func (te *TarExtractor) restoreMetadata(path string, hdr *tar.Header) error {
	// Some of the tar.Header fields don't match the OS API.
	fi := hdr.FileInfo()

	// Get the _actual_ file info to figure out if the path is a symlink.
	isSymlink := hdr.Typeflag == tar.TypeSymlink
	if realFi, err := te.fsEval.Lstat(path); err == nil {
		isSymlink = realFi.Mode()&os.ModeSymlink == os.ModeSymlink
	}

	// Apply the owner. If we are rootless then "user.rootlesscontainers" has
	// already been set up by unmapHeader, so nothing to do here.
	if !te.mapOptions.Rootless {
		// NOTE: This is not done through fsEval.
		if err := os.Lchown(path, hdr.Uid, hdr.Gid); err != nil {
			return fmt.Errorf("restore chown metadata: %s: %w", path, err)
		}
	}

	// We cannot apply hdr.Mode to symlinks, because symlinks don't have a mode
	// of their own (they're special in that way). We have to apply this after
	// we've applied the owner because setuid bits are cleared when changing
	// owner (in rootless we don't care because we're always the owner).
	if !isSymlink {
		if err := te.fsEval.Chmod(path, fi.Mode()); err != nil {
			return fmt.Errorf("restore chmod metadata: %s: %w", path, err)
		}
	}

	// Apply access and modified time. Note that some archives won't fill the
	// atime and mtime fields, so we have to set them to a more sane value.
	// Otherwise Linux will start screaming at us, and nobody wants that.
	mtime := hdr.ModTime
	if mtime.IsZero() {
		// XXX: Should we instead default to atime if it's non-zero?
		mtime = time.Now()
	}
	atime := hdr.AccessTime
	if atime.IsZero() {
		// Default to the mtime.
		atime = mtime
	}

	// Apply xattrs. In order to make sure that we *only* have the xattr set we
	// want, we first clear the set of xattrs from the file then apply the ones
	// set in the tar.Header.
	err := te.fsEval.Lclearxattrs(path, ignoreXattrs)
	if err != nil {
		if !errors.Is(err, unix.ENOTSUP) {
			return fmt.Errorf("clear xattr metadata: %s: %w", path, err)
		}
		if !te.enotsupWarned {
			log.Warnf("xattr{%s} ignoring ENOTSUP on clearxattrs", path)
			log.Warnf("xattr{%s} destination filesystem does not support xattrs, further warnings will be suppressed", path)
			te.enotsupWarned = true
		} else {
			log.Debugf("xattr{%s} ignoring ENOTSUP on clearxattrs", path)
		}
	}

	for name, value := range hdr.Xattrs {
		value := []byte(value)

		// Forbidden xattrs should never be touched.
		if _, skip := ignoreXattrs[name]; skip {
			// If the xattr is already set to the requested value, don't bail.
			// The reason for this logic is kinda convoluted, but effectively
			// because restoreMetadata is called with the *on-disk* metadata we
			// run the risk of things like "security.selinux" being included in
			// that metadata (and thus tripping the forbidden xattr error). By
			// only touching xattrs that have a different value we are somewhat
			// more efficient and we don't have to special case parent restore.
			// Of course this will only ever impact ignoreXattrs.
			if oldValue, err := te.fsEval.Lgetxattr(path, name); err == nil {
				if bytes.Equal(value, oldValue) {
					log.Debugf("restore xattr metadata: skipping already-set xattr %q: %s", name, hdr.Name)
					continue
				}
			}
			log.Warnf("xattr{%s} ignoring forbidden xattr: %q", hdr.Name, name)
			continue
		}
		if err := te.fsEval.Lsetxattr(path, name, value, 0); err != nil {
			// In rootless mode, some xattrs will fail (security.capability).
			// This is _fine_ as long as we're not running as root (in which
			// case we shouldn't be ignoring xattrs that we were told to set).
			//
			// TODO: We should translate all security.capability capabilities
			//       into v3 capabilities, which allow us to write them as
			//       unprivileged users (we also would need to translate them
			//       back when creating archives).
			if te.partialRootless && errors.Is(err, os.ErrPermission) {
				log.Warnf("rootless{%s} ignoring (usually) harmless EPERM on setxattr %q", hdr.Name, name)
				continue
			}
			// We cannot do much if we get an ENOTSUP -- this usually means
			// that extended attributes are simply unsupported by the
			// underlying filesystem (such as AUFS or NFS).
			if errors.Is(err, unix.ENOTSUP) {
				if !te.enotsupWarned {
					log.Warnf("xattr{%s} ignoring ENOTSUP on setxattr %q", hdr.Name, name)
					log.Warnf("xattr{%s} destination filesystem does not support xattrs, further warnings will be suppressed", path)
					te.enotsupWarned = true
				} else {
					log.Debugf("xattr{%s} ignoring ENOTSUP on clearxattrs", path)
				}
				continue
			}
			return fmt.Errorf("restore xattr metadata: %s: %w", path, err)
		}
	}

	if err := te.fsEval.Lutimes(path, atime, mtime); err != nil {
		return fmt.Errorf("restore lutimes metadata: %s: %w", path, err)
	}

	return nil
}

// applyMetadata applies the state described in tar.Header to the filesystem at
// the given path, using the state of the TarExtractor to remap information
// within the header. This should only be used with headers from a tar layer
// (not from the filesystem). No sanity checking is done of the tar.Header's
// pathname or other information.
func (te *TarExtractor) applyMetadata(path string, hdr *tar.Header) error {
	// Modify the header.
	if err := unmapHeader(hdr, te.mapOptions); err != nil {
		return fmt.Errorf("unmap header: %w", err)
	}

	// Restore it on the filesystme.
	return te.restoreMetadata(path, hdr)
}

// isDirlink returns whether the given path is a link to a directory (or a
// dirlink in rsync(1) parlance) which is used by --keep-dirlink to see whether
// we should extract through the link or clobber the link with a directory (in
// the case where we see a directory to extract and a symlink already exists
// there).
func (te *TarExtractor) isDirlink(root string, path string) (bool, error) {
	// Make sure it exists and is a symlink.
	if _, err := te.fsEval.Readlink(path); err != nil {
		return false, fmt.Errorf("read dirlink: %w", err)
	}

	// Technically a string.TrimPrefix would also work...
	unsafePath, err := filepath.Rel(root, path)
	if err != nil {
		return false, fmt.Errorf("get relative-to-root path: %w", err)
	}

	// It should be noted that SecureJoin will evaluate all symlinks in the
	// path, so we don't need to loop over it or anything like that. It'll just
	// be done for us (in UnpackEntry only the dirname(3) is evaluated but here
	// we evaluate the whole thing).
	targetPath, err := securejoin.SecureJoinVFS(root, unsafePath, te.fsEval)
	if err != nil {
		// We hit a symlink loop -- which is fine but that means that this
		// cannot be considered a dirlink.
		if errors.Is(err, unix.ELOOP) {
			return false, nil
		}
		return false, fmt.Errorf("sanitize old target: %w", err)
	}

	targetInfo, err := te.fsEval.Lstat(targetPath)
	if err != nil {
		// ENOENT or similar just means that it's a broken symlink, which
		// means we have to overwrite it (but it's an allowed case).
		if securejoin.IsNotExist(err) {
			err = nil
		}
		return false, err
	}

	return targetInfo.IsDir(), nil
}

func (te *TarExtractor) ociWhiteout(root string, dir string, file string) error {
	isOpaque := file == whOpaque
	file = strings.TrimPrefix(file, whPrefix)

	// We have to be quite careful here. While the most intuitive way of
	// handling whiteouts would be to just RemoveAll without prejudice, We
	// have to be careful here. If there is a whiteout entry for a file
	// *after* a normal entry (in the same layer) then the whiteout must
	// not remove the new entry. We handle this by keeping track of
	// whichpaths have been touched by this layer's extraction (these form
	// the "upperdir"). We also have to handle cases where a directory has
	// been marked for deletion, but a child has been extracted in this
	// layer.

	path := filepath.Join(dir, file)
	if isOpaque {
		path = dir
	}

	// If the root doesn't exist we've got nothing to do.
	// XXX: We currently cannot error out if a layer asks us to remove a
	//      non-existent path with this implementation (because we don't
	//      know if it was implicitly removed by another whiteout). In
	//      future we could add lowerPaths that would help track whether
	//      another whiteout caused the removal to "fail" or if the path
	//      was actually missing -- which would allow us to actually error
	//      out here if the layer is invalid).
	if _, err := te.fsEval.Lstat(path); err != nil {
		// Need to use securejoin.IsNotExist to handle ENOTDIR.
		if securejoin.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("check whiteout target: %w", err)
	}

	// Walk over the path to remove it. We remove a given path as soon as
	// it isn't present in upperPaths (which includes ancestors of paths
	// we've extracted so we only need to look up the one path). Otherwise
	// we iterate over any children and try again. The only difference
	// between opaque whiteouts and regular whiteouts is that we don't
	// delete the directory itself with opaque whiteouts.
	if err := te.fsEval.Walk(path, func(subpath string, info os.FileInfo, err error) error {
		// If we are passed an error, bail unless it's ENOENT.
		if err != nil {
			// If something was deleted outside of our knowledge it's not
			// the end of the world. In principle this shouldn't happen
			// though, so we log it for posterity.
			if errors.Is(err, os.ErrNotExist) {
				log.Debugf("whiteout removal hit already-deleted path: %s", subpath)
				err = filepath.SkipDir
			}
			return err
		}

		// Get the relative form of subpath to root to match
		// te.upperPaths.
		upperPath, err := filepath.Rel(root, subpath)
		if err != nil {
			return fmt.Errorf("find relative-to-root [should never happen]: %w", err)
		}

		// Remove the path only if it hasn't been touched.
		if _, ok := te.upperPaths[upperPath]; !ok {
			// Opaque whiteouts don't remove the directory itself, so skip
			// the top-level directory.
			if isOpaque && CleanPath(path) == CleanPath(subpath) {
				return nil
			}

			// Purge the path. We skip anything underneath (if it's a
			// directory) since we just purged it -- and we don't want to
			// hit ENOENT during iteration for no good reason.
			if err := te.fsEval.RemoveAll(subpath); err != nil {
				return fmt.Errorf("whiteout subpath: %w", err)
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf("whiteout remove: %w", err)
	}
	return nil
}

func (te *TarExtractor) overlayFSWhiteout(dir string, file string) error {
	isOpaque := file == whOpaque

	// if this is an opaque whiteout, whiteout the directory
	if isOpaque {
		if err := te.fsEval.Lsetxattr(dir, "trusted.overlay.opaque", []byte("y"), 0); err != nil {
			return fmt.Errorf("couldn't set overlayfs whiteout attr for %s: %w", dir, err)
		}
		return nil
	}

	// otherwise, white out the file itself.
	p := filepath.Join(dir, strings.TrimPrefix(file, whPrefix))
	if err := os.RemoveAll(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("couldn't create overlayfs whiteout for %s: %w", p, err)
	}

	if err := te.fsEval.Mknod(p, unix.S_IFCHR|0666, unix.Mkdev(0, 0)); err != nil {
		return fmt.Errorf("couldn't create overlayfs whiteout for %s: %w", p, err)
	}
	return nil
}

// UnpackEntry extracts the given tar.Header to the provided root, ensuring
// that the layer state is consistent with the layer state that produced the
// tar archive being iterated over. This does handle whiteouts, so a tar.Header
// that represents a whiteout will result in the path being removed.
func (te *TarExtractor) UnpackEntry(root string, hdr *tar.Header, r io.Reader) (Err error) {
	// Make the paths safe.
	hdr.Name = CleanPath(hdr.Name)
	root = filepath.Clean(root)

	log.WithFields(log.Fields{
		"root": root,
		"path": hdr.Name,
		"type": hdr.Typeflag,
	}).Debugf("unpacking entry")

	// Get directory and filename, but we have to safely get the directory
	// component of the path. SecureJoinVFS will evaluate the path itself,
	// which we don't want (we're clever enough to handle the actual path being
	// a symlink).
	unsafeDir, file := filepath.Split(hdr.Name)
	if filepath.Join("/", hdr.Name) == "/" {
		// If we got an entry for the root, then unsafeDir is the full path.
		unsafeDir, file = hdr.Name, "."
		// If we're being asked to change the root type, bail because they may
		// change it to a symlink which we could inadvertently follow.
		if hdr.Typeflag != tar.TypeDir {
			return errors.New("malicious tar entry -- refusing to change type of root directory")
		}
	}
	dir, err := securejoin.SecureJoinVFS(root, unsafeDir, te.fsEval)
	if err != nil {
		return fmt.Errorf("sanitise symlinks in root: %w", err)
	}
	path := filepath.Join(dir, file)

	// Before we do anything, get the state of dir. Because we might be adding
	// or removing files, our parent directory might be modified in the
	// process. As a result, we want to be able to restore the old state
	// (because we only apply state that we find in the archive we're iterating
	// over). We can safely ignore an error here, because a non-existent
	// directory will be fixed by later archive entries.
	if dirFi, err := te.fsEval.Lstat(dir); err == nil && path != dir {
		// FIXME: This is really stupid.
		// #nosec G104
		link, _ := te.fsEval.Readlink(dir)
		dirHdr, err := tar.FileInfoHeader(dirFi, link)
		if err != nil {
			return fmt.Errorf("convert dirFi to dirHdr: %w", err)
		}

		// More faking to trick restoreMetadata to actually restore the directory.
		dirHdr.Typeflag = tar.TypeDir
		dirHdr.Linkname = ""

		// os.Lstat doesn't get the list of xattrs by default. We need to fill
		// this explicitly. Note that while Go's "archive/tar" takes strings,
		// in Go strings can be arbitrary byte sequences so this doesn't
		// restrict the possible values.
		// TODO: Move this to a separate function so we can share it with
		//       tar_generate.go.
		xattrs, err := te.fsEval.Llistxattr(dir)
		if err != nil {
			if !errors.Is(err, unix.ENOTSUP) {
				return fmt.Errorf("get dirHdr.Xattrs: %w", err)
			}
			if !te.enotsupWarned {
				log.Warnf("xattr{%s} ignoring ENOTSUP on llistxattr", dir)
				log.Warnf("xattr{%s} destination filesystem does not support xattrs, further warnings will be suppressed", path)
				te.enotsupWarned = true
			} else {
				log.Debugf("xattr{%s} ignoring ENOTSUP on clearxattrs", path)
			}
		}
		if len(xattrs) > 0 {
			dirHdr.Xattrs = map[string]string{}
			for _, xattr := range xattrs {
				value, err := te.fsEval.Lgetxattr(dir, xattr)
				if err != nil {
					return fmt.Errorf("get xattr: %w", err)
				}
				dirHdr.Xattrs[xattr] = string(value)
			}
		}

		// Ensure that after everything we correctly re-apply the old metadata.
		// We don't map this header because we're restoring files that already
		// existed on the filesystem, not from a tar layer.
		defer func() {
			// Only overwrite the error if there wasn't one already.
			if err := te.restoreMetadata(dir, dirHdr); err != nil {
				if Err == nil {
					Err = fmt.Errorf("restore parent directory: %w", err)
				}
			}
		}()
	}

	// Currently the spec doesn't specify what the hdr.Typeflag of whiteout
	// files is meant to be. We specifically only produce regular files
	// ('\x00') but it could be possible that someone produces a different
	// Typeflag, expecting that the path is the only thing that matters in a
	// whiteout entry.
	if strings.HasPrefix(file, whPrefix) {
		switch te.whiteoutMode {
		case OCIStandardWhiteout:
			return te.ociWhiteout(root, dir, file)
		case OverlayFSWhiteout:
			return te.overlayFSWhiteout(dir, file)
		default:
			return fmt.Errorf("unknown whiteout mode %d", te.whiteoutMode)
		}
	}

	// Get information about the path. This has to be done after we've dealt
	// with whiteouts because it turns out that lstat(2) will return EPERM if
	// you try to stat a whiteout on AUFS.
	fi, err := te.fsEval.Lstat(path)
	if err != nil {
		// File doesn't exist, just switch fi to the file header.
		fi = hdr.FileInfo()
	}

	// Attempt to create the parent directory of the path we're unpacking.
	// We do a MkdirAll here because even though you need to have a tar entry
	// for every component of a new path, applyMetadata will correct any
	// inconsistencies.
	// FIXME: We have to make this consistent, since if the tar archive doesn't
	//        have entries for some of these components we won't be able to
	//        verify that we have consistent results during unpacking.
	if err := te.fsEval.MkdirAll(dir, 0777); err != nil {
		return fmt.Errorf("mkdir parent: %w", err)
	}

	isDirlink := false
	// We remove whatever existed at the old path to clobber it so that
	// creating a new path will not break. The only exception is if the path is
	// a directory in both the layer and the current filesystem, in which case
	// we don't delete it for obvious reasons. In all other cases we clobber.
	//
	// Note that this will cause hard-links in the "lower" layer to not be able
	// to point to "upper" layer inodes even if the extracted type is the same
	// as the old one, however it is not clear whether this is something a user
	// would expect anyway. In addition, this will incorrectly deal with a
	// TarLink that is present before the "upper" entry in the layer but the
	// "lower" file still exists (so the hard-link would point to the old
	// inode). It's not clear if such an archive is actually valid though.
	if !fi.IsDir() || hdr.Typeflag != tar.TypeDir {
		// If we are in --keep-dirlinks mode and the existing fs object is a
		// symlink to a directory (with the pending object is a directory), we
		// don't remove the symlink (and instead allow subsequent objects to be
		// just written through the symlink into the directory). This is a very
		// specific usecase where layers that were generated independently from
		// each other (on different base filesystems) end up with weird things
		// like /lib64 being a symlink only sometimes but you never want to
		// delete libraries (not just the ones that were under the "real"
		// directory).
		//
		// TODO: This code should also handle a pending symlink entry where the
		//       existing object is a directory. I'm not sure how we could
		//       disambiguate this from a symlink-to-a-file but I imagine that
		//       this is something that would also be useful in the same vein
		//       as --keep-dirlinks (which currently only prevents clobbering
		//       in the opposite case).
		if te.keepDirlinks &&
			fi.Mode()&os.ModeSymlink == os.ModeSymlink && hdr.Typeflag == tar.TypeDir {
			isDirlink, err = te.isDirlink(root, path)
			if err != nil {
				return fmt.Errorf("check is dirlink: %w", err)
			}
		}
		if !(isDirlink && te.keepDirlinks) {
			if err := te.fsEval.RemoveAll(path); err != nil {
				return fmt.Errorf("clobber old path: %w", err)
			}
		}
	}

	// Now create or otherwise modify the state of the path. Right now, either
	// the type of path matches hdr or the path doesn't exist. Note that we
	// don't care about umasks or the initial mode here, since applyMetadata
	// will fix all of that for us.
	switch hdr.Typeflag {
	// regular file
	case tar.TypeReg, tar.TypeRegA:
		// Create a new file, then just copy the data.
		fh, err := te.fsEval.Create(path)
		if err != nil {
			return fmt.Errorf("create regular: %w", err)
		}
		defer fh.Close()

		// We need to make sure that we copy all of the bytes.
		n, err := system.Copy(fh, r)
		if int64(n) != hdr.Size {
			if err != nil {
				err = fmt.Errorf("short write: %w", err)
			} else {
				err = io.ErrShortWrite
			}
		}
		if err != nil {
			return fmt.Errorf("unpack to regular file: %w", err)
		}

		// Force close here so that we don't affect the metadata.
		if err := fh.Close(); err != nil {
			return fmt.Errorf("close unpacked regular file: %w", err)
		}

	// directory
	case tar.TypeDir:
		if isDirlink {
			break
		}

		// Attempt to create the directory. We do a MkdirAll here because even
		// though you need to have a tar entry for every component of a new
		// path, applyMetadata will correct any inconsistencies.
		if err := te.fsEval.MkdirAll(path, 0777); err != nil {
			return fmt.Errorf("mkdirall: %w", err)
		}

	// hard link, symbolic link
	case tar.TypeLink, tar.TypeSymlink:
		linkname := hdr.Linkname

		// Hardlinks and symlinks act differently when it comes to the scoping.
		// In both cases, we have to just unlink and then re-link the given
		// path. But the function used and the argument are slightly different.
		var linkFn func(string, string) error
		switch hdr.Typeflag {
		case tar.TypeLink:
			linkFn = te.fsEval.Link
			// Because hardlinks are inode-based we need to scope the link to
			// the rootfs using SecureJoinVFS. As before, we need to be careful
			// that we don't resolve the last part of the link path (in case
			// the user actually wanted to hardlink to a symlink).
			unsafeLinkDir, linkFile := filepath.Split(CleanPath(linkname))
			linkDir, err := securejoin.SecureJoinVFS(root, unsafeLinkDir, te.fsEval)
			if err != nil {
				return fmt.Errorf("sanitise hardlink target in root: %w", err)
			}
			linkname = filepath.Join(linkDir, linkFile)
		case tar.TypeSymlink:
			linkFn = te.fsEval.Symlink
		}

		// Link the new one.
		if err := linkFn(linkname, path); err != nil {
			// FIXME: Currently this can break if tar hardlink entries occur
			//        before we hit the entry those hardlinks link to. I have a
			//        feeling that such archives are invalid, but the correct
			//        way of handling this is to delay link creation until the
			//        very end. Unfortunately this won't work with symlinks
			//        (which can link to directories).
			return fmt.Errorf("link: %w", err)
		}

	// character device node, block device node
	case tar.TypeChar, tar.TypeBlock:
		// In rootless mode we have no choice but to fake this, since mknod(2)
		// doesn't work as an unprivileged user here.
		//
		// TODO: We need to add the concept of a fake block device in
		//       "user.rootlesscontainers", because this workaround suffers
		//       from the obvious issue that if the file is touched (even the
		//       metadata) then it will be incorrectly copied into the layer.
		//       This would break distribution images fairly badly.
		if te.partialRootless {
			log.Warnf("rootless{%s} creating empty file in place of device %d:%d", hdr.Name, hdr.Devmajor, hdr.Devminor)
			fh, err := te.fsEval.Create(path)
			if err != nil {
				return fmt.Errorf("create rootless block: %w", err)
			}
			defer fh.Close()
			if err := fh.Chmod(0); err != nil {
				return fmt.Errorf("chmod 0 rootless block: %w", err)
			}
			goto out
		}

		// Otherwise the handling is the same as a FIFO.
		fallthrough
	// fifo node
	case tar.TypeFifo:
		// We have to remove and then create the device. In the FIFO case we
		// could choose not to do so, but we do it anyway just to be on the
		// safe side.

		mode := system.Tarmode(hdr.Typeflag)
		dev := unix.Mkdev(uint32(hdr.Devmajor), uint32(hdr.Devminor))

		// Create the node.
		if err := te.fsEval.Mknod(path, os.FileMode(int64(mode)|hdr.Mode), dev); err != nil {
			return fmt.Errorf("mknod: %w", err)
		}

	// We should never hit any other headers (Go abstracts them away from us),
	// and we can't handle any custom Tar extensions. So just error out.
	default:
		return fmt.Errorf("unpack entry: %s: unknown typeflag '\\x%x'", hdr.Name, hdr.Typeflag)
	}

out:
	// Apply the metadata, which will apply any mappings necessary. We don't
	// apply metadata for hardlinks, because hardlinks don't have any separate
	// metadata from their link (and the tar headers might not be filled).
	if hdr.Typeflag != tar.TypeLink {
		if err := te.applyMetadata(path, hdr); err != nil {
			return fmt.Errorf("apply hdr metadata: %w", err)
		}
	}

	// Everything is done -- the path now exists. Add it (and all its
	// ancestors) to the set of upper paths. We first have to figure out the
	// proper path corresponding to hdr.Name though.
	upperPath, err := filepath.Rel(root, path)
	if err != nil {
		// Really shouldn't happen because of the guarantees of SecureJoinVFS.
		return fmt.Errorf("find relative-to-root [should never happen]: %w", err)
	}
	for pth := upperPath; pth != filepath.Dir(pth); pth = filepath.Dir(pth) {
		te.upperPaths[pth] = struct{}{}
	}
	return nil
}
