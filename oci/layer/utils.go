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
	"os"
	"path/filepath"

	"github.com/openSUSE/umoci/pkg/idtools"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// MapOptions specifies the UID and GID mappings used when unpacking and
// repacking images.
type MapOptions struct {
	// UIDMappings and GIDMappings are the UID and GID mappings to apply when
	// packing and unpacking image rootfs layers.
	UIDMappings []rspec.LinuxIDMapping `json:"uid_mappings"`
	GIDMappings []rspec.LinuxIDMapping `json:"gid_mappings"`

	// Rootless specifies whether any to error out if chown fails.
	Rootless bool `json:"rootless"`
}

// mapHeader maps a tar.Header generated from the filesystem so that it
// describes the inode as it would be observed by a container process. In
// particular this involves apply an ID mapping from the host filesystem to the
// container mappings. Returns an error if it's not possible to map the given
// UID.
func mapHeader(hdr *tar.Header, mapOptions MapOptions) error {
	// If we're in rootless mode, we assume all of the files are owned by
	// (0, 0) in the container -- since we cannot map any other users.
	if mapOptions.Rootless {
		hdr.Uid, _ = idtools.ToHost(0, mapOptions.UIDMappings)
		hdr.Gid, _ = idtools.ToHost(0, mapOptions.GIDMappings)
	}

	newUID, err := idtools.ToContainer(hdr.Uid, mapOptions.UIDMappings)
	if err != nil {
		return errors.Wrap(err, "map uid to container")
	}
	newGID, err := idtools.ToContainer(hdr.Gid, mapOptions.GIDMappings)
	if err != nil {
		return errors.Wrap(err, "map gid to container")
	}

	hdr.Uid = newUID
	hdr.Gid = newGID
	return nil
}

// unmapHeader maps a tar.Header from a tar layer stream so that it describes
// the inode as it would be exist on the host filesystem. In particular this
// involves applying an ID mapping from the container filesystem to the host
// mappings. Returns an error if it's not possible to map the given UID.
func unmapHeader(hdr *tar.Header, mapOptions MapOptions) error {
	// If we're in rootless mode we assume that all of the files in the layer
	// are owned by (0, 0) because we cannot map any other users in the
	// container (and we cannot Lchown to any user other than ourselves).
	if mapOptions.Rootless {
		hdr.Uid = 0
		hdr.Gid = 0
	}

	newUID, err := idtools.ToHost(hdr.Uid, mapOptions.UIDMappings)
	if err != nil {
		return errors.Wrap(err, "map uid to host")
	}
	newGID, err := idtools.ToHost(hdr.Gid, mapOptions.GIDMappings)
	if err != nil {
		return errors.Wrap(err, "map gid to host")
	}

	hdr.Uid = newUID
	hdr.Gid = newGID
	return nil
}

// CleanPath makes a path safe for use with filepath.Join. This is done by not
// only cleaning the path, but also (if the path is relative) adding a leading
// '/' and cleaning it (then removing the leading '/'). This ensures that a
// path resulting from prepending another path will always resolve to lexically
// be a subdirectory of the prefixed path. This is all done lexically, so paths
// that include symlinks won't be safe as a result of using CleanPath.
//
// This function comes from runC (libcontainer/utils/utils.go).
func CleanPath(path string) string {
	// Deal with empty strings nicely.
	if path == "" {
		return ""
	}

	// Ensure that all paths are cleaned (especially problematic ones like
	// "/../../../../../" which can cause lots of issues).
	path = filepath.Clean(path)

	// If the path isn't absolute, we need to do more processing to fix paths
	// such as "../../../../<etc>/some/path". We also shouldn't convert absolute
	// paths to relative ones.
	if !filepath.IsAbs(path) {
		path = filepath.Clean(string(os.PathSeparator) + path)
		// This can't fail, as (by definition) all paths are relative to root.
		path, _ = filepath.Rel(string(os.PathSeparator), path)
	}

	// Clean the path again for good measure.
	return filepath.Clean(path)
}
