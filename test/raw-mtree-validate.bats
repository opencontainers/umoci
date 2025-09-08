#!/usr/bin/env bats -t
# SPDX-License-Identifier: Apache-2.0
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2025 SUSE LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

load helpers

function setup() {
	setup_tmpdirs
	setup_image
}

function teardown() {
	teardown_tmpdirs
	teardown_image
}

@test "umoci raw mtree-validate [unpacked rootfs]" {
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# mtree-validate should pass for a rootfs we just extracted.
	umoci raw mtree-validate -K sha256digest -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
	[ "$status" -eq 0 ]
	# Make sure that regular gomtree succeeds (for rootful runs).
	if [[ "$IS_ROOTLESS" == 0 ]]; then
		sane_run gomtree validate --strict -K sha256digest -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
		[ "$status" -eq 0 ]
	fi
	# Use mtree-validate wrapper.
	mtree-validate -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
	[ "$status" -eq 0 ]

	# Modify the rootfs.
	touch -d "1997-03-25T13:40:00.12345" "$ROOTFS/etc/passwd"

	# mtree-validate should fail for a rootfs we modified.
	umoci raw mtree-validate -K sha256digest -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
	[ "$status" -ne 0 ]
	# Make sure that regular gomtree fails (for rootful runs).
	if [[ "$IS_ROOTLESS" == 0 ]]; then
		sane_run gomtree validate --strict -K sha256digest -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
		[ "$status" -ne 0 ]
	fi
	# Use mtree-validate wrapper.
	mtree-validate -f "$BUNDLE"/sha256_*.mtree -p "$ROOTFS"
	[ "$status" -ne 0 ]
}

@test "umoci raw mtree-validate [basic manifest]" {
	tmpdir="$(setup_tmpdir)"
	rootfs="$tmpdir/rootfs"
	manifest="$tmpdir/rootfs.mtree"

	# Create some dummy files.
	mkdir -p "$rootfs/foo/bar/baz"
	echo "foobar" >"$rootfs/foobar"
	echo "test" > "$rootfs/foo/bar/baz/boop"
	touch -d "1997-03-25T13:40:12" "$rootfs/aki"
	touch -d "2025-09-05T13:05:34" "$rootfs/yuki"

	# Construct a manifest.
	sane_run gomtree -c -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	[ -f "$manifest" ]

	umoci raw mtree-validate -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	sane_run gomtree validate --strict -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]

	touch "$rootfs/aki"

	umoci raw mtree-validate -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	sane_run gomtree validate --strict -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
}

@test "umoci raw mtree-validate -k" {
	tmpdir="$(setup_tmpdir)"
	rootfs="$tmpdir/rootfs"
	manifest="$tmpdir/rootfs.mtree"

	# Create some dummy files.
	mkdir -p "$rootfs/foo/bar/baz"
	echo "foobar" >"$rootfs/foobar"
	echo "test" > "$rootfs/foo/bar/baz/boop"
	touch -d "1997-03-25T13:40:22" "$rootfs/aki"
	touch -d "2025-09-05T13:05:33" "$rootfs/yuki"

	# Construct a manifest.
	sane_run gomtree -c -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	[ -f "$manifest" ]

	touch "$rootfs/aki"
	touch "$rootfs/yuki"

	umoci raw mtree-validate -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	sane_run gomtree validate --strict -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]

	# -k without time should result in no deltas.
	umoci raw mtree-validate -k sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	sane_run gomtree validate --strict -k sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]

	# -k with key not in manifest should error out.
	umoci raw mtree-validate -k sha1digest -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *'keyword "sha1digest"'* ]]
}

@test "umoci raw mtree-validate -K" {
	tmpdir="$(setup_tmpdir)"
	rootfs="$tmpdir/rootfs"
	manifest="$tmpdir/rootfs.mtree"

	# Create some dummy files.
	mkdir -p "$rootfs/foo/bar/baz"
	echo "foobar" >"$rootfs/foobar"
	echo "test" > "$rootfs/foo/bar/baz/boop"
	touch -d "1997-03-25T13:40:44" "$rootfs/aki"
	touch -d "2025-09-05T13:05:55" "$rootfs/yuki"

	# Construct a manifest.
	sane_run gomtree -c -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	[ -f "$manifest" ]

	# -K with already set keywords should just work.
	umoci raw mtree-validate -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]
	sane_run gomtree validate --strict -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -eq 0 ]

	# -K with key not in manifest should error out.
	umoci raw mtree-validate -K sha1digest -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *'keyword "sha1digest"'* ]]

	touch "$rootfs/aki"
	touch "$rootfs/yuki"

	umoci raw mtree-validate -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	sane_run gomtree validate --strict -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]

	# -K doesn't clear the set of   without time should result in no deltas.
	umoci raw mtree-validate -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
	sane_run gomtree validate --strict -K sha256digest -p "$rootfs" -f "$manifest"
	[ "$status" -ne 0 ]
}

# See the 0012trunc.sh test in <https://github.com/vbatts/go-mtree/pull/209>.
@test "umoci raw mtree-validate [tar_time]" {
	tmpdir="$(setup_tmpdir)"
	rootfs="$tmpdir/rootfs"
	time_manifest="$tmpdir/rootfs.mtree"
	tartime_manifest="$tmpdir/rootfs-tartime.mtree"

	# Create some dummy files.
	mkdir -p "$rootfs"
	date="2025-09-05T13:05:10"
	touch -d "$date.1000" "$rootfs/lowerhalf"
	touch -d "$date.8000" "$rootfs/upperhalf"
	touch -d "$date.0000" "$rootfs/tartime"

	keywords=type,uid,gid,nlink,link,mode,flags,xattr,size,sha256digest
	sane_run gomtree -c -k "$keywords,time" -p "$rootfs" -f "$time_manifest"
	[ "$status" -eq 0 ]
	[ -f "$time_manifest" ]
	sane_run gomtree -c -k "$keywords,tar_time" -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -eq 0 ]
	[ -f "$tartime_manifest" ]

	# Make sure tar_time truncated the value.
	unix="$(date -d "$date" +%s)"
	grep -Eq "lowerhalf.*\<tar_time=$unix\.0{9}\>" "$tartime_manifest"
	grep -Eq "upperhalf.*\<tar_time=$unix\.0{9}\>" "$tartime_manifest"
	grep -Eq "tartime.*\<tar_time=$unix\.0{9}\>" "$tartime_manifest"

	# Validation should work.
	umoci raw mtree-validate -p "$rootfs" -f "$time_manifest"
	[ "$status" -eq 0 ]
	umoci raw mtree-validate -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -eq 0 ]
	# Even if we force the usage of the opposite time type.
	umoci raw mtree-validate -k "$keywords,tar_time" -p "$rootfs" -f "$time_manifest"
	[ "$status" -eq 0 ]
	umoci raw mtree-validate -k "$keywords,time" -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -eq 0 ]

	# Manually truncate the mtime.
	touch -d "$date.0000" "$rootfs/lowerhalf"
	touch -d "$date.0000" "$rootfs/upperhalf"
	touch -d "$date.0000" "$rootfs/tartime"

	# Now only tar_time validation should work.
	umoci raw mtree-validate -p "$rootfs" -f "$time_manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *"lowerhalf"* ]]
	[[ "$output" == *"upperhalf"* ]]
	[[ "$output" != *"tartime"* ]] # no difference after truncation
	umoci raw mtree-validate -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -eq 0 ]
	# If we force the usage of the opposite time type, as long as one time type
	# is tar_time then validation should still work.
	umoci raw mtree-validate -k "$keywords,tar_time" -p "$rootfs" -f "$time_manifest"
	[ "$status" -eq 0 ]
	umoci raw mtree-validate -k "$keywords,time" -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -eq 0 ]

	# Modify the mtime completely.
	touch -d "1997-03-25T13:40:00.0000" "$rootfs/lowerhalf"
	touch -d "1997-03-25T13:40:00.0000" "$rootfs/upperhalf"
	touch -d "1997-03-25T13:40:00.0000" "$rootfs/tartime"

	# All validations should now fail.
	umoci raw mtree-validate -p "$rootfs" -f "$time_manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *"lowerhalf"* ]]
	[[ "$output" == *"upperhalf"* ]]
	[[ "$output" == *"tartime"* ]]
	umoci raw mtree-validate -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *"lowerhalf"* ]]
	[[ "$output" == *"upperhalf"* ]]
	[[ "$output" == *"tartime"* ]]
	# Even if we force the usage of the opposite time type.
	umoci raw mtree-validate -p "$rootfs" -f "$time_manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *"lowerhalf"* ]]
	[[ "$output" == *"upperhalf"* ]]
	[[ "$output" == *"tartime"* ]]
	umoci raw mtree-validate -p "$rootfs" -f "$tartime_manifest"
	[ "$status" -ne 0 ]
	[[ "$output" == *"lowerhalf"* ]]
	[[ "$output" == *"upperhalf"* ]]
	[[ "$output" == *"tartime"* ]]
}
