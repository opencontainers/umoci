//go:build linux

// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
 * Copyright (C) 2020 Cisco Inc.
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
	"maps"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"

	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/pkg/fseval"
)

func isForbiddenXattr(xattrName string) bool {
	found, ok := getXattrFilter(xattrName)
	return ok && found == forbiddenXattrFilter{}
}

func getAllXattrs(t *testing.T, path string) map[string]string {
	names, err := system.Llistxattr(path)
	require.NoErrorf(t, err, "fetch all xattrs for %q", path)
	xattrs := map[string]string{}
	for _, name := range names {
		value, err := system.Lgetxattr(path, name)
		require.NoErrorf(t, err, "fetch xattr %q for %q", name, path)
		xattrs[name] = string(value)
	}
	return xattrs
}

func testGenerateLayersForRoundTrip(t *testing.T, dir string, onDiskFmt OnDiskFormat, wantDentries []tarDentry) {
	t.Run("ToGenerateLayer", func(t *testing.T) {
		// something reasonable
		mtreeKeywords := []mtree.Keyword{
			"size",
			"type",
			"uid",
			"gid",
			"mode",
		}
		deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
		require.NoError(t, err, "mtree check")

		reader, err := GenerateLayer(dir, deltas, &RepackOptions{
			OnDiskFormat: onDiskFmt,
		})
		require.NoError(t, err, "generate layer")
		defer reader.Close() //nolint:errcheck

		// We expect to get the exact same thing as the original archive
		// entries in the new archive.
		checkLayerEntries(t, reader, wantDentries)
	})

	t.Run("ToGenerateInsertLayer", func(t *testing.T) {
		reader := GenerateInsertLayer(dir, ".", false, &RepackOptions{
			OnDiskFormat: onDiskFmt,
		})
		defer reader.Close() //nolint:errcheck

		// We expect to get the exact same thing as the original archive
		// entries in the new archive.
		checkLayerEntries(t, reader, wantDentries)
	})
}

func testUnpackGenerateRoundTrip_ComplexXattr_OverlayfsRootfs(t *testing.T, tarXattrNamespace string) { //nolint:revive // var-naming is less important than matching the func TestXyz name
	if tarXattrNamespace == "trusted." {
		testNeedsTrustedOverlayXattrs(t)
	}

	dentries := []struct {
		tarDentry
		remapXattrs map[string]string
	}{
		{
			tarDentry{path: ".", ftype: tar.TypeDir, xattrs: map[string]string{
				tarXattrNamespace + "overlay.opaque": "x",
				"user.dummy.xattr":                   "foobar",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.opaque": "x",
				"user.dummy.xattr":                           "foobar",
			},
		},
		// Set a fake overlayfs xattr in the trusted.overlay namespace on a
		// directory that contains entries. Since restoreMetadata() gets called
		// on all parent directories when unpacking, this will cause
		// restoreMetadata() to be run on the same directory multiple times.
		// This lets us test that restoreMetadata will not re-apply the xattr
		// escaping even after being called multiple times.
		{
			tarDentry{path: "foo/", ftype: tar.TypeDir, xattrs: map[string]string{
				tarXattrNamespace + "overlay.fakexattr": "fakexattr",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.fakexattr": "fakexattr",
			},
		},
		// Some subpaths with dummy overlayfs xattrs.
		{
			tarDentry{path: "foo/bar", ftype: tar.TypeReg, contents: "file", xattrs: map[string]string{
				tarXattrNamespace + "overlay.whiteout": "foo",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.whiteout": "foo",
			},
		},
		{
			tarDentry{path: "foo/baz/", ftype: tar.TypeDir, xattrs: map[string]string{
				tarXattrNamespace + "overlay.opaque": "y",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.opaque": "y",
			},
		},
		// Several levels nested overlayfs xattrs.
		{
			tarDentry{path: "foo/extra-nesting/", ftype: tar.TypeDir, xattrs: map[string]string{
				tarXattrNamespace + "overlay.overlay.opaque":                                "x",
				tarXattrNamespace + "overlay.overlay.overlay.whiteout":                      "foobar",
				tarXattrNamespace + "overlay.overlay.overlay.overlay.overlay.overlay.dummy": "dummy xattr",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.overlay.opaque":                                "x",
				tarXattrNamespace + "overlay.overlay.overlay.overlay.whiteout":                      "foobar",
				tarXattrNamespace + "overlay.overlay.overlay.overlay.overlay.overlay.overlay.dummy": "dummy xattr",
			},
		},
		{
			tarDentry{path: "foo/extra-nesting/reg", ftype: tar.TypeReg, contents: "reg", xattrs: map[string]string{
				tarXattrNamespace + "overlay.overlay.overlay.overlay.overlay.dummy123": "dummy xattr 123",
			}},
			map[string]string{
				tarXattrNamespace + "overlay.overlay.overlay.overlay.overlay.overlay.dummy123": "dummy xattr 123",
			},
		},
	}

	for _, test := range []struct {
		name        string
		onDiskFmt   OnDiskFormat
		expectRemap bool
	}{
		// DirRootfs will never remap overlayfs xattrs.
		{"DirRootfs", DirRootfs{}, false},
		// We only expect remapping if the OverlayfsRootfs on-disk xattr
		// namespace matches the tar xattr namespace. Otherwise the xattrs
		// should be treated the same as any other xattr.
		{"OverlayfsRootfs-TrustedXattr", OverlayfsRootfs{UserXattr: false}, tarXattrNamespace == "trusted."},
		{"OverlayfsRootfs-UserXattr", OverlayfsRootfs{UserXattr: true}, tarXattrNamespace == "user."},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			switch onDiskFmt := test.onDiskFmt.(type) {
			case DirRootfs:
				onDiskFmt.MapOptions.Rootless = os.Geteuid() != 0
				test.onDiskFmt = onDiskFmt
			case OverlayfsRootfs:
				onDiskFmt.MapOptions.Rootless = os.Geteuid() != 0
				test.onDiskFmt = onDiskFmt
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: test.onDiskFmt,
			}

			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de.tarDentry)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)
			}

			for _, de := range dentries {
				path := de.path
				fullPath := filepath.Join(dir, path)

				xattrs := getAllXattrs(t, fullPath)
				// Strip out any forbidden xattrs from the returned set.
				// TODO: This is a little ugly -- ideally we would create
				// copies of the necessary maps with special xattrs added.
				for name := range xattrs {
					if isForbiddenXattr(name) {
						delete(xattrs, name)
					}
				}

				if test.expectRemap {
					assert.Equalf(t, de.remapXattrs, xattrs, "UnpackEntry(%q): expected to see %#v remapped properly", path, de.xattrs)

					onDiskFmt, isOverlayfs := test.onDiskFmt.(OverlayfsRootfs)
					require.True(t, isOverlayfs, "expectRemap must only be true for OverlayfsRootfs")
					// None of the inodes should be actual whiteouts.
					_, isWo, err := isOverlayWhiteout(onDiskFmt, fullPath, fseval.Default)
					require.NoErrorf(t, err, "isOverlayWhiteout(%q)", path)
					assert.Falsef(t, isWo, "isOverlayWhiteout(%q): regular entries with overlayfs xattrs should not end up being unpacked with overlayfs whiteout xattrs", path)
				} else {
					assert.Equalf(t, de.xattrs, xattrs, "UnpackEntry(%q): expected to see %#v not be remapped", path, de.xattrs)
					assert.NotEqualf(t, de.remapXattrs, xattrs, "UnpackEntry(%q): expected to see %#v not be remapped", path, de.xattrs)
				}
			}

			// We expect to get the exact same thing as the original archive
			// entries in the new archive.
			var wantDentries []tarDentry
			for _, dentry := range dentries {
				wantDentries = append(wantDentries, dentry.tarDentry)
			}
			testGenerateLayersForRoundTrip(t, dir, unpackOptions.OnDiskFormat, wantDentries)
		})
	}
}

func TestUnpackGenerateRoundTrip_ComplexXattr_OverlayfsRootfs(t *testing.T) {
	t.Run("TarEntries=trusted.overlay.", func(t *testing.T) {
		testUnpackGenerateRoundTrip_ComplexXattr_OverlayfsRootfs(t, "trusted.")
	})

	t.Run("TarEntries=user.overlay.", func(t *testing.T) {
		testUnpackGenerateRoundTrip_ComplexXattr_OverlayfsRootfs(t, "user.")
	})
}

func TestUnpackGenerateRoundTrip_MockedSELinux(t *testing.T) {
	// For test purposes we add a fake forbidden attribute that an unprivileged
	// user can easily write to (and thus we can test it). This is meant to be
	// a stand-in for "security.selinux" or any other xattr that gets
	// auto-applied and needs special handling with forbiddenXattrFilter{}.
	const forbiddenTestXattr = "user.UMOCI.fake_selinux"
	specialXattrs[forbiddenTestXattr] = forbiddenXattrFilter{}
	defer delete(specialXattrs, forbiddenTestXattr)

	// Make sure it actually is masked according to the filters.
	filter, isSpecial := getXattrFilter(forbiddenTestXattr)
	require.Truef(t, isSpecial, "getXattrFilter(%q) should return a filter", forbiddenTestXattr)
	require.Equalf(t, forbiddenXattrFilter{}, filter, "getXattrFilter(%q) should return the forbidden filter", forbiddenTestXattr)
	require.Truef(t, filter.MaskedOnDisk(DirRootfs{}, forbiddenTestXattr), "getXattrFilter(%q).MaskedOnDisk should be true", forbiddenTestXattr)

	dentries := []struct {
		tarDentry
		autoXattrs map[string]string
	}{
		{
			tarDentry{path: ".", ftype: tar.TypeDir, xattrs: map[string]string{
				"user.dummy.xattr": "foobar",
			}},
			map[string]string{
				forbiddenTestXattr: "rootdir",
				// This should be auto-cleared because its not a masked xattr
				// nor is it in the tar header.
				"user.UMOCI.fake_nonmasked_xattr": "should get removed",
			},
		},
		{
			tarDentry{path: "foo/", ftype: tar.TypeDir, xattrs: map[string]string{
				"user.dummy.xattr": "barbaz",
			}},
			map[string]string{
				forbiddenTestXattr: "foodir",
			},
		},
		{
			tarDentry{path: "foo/bar", ftype: tar.TypeReg, contents: "file"},
			map[string]string{
				forbiddenTestXattr: "foobarfile",
				// This should be auto-cleared because its not a masked xattr
				// nor is it in the tar header.
				"user.UMOCI.another_fake_nonmasked_xattr": "should also get removed",
			},
		},
	}

	for _, test := range []struct {
		name      string
		onDiskFmt OnDiskFormat
	}{
		{"DirRootfs", DirRootfs{}},
		{"OverlayfsRootfs", OverlayfsRootfs{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()

			switch onDiskFmt := test.onDiskFmt.(type) {
			case DirRootfs:
				onDiskFmt.MapOptions.Rootless = os.Geteuid() != 0
				test.onDiskFmt = onDiskFmt
			case OverlayfsRootfs:
				onDiskFmt.MapOptions.Rootless = os.Geteuid() != 0
				test.onDiskFmt = onDiskFmt
			}

			unpackOptions := UnpackOptions{
				OnDiskFormat: test.onDiskFmt,
			}

			te := NewTarExtractor(&unpackOptions)

			for _, de := range dentries {
				hdr, rdr := tarFromDentry(de.tarDentry)
				err := te.UnpackEntry(dir, hdr, rdr)
				require.NoErrorf(t, err, "UnpackEntry %s", hdr.Name)

				// Apply the "auto" xattrs -- in order to make it seem like this
				// was done automatically during extraction when the inode was
				// created, we want to call applyMetadata here again to emulate
				// this xattr being added by the system during UnpackEntry.
				pth := filepath.Join(dir, de.path)
				for xattr, value := range de.autoXattrs {
					err := unix.Lsetxattr(pth, xattr, []byte(value), 0)
					require.NoErrorf(t, err, "setxattr %s=%s on %q", xattr, value, hdr.Name)
				}
				err = te.restoreMetadata(pth, hdr)
				require.NoErrorf(t, err, "restoreMetadata %s", hdr.Name)
			}

			for _, de := range dentries {
				path := de.path
				fullPath := filepath.Join(dir, path)

				xattrs := getAllXattrs(t, fullPath)

				wantXattrs := map[string]string{}
				// We should see all of the hdr xattrs.
				maps.Copy(wantXattrs, de.xattrs)
				// Of the auto-applied xattrs, we only expect to see our dummy
				// forbidden xattr after all the extractions.
				if value, ok := de.autoXattrs[forbiddenTestXattr]; ok {
					wantXattrs[forbiddenTestXattr] = value
				}
				// If we are running on an SELinux system (or even more
				// unlikely, on top of nfsv4) we need to also allow any masked
				// xattrs auto-applied by the system.
				for gotXattr, value := range xattrs {
					if isForbiddenXattr(gotXattr) {
						wantXattrs[gotXattr] = value
					}
				}
				assert.Equalf(t, wantXattrs, xattrs, "UnpackEntry(%q): expected to only see specific subset of applied xattrs", path)
			}

			// We expect to get the exact same thing as the original archive
			// entries in the new archive.
			var wantDentries []tarDentry
			for _, dentry := range dentries {
				wantDentries = append(wantDentries, dentry.tarDentry)
			}
			testGenerateLayersForRoundTrip(t, dir, unpackOptions.OnDiskFormat, wantDentries)
		})
	}
}
