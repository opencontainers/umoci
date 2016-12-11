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
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/cyphar/umoci/pkg/unpriv"
	"github.com/pkg/errors"
)

// tarGenerator is a helper for generating layer diff tars. It should be noted
// that when using tarGenerator.Add{Path,Whiteout} it is recommended to do it
// in lexicographic order.
type tarGenerator struct {
	tw *tar.Writer

	// mapOptions is the set of mapping options for modifying entries before
	// they're added to the layer.
	mapOptions MapOptions

	// Hardlink mapping.
	// XXX: Do we need to handle having a rootfs/ which is on more than one
	//      filesystem? In which case this will have to be more complicated
	//      than a simple inode mapping.
	inodes map[uint64]string

	// Parent directory mappings, so we can add dummy entries for any parent
	// directory we wanted to modify.
	// XXX: Is this actually necessary? Docker does this to "preserve
	//      permissions" but I'm not entirely convinced it's necessary and as
	//      far as I can tell there's no explicit requirement in the image-spec
	//      that mandates this behaviour.
	directories map[string]bool

	// XXX: Should we add a saftey check to make sure we don't generate two of
	//      the same path in a tar archive? This is not permitted by the spec.
}

// newTarGenerator creates a new tarGenerator using the provided writer as the
// output writer.
func newTarGenerator(w io.Writer, opt MapOptions) *tarGenerator {
	return &tarGenerator{
		tw:          tar.NewWriter(w),
		mapOptions:  opt,
		inodes:      map[uint64]string{},
		directories: map[string]bool{},
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

	// Check that the path is "safe", meaning that it doesn't resolve outside
	// of the tar archive. While this might seem paranoid, it is a legitimate
	// concern.
	if "/"+path != filepath.Join("/", path) {
		return "", errors.Errorf("escape warning: generated path is outside tar root: %s", rawPath)
	}

	// With some other tar formats, you needed to have a '/' at the end of a
	// pathname in order to state that it is a directory. While this is no
	// longer necessary, some older tooling may assume that.
	if isDir {
		path += "/"
	}

	return path, nil
}

func (tg *tarGenerator) open(path string) (*os.File, error) {
	open := os.Open
	if tg.mapOptions.Rootless {
		open = unpriv.Open
	}
	return open(path)
}

func (tg *tarGenerator) lstat(path string) (os.FileInfo, error) {
	lstat := os.Lstat
	if tg.mapOptions.Rootless {
		lstat = unpriv.Lstat
	}
	return lstat(path)
}

func (tg *tarGenerator) readlink(path string) (string, error) {
	readlink := os.Readlink
	if tg.mapOptions.Rootless {
		readlink = unpriv.Readlink
	}
	return readlink(path)
}

// AddFile adds a file from the filesystem to the tar archive. It copies all of
// the relevant stat information about the file, and also attempts to track
// hardlinks. This should be functionally equivalent to adding entries with GNU
// tar.
func (tg *tarGenerator) AddFile(name, path string) error {
	fi, err := tg.lstat(path)
	if err != nil {
		return errors.Wrap(err, "add file lstat")
	}

	linkname := ""
	if fi.Mode()&os.ModeSymlink == os.ModeSymlink {
		if linkname, err = tg.readlink(path); err != nil {
			return errors.Wrap(err, "add file readlink")
		}
	}

	hdr, err := tar.FileInfoHeader(fi, linkname)
	if err != nil {
		return errors.Wrap(err, "convert fi to hdr")
	}

	name, err = normalise(name, fi.IsDir())
	if err != nil {
		return errors.Wrap(err, "normalise path")
	}
	hdr.Name = name

	// FIXME: Do we need to ensure that the parent paths have all been added to
	//        the archive? I haven't found any tar specification that makes
	//        this mandatory, but I have a feeling that some people might rely
	//        on it. The issue with implementing it is that we'd have to get
	//        the FileInfo about the directory from somewhere (and we don't
	//        want to waste space by adding an entry that will be overwritten
	//        later).

	// Different systems have different special things they need to set within
	// a tar header. In principle, tar.FileInfoHeader should've done it for us
	// but we might as well double-check it.
	if err := updateHeader(hdr, fi); err != nil {
		return errors.Wrap(err, "update hdr header")
	}

	// Not all systems have the concept of an inode, but I'm not in the mood to
	// handle this in a way that makes anything other than GNU/Linux happy
	// right now.
	ino, err := getInode(fi)
	if err != nil {
		return errors.Wrap(err, "get inode")
	}

	// Handle hardlinks.
	if oldpath, ok := tg.inodes[ino]; ok {
		// We just hit a hardlink, so we just have to change the header.
		hdr.Typeflag = tar.TypeLink
		hdr.Linkname = oldpath
		hdr.Size = 0
	} else {
		tg.inodes[ino] = name
	}

	// XXX: What about xattrs.

	// Apply any header mappings.
	if err := mapHeader(hdr, tg.mapOptions); err != nil {
		return errors.Wrap(err, "map header")
	}
	if err := tg.tw.WriteHeader(hdr); err != nil {
		return errors.Wrap(err, "write header")
	}

	// Write the contents of regular files.
	if hdr.Typeflag == tar.TypeReg {
		// XXX: Do we need bufio here?
		fh, err := tg.open(path)
		if err != nil {
			return errors.Wrap(err, "open file")
		}
		defer fh.Close()

		n, err := io.Copy(tg.tw, fh)
		if err != nil {
			return errors.Wrap(err, "copy to layer")
		}
		if n != hdr.Size {
			return errors.Wrap(io.ErrShortWrite, "copy to layer")
		}
	}

	return nil
}

const whPrefix = ".wh."

// AddWhiteout adds a whiteout file for the given name inside the tar archive.
// It's not recommended to add a file with AddFile and then white it out.
//
// TODO: We don't use opaque whiteouts if we have a directory which has had
//       many children removed. While this is fine for the image-spec (in fact
//       it recommends it) I am not entirely sure this is the best idea in the
//       world.
func (tg *tarGenerator) AddWhiteout(name string) error {
	name, err := normalise(name, false)
	if err != nil {
		return errors.Wrap(err, "normalise path")
	}

	// Create the explicit whiteout for the file.
	// FIXME: Currently we are not ignoring directories which have been entirely
	//        removed. This means that we will generate an explicit whiteout
	//        file for every file underneath a deleted directory. I'm not
	//        entirely sure this is actually correct.

	dir, file := filepath.Split(name)
	whiteout := filepath.Join(dir, whPrefix+file)
	timestamp := time.Now()

	// FIXME: Do we need to ensure that the parent paths have all been added to
	//        the archive? I haven't found any tar specification that makes
	//        this mandatory, but I have a feeling that some people might rely
	//        on it.
	//
	//        The big issue with implementing it for whiteouts is that you then
	//        have to ensure you match the old FileInfo in a lower layer (which
	//        we don't have access to in this context). In addition, I don't
	//        really buy why it would be necessary.

	// Add a dummy header for the whiteout file.
	if err := tg.tw.WriteHeader(&tar.Header{
		Name:       whiteout,
		Size:       0,
		ModTime:    timestamp,
		AccessTime: timestamp,
		ChangeTime: timestamp,
	}); err != nil {
		return errors.Wrap(err, "write whiteout header")
	}

	return nil
}
