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
	"bytes"
	"errors"
	"fmt"
	"slices"

	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/pkg/fseval"
)

type overlayWhiteoutType string

const (
	overlayWhiteoutPlain  overlayWhiteoutType = "plain whiteout"
	overlayWhiteoutOpaque overlayWhiteoutType = "opaque whiteout"
)

// isOverlayWhiteout returns true if the given path is an overlayfs whiteout,
// and what kind of whiteout it represents.
func isOverlayWhiteout(onDiskFmt OverlayfsRootfs, path string, fsEval fseval.FsEval) (overlayWhiteoutType, bool, error) {
	stat, err := fsEval.Lstatx(path)
	if err != nil {
		return "", false, err
	}

	switch stat.Mode & unix.S_IFMT {
	case unix.S_IFCHR:
		// classic char 0:0 style whiteouts
		if stat.Rdev == 0 {
			return overlayWhiteoutPlain, true, nil
		}
	case unix.S_IFDIR:
		// opaque whiteouts
		val, err := fsEval.Lgetxattr(path, onDiskFmt.xattr("opaque"))
		if err != nil {
			// If we are missing privileges to read the xattr (ENODATA) or the
			// filesystem doesn't support xattrs (EOPNOTSUPP) we ignore the
			// xattr.
			if !errors.Is(err, unix.EOPNOTSUPP) && !errors.Is(err, unix.ENODATA) {
				return "", false, fmt.Errorf("failed to get overlayfs opaque whiteout xattr: %w", err)
			}
			return "", false, nil
		}
		if bytes.Equal(val, []byte("y")) {
			return overlayWhiteoutOpaque, true, nil
		}
		// TODO: What should we do for overlay.opaque=x? The docs imply that it
		// should act like an opaque directory but in practice it seems that it
		// only has an effect on whether overlay.whiteout shows up in readdir.
	case unix.S_IFREG:
		// overlayfs xattr-whiteouts
		if stat.Size == 0 {
			// Unlike overlay.opaque, the value stored in overlay.whiteout is
			// not actually relevant -- it just needs to be set. Unprivileged
			// users will get ENODATA if they try to read trusted.* xattrs
			// (which is the same as for xattrs that don't exist), which would
			// normally require us to ignore the xattr but we can very
			// trivially work around this by listing the xattrs and checking if
			// the xattr is present.
			names, err := fsEval.Llistxattr(path)
			if err != nil {
				if !errors.Is(err, unix.EOPNOTSUPP) {
					return "", false, fmt.Errorf("failed to get xattr list to look for overlayfs.whiteout: %w", err)
				}
				names = []string{}
			}
			if slices.Contains(names, onDiskFmt.xattr("whiteout")) {
				return overlayWhiteoutPlain, true, nil
			}
		}
	}
	return "", false, nil
}
