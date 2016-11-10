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
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/system"
	"github.com/cyphar/umoci/third_party/symlink"
)

// applyMetadata applies the state described in tar.Header to the filesystem at
// the given path. No sanity checking is done of the tar.Header's pathname or
// other information.
func applyMetadata(path string, hdr *tar.Header) error {
	// Some of the tar.Header fields don't match the OS API.
	fi := hdr.FileInfo()

	// We cannot apply hdr.Mode to symlinks, because symlinks don't have a mode
	// of their own (they're special in that way).
	// XXX: Make sure that the same doesn't hold for hardlinks in a tar file
	//      (hardlinks share their inode, but in a tar file they have separate
	//      headers).
	if hdr.Typeflag != tar.TypeSymlink {
		if err := os.Chmod(path, fi.Mode()); err != nil {
			return fmt.Errorf("apply metadata: %s: %s", path, err)
		}
	}

	// Apply owner.
	if err := os.Lchown(path, hdr.Uid, hdr.Gid); err != nil {
		return fmt.Errorf("apply metadata: %s: %s", path, err)
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

	if err := system.Lutimes(path, atime, mtime); err != nil {
		return fmt.Errorf("apply metadata: %s: %s", path, err)
	}

	// Apply xattrs. In order to make sure that we *only* have the xattr set we
	// want, we first clear the set of xattrs from the file then apply the ones
	// set in the tar.Header.
	// FIXME: This will almost certainly break horribly on RedHat.
	if err := system.Lclearxattrs(path); err != nil {
		return fmt.Errorf("apply metadata: %s: %s", path, err)
	}
	for name, value := range hdr.Xattrs {
		if err := system.Lsetxattr(path, name, []byte(value), 0); err != nil {
			return fmt.Errorf("apply metadata: %s: %s", path, err)
		}
	}

	// TODO.
	return nil
}

// unpackEntry extracts the given tar.Header to the provided root, ensuring
// that the layer state is consistent with the layer state that produced the
// tar archive being iterated over. This does handle whiteouts, so a tar.Header
// that represents a whiteout will result in the path being removed.
func unpackEntry(root string, hdr *tar.Header, r io.Reader) error {
	logrus.WithFields(logrus.Fields{
		"root": root,
		"path": hdr.Name,
		"type": hdr.Typeflag,
	}).Debugf("unpacking entry")

	// Make hdr.Name safe.
	hdr.Name = filepath.Clean(filepath.Join("/", hdr.Name))

	// Get directory and filename, but we have to safely get the directory
	// component of the path. FollowSymlinkInScope will evaluate the path
	// itself, which we don't want (we're clever enough to handle the actual
	// path being a symlink).
	unsafePath := filepath.Join(root, hdr.Name)
	unsafeDir, file := filepath.Split(unsafePath)
	dir, err := symlink.FollowSymlinkInScope(unsafeDir, root)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, file)

	// Before we do anything, get the state of dir. Because we might be adding
	// or removing files, our parent directory might be modified in the
	// process. As a result, we want to be able to restore the old state
	// (because we only apply state that we find in the archive we're iterating
	// over). We can safely ignore an error here, because a non-existent
	// directory will be fixed by later archive entries.
	if dirFi, err := os.Lstat(dir); err == nil {
		// FIXME: This is really stupid.
		link, _ := os.Readlink(dir)
		dirHdr, err := tar.FileInfoHeader(dirFi, link)
		if err != nil {
			return err
		}

		// FIXME: This doesn't return an error, we should fix that by using the
		//        (really dumb) (err error) construction.
		defer func() {
			if err := applyMetadata(dir, dirHdr); err != nil {
				panic(err)
			}
		}()
	}

	// FIXME: Currently we cannot use os.Link because we have to wait until the
	//        entire archive has been extracted to be sure that hardlinks will
	//        work. There are a few ways of solving this, one of which is to
	//        keep an inode index. For now we don't have any other option than
	//        to "fake" hardlinks with symlinks.
	if hdr.Typeflag == tar.TypeLink {
		hdr.Typeflag = tar.TypeSymlink
	}

	// Get information about the path.
	hdrFi := hdr.FileInfo()
	fi, err := os.Lstat(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}

		// File doesn't exist, just switch fi to the file header.
		fi = hdr.FileInfo()
	}

	// Currently the spec doesn't specify what the hdr.Typeflag of whiteout
	// files is meant to be. We specifically only produce regular files
	// ('\x00') but it could be possible that someone produces a different
	// Typeflag, expecting that the path is the only thing that matters in a
	// whiteout entry.
	if strings.HasPrefix(file, whPrefix) {
		file = strings.TrimPrefix(file, whPrefix)
		path = filepath.Join(dir, file)

		// We would like to use RemoveAll, to recursively remove directories,
		// but we first need to make sure that the directory existed in the
		// first place.
		if _, err := os.Lstat(path); err != nil {
			if os.IsNotExist(err) {
				err = fmt.Errorf("unpack entry: encountered whiteout %s: %s", hdr.Name, err)
			}
			return err
		}

		// Just remove the path. The defer will reapply the correct parent
		// metadata. We have nothing left to do here.
		return os.RemoveAll(path)
	}

	// If the type of the file has changed, there's nothing we can do other
	// than just remove the old path and replace it.
	// XXX Is this actually valid according to the spec? Do you need to have a
	//     whiteout in this case, or can we just assume that a change in the
	//     type is reason enough to purge the old type.
	if hdrFi.Mode()&os.ModeType != fi.Mode()&os.ModeType {
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}

	// Attempt to create the parent directory of the path we're unpacking.
	// We do a MkdirAll here because even though you need to have a tar entry
	// for every component of a new path, applyMetadata will correct any
	// inconsistencies.
	//
	// FIXME: We have to make this consistent, since if the tar archive doesn't
	//        have entries for some of these components we won't be able to
	//        verify that we have consistent results during unpacking.
	if err := os.MkdirAll(dir, 0777); err != nil {
		return err
	}

	// Now create or otherwise modify the state of the path. Right now, either
	// the type of path matches hdr or the path doesn't exist. Note that we
	// don't care about umasks or the initial mode here, since applyMetadata
	// will fix all of that for us.
	switch hdr.Typeflag {
	// regular file
	case tar.TypeReg, tar.TypeRegA:
		// Truncate file, then just copy the data.
		fh, err := os.Create(path)
		if err != nil {
			return err
		}
		defer fh.Close()

		// We need to make sure that we copy all of the bytes.
		if n, err := io.Copy(fh, r); err != nil {
			return err
		} else if int64(n) != hdr.Size {
			return fmt.Errorf("unpack entry: regular file %s: incomplete write", hdr.Name)
		}

		// Force close here so that we don't affect the metadata.
		fh.Close()

	// directory
	case tar.TypeDir:
		// Attempt to create the directory. We do a MkdirAll here because even
		// though you need to have a tar entry for every component of a new
		// path, applyMetadata will correct any inconsistencies.
		if err := os.MkdirAll(path, 0777); err != nil {
			return err
		}

	// hard link, symbolic link
	case tar.TypeLink, tar.TypeSymlink:
		// In both cases, we have to just unlinkg and then re-link the given
		// path. The only difference is the function we're using.
		var linkFn func(string, string) error
		switch hdr.Typeflag {
		case tar.TypeLink:
			linkFn = os.Link
		case tar.TypeSymlink:
			linkFn = os.Symlink
		}

		// Unlink the old path, and ignore it if the path didn't exist.
		if err := os.RemoveAll(path); err != nil {
			return err
		}

		// Link the new one.
		if err := linkFn(hdr.Linkname, path); err != nil {
			return err
		}

	// character device node, block device node, fifo node
	case tar.TypeChar, tar.TypeBlock, tar.TypeFifo:
		// We have to remove and then create the device. In the FIFO case we
		// could choose not to do so, but we do it anyway just to be on the
		// safe side.
		// FIXME: What is the right thing to do here? If we don't remove an
		//        existing node we might end up with a container having
		//        unexpected buffered data. If we do remove it we might remove
		//        data that we shouldn't have.

		mode := system.Tarmode(hdr.Typeflag)
		dev := system.Makedev(uint64(hdr.Devmajor), uint64(hdr.Devminor))

		// Unlink the old path, and ignore it if the path didn't exist.
		if err := os.RemoveAll(path); err != nil {
			return err
		}

		// Create the node.
		if err := system.Mknod(path, os.FileMode(mode|0666), dev); err != nil {
			return err
		}

	// We should never hit any other headers (Go abstracts them away from us),
	// and we can't handle any custom Tar extensions. So just error out.
	default:
		return fmt.Errorf("unpack entry: %s: unknown typeflag '\\x%x'", hdr.Name, hdr.Typeflag)
	}

	// Apply the metadata.
	if err := applyMetadata(path, hdr); err != nil {
		return err
	}

	return nil
}
