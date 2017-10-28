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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/cyphar/filepath-securejoin"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/openSUSE/umoci/pkg/system"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type tarExtractor struct {
	// mapOptions is the set of mapping options to use when extracting filesystem layers.
	mapOptions MapOptions

	// fsEval is an fseval.FsEval used for extraction.
	fsEval fseval.FsEval
}

// newTarExtractor creates a new tarExtractor.
func newTarExtractor(opt MapOptions) *tarExtractor {
	fsEval := fseval.DefaultFsEval
	if opt.Rootless {
		fsEval = fseval.RootlessFsEval
	}

	return &tarExtractor{
		mapOptions: opt,
		fsEval:     fsEval,
	}
}

// restoreMetadata applies the state described in tar.Header to the filesystem
// at the given path. No sanity checking is done of the tar.Header's pathname
// or other information. In addition, no mapping is done of the header.
func (te *tarExtractor) restoreMetadata(path string, hdr *tar.Header) error {
	// Some of the tar.Header fields don't match the OS API.
	fi := hdr.FileInfo()

	// Get the _actual_ file info to figure out if the path is a symlink.
	isSymlink := hdr.Typeflag == tar.TypeSymlink
	if realFi, err := te.fsEval.Lstat(path); err == nil {
		isSymlink = realFi.Mode()&os.ModeSymlink == os.ModeSymlink
	}

	// Apply owner (only used in non-rootless case).
	if !te.mapOptions.Rootless {
		// XXX: While unpriv.Lchown doesn't make a whole lot of sense this
		//      should _probably_ be put inside FsEval.
		if err := os.Lchown(path, hdr.Uid, hdr.Gid); err != nil {
			return errors.Wrapf(err, "restore chown metadata: %s", path)
		}
	}

	// We cannot apply hdr.Mode to symlinks, because symlinks don't have a mode
	// of their own (they're special in that way). We have to apply this after
	// we've applied the owner because setuid bits are cleared when changing
	// owner (in rootless we don't care because we're always the owner).
	if !isSymlink {
		if err := te.fsEval.Chmod(path, fi.Mode()); err != nil {
			return errors.Wrapf(err, "restore chmod metadata: %s", path)
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
	if err := te.fsEval.Lclearxattrs(path); err != nil {
		return errors.Wrapf(err, "clear xattr metadata: %s", path)
	}
	for name, value := range hdr.Xattrs {
		if err := te.fsEval.Lsetxattr(path, name, []byte(value), 0); err != nil {
			// In rootless mode, some xattrs will fail (security.capability).
			// This is _fine_ as long as we're not running as root (in which
			// case we shouldn't be ignoring xattrs that we were told to set).
			if te.mapOptions.Rootless && os.IsPermission(errors.Cause(err)) {
				log.Warnf("rootless{%s} ignoring (usually) harmless EPERM on setxattr %q", hdr.Name, name)
				continue
			}
			return errors.Wrapf(err, "restore xattr metadata: %s", path)
		}
	}

	if err := te.fsEval.Lutimes(path, atime, mtime); err != nil {
		return errors.Wrapf(err, "restore lutimes metadata: %s", path)
	}

	return nil
}

// applyMetadata applies the state described in tar.Header to the filesystem at
// the given path, using the state of the tarExtractor to remap information
// within the header. This should only be used with headers from a tar layer
// (not from the filesystem). No sanity checking is done of the tar.Header's
// pathname or other information.
func (te *tarExtractor) applyMetadata(path string, hdr *tar.Header) error {
	// Modify the header.
	if err := unmapHeader(hdr, te.mapOptions); err != nil {
		return errors.Wrap(err, "unmap header")
	}

	// Restore it on the filesystme.
	return te.restoreMetadata(path, hdr)
}

// unpackEntry extracts the given tar.Header to the provided root, ensuring
// that the layer state is consistent with the layer state that produced the
// tar archive being iterated over. This does handle whiteouts, so a tar.Header
// that represents a whiteout will result in the path being removed.
func (te *tarExtractor) unpackEntry(root string, hdr *tar.Header, r io.Reader) (Err error) {
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
	}
	dir, err := securejoin.SecureJoinVFS(root, unsafeDir, te.fsEval)
	if err != nil {
		return errors.Wrap(err, "sanitise symlinks in root")
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
		link, _ := te.fsEval.Readlink(dir)
		dirHdr, err := tar.FileInfoHeader(dirFi, link)
		if err != nil {
			return errors.Wrap(err, "convert dirFi to dirHdr")
		}

		// More faking to trick restoreMetadata to actually restore the directory.
		dirHdr.Typeflag = tar.TypeDir
		dirHdr.Linkname = ""

		// os.Lstat doesn't get the list of xattrs by default. We need to fill
		// this explicitly.
		xattrs, err := te.fsEval.Llistxattr(dir)
		if err != nil {
			return errors.Wrap(err, "get dirHdr.Xattrs")
		}
		if len(xattrs) > 0 {
			dirHdr.Xattrs = map[string]string{}
			for _, xattr := range xattrs {
				value, err := te.fsEval.Lgetxattr(dir, xattr)
				if err != nil {
					return errors.Wrap(err, "get xattr")
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
					Err = errors.Wrap(err, "restore parent directory")
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
		file = strings.TrimPrefix(file, whPrefix)
		path = filepath.Join(dir, file)

		// Unfortunately we can't just stat the file here, because if we hit a
		// parent directory whiteout earlier than this one then stating here
		// would fail. The best solution would be to keep a list of whiteouts
		// we've seen and then Lstat accordingly (though it won't help in some
		// cases).

		// Just remove the path. The defer will reapply the correct parent
		// metadata. We have nothing left to do here.
		if err := te.fsEval.RemoveAll(path); err != nil {
			return errors.Wrap(err, "whiteout remove all")
		}
		return nil
	}

	// Get information about the path. This has to be done after we've dealt
	// with whiteouts because it turns out that lstat(2) will return EPERM if
	// you try to stat a whiteout on AUFS.
	hdrFi := hdr.FileInfo()
	fi, err := te.fsEval.Lstat(path)
	if err != nil {
		// File doesn't exist, just switch fi to the file header.
		fi = hdr.FileInfo()
	}

	// If the type of the file has changed, there's nothing we can do other
	// than just remove the old path and replace it.
	// XXX: Is this actually valid according to the spec? Do you need to have a
	//      whiteout in this case, or can we just assume that a change in the
	//      type is reason enough to purge the old type.
	if hdrFi.Mode()&os.ModeType != fi.Mode()&os.ModeType {
		if err := te.fsEval.RemoveAll(path); err != nil {
			return errors.Wrap(err, "replace removeall")
		}
	}

	// Attempt to create the parent directory of the path we're unpacking.
	// We do a MkdirAll here because even though you need to have a tar entry
	// for every component of a new path, applyMetadata will correct any
	// inconsistencies.
	// FIXME: We have to make this consistent, since if the tar archive doesn't
	//        have entries for some of these components we won't be able to
	//        verify that we have consistent results during unpacking.
	if err := te.fsEval.MkdirAll(dir, 0777); err != nil {
		return errors.Wrap(err, "mkdir parent")
	}

	// Now create or otherwise modify the state of the path. Right now, either
	// the type of path matches hdr or the path doesn't exist. Note that we
	// don't care about umasks or the initial mode here, since applyMetadata
	// will fix all of that for us.
	switch hdr.Typeflag {
	// regular file
	case tar.TypeReg, tar.TypeRegA:
		// Truncate file, then just copy the data.
		fh, err := te.fsEval.Create(path)
		if err != nil {
			return errors.Wrap(err, "create regular")
		}
		defer fh.Close()

		// We need to make sure that we copy all of the bytes.
		if n, err := io.Copy(fh, r); err != nil {
			return err
		} else if int64(n) != hdr.Size {
			return errors.Wrap(io.ErrShortWrite, "unpack to regular file")
		}

		// Force close here so that we don't affect the metadata.
		fh.Close()

	// directory
	case tar.TypeDir:
		// Attempt to create the directory. We do a MkdirAll here because even
		// though you need to have a tar entry for every component of a new
		// path, applyMetadata will correct any inconsistencies.
		if err := te.fsEval.MkdirAll(path, 0777); err != nil {
			return errors.Wrap(err, "mkdirall")
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
				return errors.Wrap(err, "sanitise hardlink target in root")
			}
			linkname = filepath.Join(linkDir, linkFile)
		case tar.TypeSymlink:
			linkFn = te.fsEval.Symlink
		}

		// Unlink the old path, and ignore it if the path didn't exist.
		if err := te.fsEval.RemoveAll(path); err != nil {
			return errors.Wrap(err, "remove link old")
		}

		// Link the new one.
		if err := linkFn(linkname, path); err != nil {
			// FIXME: Currently this can break if tar hardlink entries occur
			//        before we hit the entry those hardlinks link to. I have a
			//        feeling that such archives are invalid, but the correct
			//        way of handling this is to delay link creation until the
			//        very end. Unfortunately this won't work with symlinks
			//        (which can link to directories).
			return errors.Wrap(err, "link")
		}

	// character device node, block device node
	case tar.TypeChar, tar.TypeBlock:
		// In rootless mode we have to fake this.
		if te.mapOptions.Rootless {
			log.Warnf("rootless{%s} creating empty file in place of device %d:%d", hdr.Name, hdr.Devmajor, hdr.Devminor)
			fh, err := te.fsEval.Create(path)
			if err != nil {
				return errors.Wrap(err, "create rootless block")
			}
			defer fh.Close()
			if err := fh.Chmod(0); err != nil {
				return errors.Wrap(err, "chmod 0 rootless block")
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

		// Unlink the old path, and ignore it if the path didn't exist.
		if err := te.fsEval.RemoveAll(path); err != nil {
			return errors.Wrap(err, "remove block old")
		}

		// Create the node.
		if err := te.fsEval.Mknod(path, os.FileMode(int64(mode)|hdr.Mode), dev); err != nil {
			return errors.Wrap(err, "mknod")
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
			return errors.Wrap(err, "apply hdr metadata")
		}
	}

	return nil
}
