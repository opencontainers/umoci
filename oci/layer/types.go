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
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// WhiteoutMode indicates how this TarExtractor will create whiteouts on the
// filesystem when it encounters them.
type WhiteoutMode int

const (
	// OCIStandardWhiteout does the standard OCI thing: a file named
	// .wh.foo indicates you should rm -rf foo.
	OCIStandardWhiteout WhiteoutMode = iota

	// OverlayFSWhiteout generates a rootfs suitable for use in overlayfs,
	// so it follows the overlayfs whiteout protocol:
	//     .wh.foo => mknod c 0 0 foo
	OverlayFSWhiteout
)

// UnpackOptions describes the behavior of the various unpack operations.
type UnpackOptions struct {
	// MapOptions are the UID and GID mappings used when unpacking an image
	MapOptions MapOptions

	// KeepDirlinks is essentially the same as rsync's optio
	// --keep-dirlinks: if, on extraction, a directory would be created
	// where a symlink to a directory previously existed, KeepDirlinks
	// doesn't create that directory, but instead just uses the existing
	// symlink.
	KeepDirlinks bool

	// AfterLayerUnpack is a function that's called after every layer is
	// unpacked.
	AfterLayerUnpack AfterLayerUnpackCallback

	// StartFrom is the descriptor in the manifest to start from
	StartFrom ispec.Descriptor

	// WhiteoutMode is the type of whiteout to write to the filesystem.
	WhiteoutMode WhiteoutMode
}

// RepackOptions describes the behavior of the various GenerateLayer operations.
type RepackOptions struct {
	// MapOptions are the UID and GID mappings used when unpacking an image
	MapOptions MapOptions

	// TranslateOverlayWhiteouts changes char devices of type 0,0 to
	// .wh.foo style whiteouts when generating tarballs. Without this,
	// whiteouts are untouched.
	TranslateOverlayWhiteouts bool
}
