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

	"github.com/opencontainers/umoci/internal/testhelpers"
)

// TODO: These method names are confusing since GenerateEntry() is called
// during extraction to make restoreMetadata() not break things...

// xattrFilter is used to modify xattrs during layer generation and extraction.
// Very simple filters may completely block the layer generation and extraction
// code from seeing certain system xattrs (such as security.selinux and
// security.nfs4_acl), while others may require xattr names to be remapped
// (such as with {user,trusted}.overlay.*).
//
// [ToTar] is conceptually the inverse of [ToDisk], though both can remove
// xattrs entirely. Some key properties that [ToTar] implementations must
// ensure are:
//
//   - For xattrs where ToTar(xattr) != nil, implementations must ensure that
//     ToDisk(ToTar(xattr)) === xattr.
//
//   - For a given xattr and [OnDiskFormat], [MaskedOnDisk] must return true
//     (meaning that the on-disk xattr will NOT be cleared) *if and only if*
//     [ToTar] will returns nil.
//
// For xattrs that are not purely masked, you can think of [ToTar] as being
// "unescape", and [ToDisk] being "escape", and [MaskedOnDisk] indicating
// whether the xattr is something that [ToDisk] would *not* generate and
// [ToTar] would
// ignore.
type xattrFilter interface {
	// MaskedOnDisk indicates whether the given xattr should be hidden from
	// code that iterates over on-disk xattrs (namely lclearxattrs). Note that
	// we will (during the normal course of extraction) clear xattrs and
	// re-apply them from tar archives, so xattrs should only be masked if
	// umoci is simply not meant to interact with a given xattr.
	//
	// In particular, due to the implementation details of UnpackEntry
	// (described in [ToTar]), MaskedOnDisk must only return true *if and only
	// if* (for the same xattr and [OnDiskFormat]) [ToTar] would return "" and
	// [ToDisk] would never generate it. Otherwise, it is impossible for this
	// on-disk xattr to have come from or be stored in the archive.
	MaskedOnDisk(onDiskFmt OnDiskFormat, xattr string) bool

	// ToDisk returns what name a tar archive xattr should be stored with
	// on-disk (with the provided [OnDiskFormat]). The returned string will be
	// used for as the on-disk xattr name instead of the original xattr -- if
	// an empty string is returned then the xattr will not be extracted.
	//
	// [MaskedOnDisk] must only return true if [ToDisk] would never generate
	// the same xattr.
	ToDisk(onDiskFmt OnDiskFormat, xattr string) (newName string)

	// ToTar returns what an on-disk xattr name should be converted to when
	// storing in a tar archive (with the provided [OnDiskFormat]). The
	// returned string will be stored inside tar archives as the xattr name
	// instead of the original xattr -- if an empty string is returned then the
	// xattr will not be included in the tar archive.
	//
	// Note that ToTar is not exclusively called on tar archive entries from
	// layer archives. When doing UnpackEntry we need to restore the metadata
	// of the parent directory of any path we extract into (to ensure that
	// {a,c,m}times are correct), and an implementation detail of UnpackEntry
	// is that this involves converting the on-disk information of the parent
	// directory into a tar.Header and then back into the on-disk
	// representation (to ensure that extracting entry metadata is identical to
	// restoring parent directory metadata). This means that ToTar will be
	// routinely called on directories that may have "special" xattrs.
	//
	// [MaskedOnDisk] must only return true for a given xattr and
	// [OnDiskFormat] *if and only if*, ToTar with the same arguments returns
	// "", Otherwise, when the metadata is re-applied with on-disk data you
	// will end up losing on-disk xattrs or adding new nonsense xattrs.
	ToTar(onDiskFmt OnDiskFormat, xattr string) (newName string)
}

// forbiddenXattrFilter is a dummy filter that will block all xattrs that are
// associated with it.
type forbiddenXattrFilter struct{}

var _ xattrFilter = forbiddenXattrFilter{}

func (forbiddenXattrFilter) MaskedOnDisk(OnDiskFormat, string) bool { return true }
func (forbiddenXattrFilter) ToDisk(OnDiskFormat, string) string     { return "" }
func (forbiddenXattrFilter) ToTar(OnDiskFormat, string) string      { return "" }

// overlayXattrFilter is a filter for all {user,trusted}.overlay.* xattrs which
// will escape the xattrs on unpack and unescape them when.
type overlayXattrFilter struct {
	// namespace is the xattr namespace used for this overlayfs xattr filter.
	// Examples would be "user." or "trusted.".
	namespace string
}

var _ xattrFilter = overlayXattrFilter{}

func (filter overlayXattrFilter) MaskedOnDisk(onDiskFmt OnDiskFormat, xattr string) bool {
	overlayfsFmt, isOverlayfs := onDiskFmt.(OverlayfsRootfs)
	if !isOverlayfs {
		// In non-overlayfs mode, overlay xattrs are not special and can be
		// treated like any other xattr. (Though it would be a little strange
		// to see them.)
		return false
	}
	if overlayfsFmt.xattrNamespace() != filter.namespace || !doesXattrMatch(xattr, overlayfsFmt.xattr()) {
		// We might be called with a different prefix than the one used for
		// extraction -- overlayfs only supports one xattr namespace for a
		// given mount, so if the prefix doesn't match we treat this like any
		// other xattr.
		return false
	}

	// Only {trusted,user}.overlay.* top-level xattrs are masked in overlayfs
	// mode. Escaped xattrs and xattrs in regular mode are allowed.
	return doesXattrMatch(xattr, filter.namespace+"overlay.") &&
		!doesXattrMatch(xattr, filter.namespace+"overlay.overlay.")
}

func (filter overlayXattrFilter) ToDisk(onDiskFmt OnDiskFormat, xattr string) string {
	if !doesXattrMatch(xattr, filter.namespace+"overlay.") {
		// For some inexplicable reason, we were called with a different xattr
		// namespace. Act as a no-op in that case.
		return xattr
	}

	overlayfsFmt, isOverlayfs := onDiskFmt.(OverlayfsRootfs)
	if !isOverlayfs {
		// In non-overlayfs mode, overlay xattrs are not special and can be
		// treated like any other xattr. (Though it would be a little strange
		// to see them.)
		return xattr
	}
	if overlayfsFmt.xattrNamespace() != filter.namespace {
		// We might be called with a different prefix than the one used for
		// extraction -- overlayfs only supports one xattr namespace for a
		// given mount, so if the prefix doesn't match we treat this like any
		// other xattr.
		return xattr
	}

	// We know it has the prefix so no need for CutPrefix.
	subXattr := strings.TrimPrefix(xattr, filter.namespace+"overlay.")
	return filter.namespace + "overlay.overlay." + subXattr
}

func (filter overlayXattrFilter) ToTar(onDiskFmt OnDiskFormat, xattr string) string {
	if !doesXattrMatch(xattr, filter.namespace+"overlay.") {
		// For some inexplicable reason, we were called with a different xattr
		// namespace. Act as a no-op in that case.
		return xattr
	}

	overlayfsFmt, isOverlayfs := onDiskFmt.(OverlayfsRootfs)
	if !isOverlayfs {
		// In non-overlayfs mode, overlay xattrs are not special and can be
		// treated like any other xattr. (Though it would be a little strange
		// to see them.)
		return xattr
	}
	if overlayfsFmt.xattrNamespace() != filter.namespace {
		// We might be called with a different prefix than the one used for
		// extraction -- overlayfs only supports one xattr namespace for a
		// given mount, so if the prefix doesn't match we treat this like any
		// other xattr.
		return xattr
	}

	subXattr, isEscapedXattr := strings.CutPrefix(xattr, filter.namespace+"overlay.overlay.")
	if !isEscapedXattr {
		// Clear any non-escaped xattrs entirely, as they may have been
		// auto-set by overlayfs or set by the user when configuring overlayfs.
		// This matches the behaviour of MaskedOnDisk.
		return ""
	}
	return filter.namespace + "overlay." + subXattr
}

// specialXattrs is a list of xattr names (or prefixes) that may need have to
// have special handling because treating them as-is would be incorrect (either
// because they are host-specific and need to be hidden from images or would
// result in counter-intuitive behaviour).
//
// TODO: Maybe we should make this configurable so users can manually blacklist
// (or even whitelist) xattrs that they actually want included? Like how GNU
// tar's xattr setup works.
var specialXattrs = map[string]xattrFilter{
	// SELinux doesn't allow you to set SELinux policies generically. They're
	// also host-specific. So just ignore them during extraction.
	"security.selinux": forbiddenXattrFilter{},

	// NFSv4 ACLs are very system-specific and shouldn't be touched by us, nor
	// should they be included in images.
	"system.nfs4_acl": forbiddenXattrFilter{},

	// The overlayfs namespace of xattrs need to have special handling when
	// operating with the overlayfs on-disk format.
	"trusted.overlay.": overlayXattrFilter{"trusted."},
	"user.overlay.":    overlayXattrFilter{"user."},
}

func init() {
	// For test purposes we add a fake forbidden attribute that an unprivileged
	// user can easily write to (and thus we can test it).
	if testhelpers.IsTestBinary() {
		specialXattrs["user.UMOCI:forbidden_xattr"] = forbiddenXattrFilter{}
	}
}

// doesXattrMatch returns whether the given xattr matches the filter. The
// semantics are very simple -- if the filter ends with "." then it is treated
// as a prefix while if it doesn't end with "." it must match exactly.
func doesXattrMatch(xattr, filter string) bool {
	return filter == xattr ||
		(strings.HasSuffix(filter, ".") && strings.HasPrefix(xattr, filter))
}

// getXattrFilter looks for the filter which matches xattr. isSpecial will be
// true if there is a registered filter that matches the provided xattr.
func getXattrFilter(xattr string) (filter xattrFilter, isSpecial bool) {
	// fast path: look up the xattr directly
	if filter, ok := specialXattrs[xattr]; ok {
		return filter, ok
	}
	// slow path: match xattr prefixes
	for prefix, filter := range specialXattrs {
		if doesXattrMatch(xattr, prefix) {
			return filter, true
		}
	}
	return nil, false
}
