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
	"strings"
	"time"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// OnDiskFormat represents the on-disk file format that is used when extracting
// and assumed when repacking a rootfs.
//
// [DirRootfs] is the default format used by umoci, and is designed to be used
// to extract a filesystem into a single directory on a regular unix
// filesystem. In order to generate a new diff layer, it is necessary to use
// mtree manifests to track changes.
//
// [OverlayfsRootfs] is an alternative format that is intended to be used with
// overlayfs to avoid the need for mtree manifests. This means that layers are
// intended to be extracted into separate directories and merged together with
// overlayfs, which also means that OCI whiteouts are converted to and from
// overlayfs's format.
//
// NOTE: At the moment [OverlayfsRootfs] cannot be used with the command-line
// version of umoci nor the top-level umoci helpers, and most of the umoci API
// still includes dependencies on mtree manifests (but you can pass a nil
// manifest if using OverlayfsRootfs).
type OnDiskFormat interface {
	// This interface is used to combine both the type of rootfs as well as
	// format-specific options inside the struct. As we use reflection, we
	// cannot allow any external implementations of this interface.
	onDiskFormatInternal()

	// Map returns the format-agnostic information about userns mapping.
	Map() MapOptions
}

// DirRootfs is the default [OnDiskFormat] used by umoci, and is designed to be
// used to extract a filesystem into a single directory on a regular unix
// filesystem. In order to generate a new diff layer, it is necessary to use
// mtree manifests to track changes.
type DirRootfs struct {
	// MapOptions represent the userns mappings that should be applied ot this
	// rootfs.
	MapOptions MapOptions
}

func (DirRootfs) onDiskFormatInternal() {}

// Map returns the format-agnostic information about userns mapping.
func (fs DirRootfs) Map() MapOptions { return fs.MapOptions }

var _ OnDiskFormat = DirRootfs{}

// OverlayfsRootfs is an alternative [OnDiskFormat] to the default [DirRootfs]
// format that is intended to be used with overlayfs to avoid the need for
// mtree manifests. This means that layers are intended to be extracted into
// separate directories and merged together with overlayfs, which also means
// that OCI whiteouts are converted to and from overlayfs's format.
//
// NOTE: At the moment [OverlayfsRootfs] cannot be used with the command-line
// version of umoci nor the top-level umoci helpers, and most of the umoci API
// still includes dependencies on mtree manifests (but you can pass a nil
// manifest if using OverlayfsRootfs).
type OverlayfsRootfs struct {
	// MapOptions represent the userns mappings that should be applied ot this
	// rootfs.
	MapOptions MapOptions

	// UserXattr indicates whether this overlayfs rootfs is going to be mounted
	// using the "userxattr" mount option for overlayfs. If set, then rather
	// than using the "trusted.overlay.*" xattr namespace (the default),
	// "user.overlay.*" will be used instead.
	UserXattr bool
}

func (OverlayfsRootfs) onDiskFormatInternal() {}

// Map returns the format-agnostic information about userns mapping.
func (fs OverlayfsRootfs) Map() MapOptions { return fs.MapOptions }

// xattrNamespace returns the correct top-level xattr namespace for the
// overlayfs mount that this on-disk format was intended for.
func (fs OverlayfsRootfs) xattrNamespace() string {
	if fs.UserXattr {
		return "user."
	}
	return "trusted."
}

// xattr returns the given sub-xattr with the appropriate overlayfs xattr
// prefix applied.
func (fs OverlayfsRootfs) xattr(parts ...string) string {
	return fs.xattrNamespace() + "overlay." + strings.Join(parts, ".")
}

var _ OnDiskFormat = OverlayfsRootfs{}

// MapOptions specifies the UID and GID mappings used when unpacking and
// repacking images, and whether the mapping is being done as a rootless user.
type MapOptions struct {
	// UIDMappings and GIDMappings are the UID and GID mappings to apply when
	// packing and unpacking image rootfs layers.
	UIDMappings []rspec.LinuxIDMapping `json:"uid_mappings"`
	GIDMappings []rspec.LinuxIDMapping `json:"gid_mappings"`

	// Rootless specifies whether any to error out if chown fails.
	Rootless bool `json:"rootless"`
}

// UnpackOptions describes the behavior of the various unpack operations.
type UnpackOptions struct {
	// OnDiskFormat is what extraction format is used when writing to the
	// filesystem. [OverlayfsRootfs] will cause whiteouts to be represented as
	// whiteout inodes in the overlayfs format.
	OnDiskFormat OnDiskFormat

	// KeepDirlinks is essentially the same as rsync's --keep-dirlinks option.
	// If, on extraction, a directory would be created where a symlink to a
	// directory previously existed, KeepDirlinks doesn't create that
	// directory, but instead just uses the existing symlink.
	KeepDirlinks bool

	// AfterLayerUnpack is a function that's called after every layer is
	// unpacked.
	AfterLayerUnpack AfterLayerUnpackCallback

	// StartFrom is the descriptor in the manifest to start unpacking from.
	StartFrom ispec.Descriptor
}

// fill replaces nil values in UnpackOptions with the correct default values.
// If opt itself is nil then a new UnpackOptions struct is allocated and
// returned.
func (opt *UnpackOptions) fill() *UnpackOptions {
	if opt == nil {
		opt = &UnpackOptions{}
	}
	if opt.OnDiskFormat == nil {
		opt.OnDiskFormat = DirRootfs{}
	}
	return opt
}

// MapOptions is shorthand for opt.OnDiskFormat.MapOptions(), except if
// OnDiskFormat is nil then it will return the default MapOptions.
func (opt UnpackOptions) MapOptions() MapOptions {
	var mapOpt MapOptions
	if opt.OnDiskFormat != nil {
		mapOpt = opt.OnDiskFormat.Map()
	}
	return mapOpt
}

// RepackOptions describes the behavior of the various GenerateLayer operations.
type RepackOptions struct {
	// OnDiskFormat is what on-disk format the rootfs we are generating a layer
	// from uses. [OverlayfsRootfs] will cause overlayfs format whiteouts to be
	// converted to OCI whiteouts in the layer.
	OnDiskFormat OnDiskFormat

	// SourceDateEpoch, if set, specifies the timestamp to use for clamping
	// layer content timestamps. If not set, layer content timestamps are
	// preserved as-is.
	SourceDateEpoch *time.Time
}

// fill replaces nil values in RepackOptions with the correct default values.
// If opt itself is nil then a new RepackOptions struct is allocated and
// returned.
func (opt *RepackOptions) fill() *RepackOptions {
	if opt == nil {
		opt = &RepackOptions{}
	}
	if opt.OnDiskFormat == nil {
		opt.OnDiskFormat = DirRootfs{}
	}
	return opt
}

// MapOptions is shorthand for opt.OnDiskFormat.MapOptions(), except if
// OnDiskFormat is nil then it will return the default MapOptions.
func (opt RepackOptions) MapOptions() MapOptions {
	var mapOpt MapOptions
	if opt.OnDiskFormat != nil {
		mapOpt = opt.OnDiskFormat.Map()
	}
	return mapOpt
}
