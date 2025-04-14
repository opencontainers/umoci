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

@test "umoci unpack --uid-map --gid-map" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:1337:65535" --gid-map "0:8888:65535" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure the config exists.
	[ -f "$BUNDLE/config.json" ]

	# Check that all of the files have a UID owner >=1337 and a GID owner >=8888.
	find "$ROOTFS" | xargs stat -c '%u:%g' | awk -F: '{
		uid = $1;
		if (uid < 1337 || uid >= 1337 + 65535)
			exit 1;
		gid = $2;
		if (gid < 8888 || gid >= 8888 + 65535)
			exit 1;
	}'

	# Unpack the image with a differen uid and gid mapping.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:8080:65535" --gid-map "0:7777:65535" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure the config exists.
	[ -f "$BUNDLE/config.json" ]

	# Check that all of the files have a UID owner >=8080 and a GID owner >=7777.
	find "$ROOTFS" | xargs stat -c '%u:%g' | awk -F: '{
		uid = $1;
		if (uid < 8080 || uid >= 8080 + 65535)
			exit 1;
		gid = $2;
		if (gid < 7777 || gid >= 7777 + 65535)
			exit 1;
	}'

	image-verify "${IMAGE}"
}

@test "umoci repack [--uid-map --gid-map]" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:1337:65535" --gid-map "0:7331:65535" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure the config exists.
	[ -f "$BUNDLE/config.json" ]

	# Create a new file with a remapped owner.
	echo "new file" > "$ROOTFS/new test file "
	chown "2000:8000" "$ROOTFS/new test file "

	# Repack the image using the same mapping.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again with a different mapping.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" --uid-map "0:4000:65535" --gid-map "0:4000:65535" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the test file is different.
	sane_run stat -c '%u:%g' "$ROOTFS/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337 + 4000)):$((8000 - 7331 + 4000))" ]]

	# Redo the unpacking with no mapping.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the test file was unpacked properly.
	sane_run stat -c '%u:%g' "$ROOTFS/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337)):$((8000 - 7331))" ]]

	image-verify "${IMAGE}"
}

@test "umoci {un,re}pack --rootless [user.rootlesscontainers]" {
	# While we forcefully use --rootless, we also change the owner of files.
	requires root

	# Root-ful unpack first to create non-root files.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create some files with non-root owners.
	touch "$ROOTFS/owner_a" && chown 992:123 "$ROOTFS/owner_a"
	touch "$ROOTFS/owner_b" && chown   0:456 "$ROOTFS/owner_b"
	touch "$ROOTFS/owner_c" && chown  98:0   "$ROOTFS/owner_c"
	touch "$ROOTFS/owner_d" && chown 492:218 "$ROOTFS/owner_d"
	touch "$ROOTFS/owner_e" && chown 123:456 "$ROOTFS/owner_e"

	# Repack.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Rootless unpack to test user.rootlesscontainers.
	new_bundle_rootfs
	umoci unpack --rootless --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Now check that the owners are all the current owner.
	[ -O "$ROOTFS/owner_a" ] && [ -G "$ROOTFS/owner_a" ]
	[ -O "$ROOTFS/owner_b" ] && [ -G "$ROOTFS/owner_b" ]
	[ -O "$ROOTFS/owner_c" ] && [ -G "$ROOTFS/owner_c" ]
	[ -O "$ROOTFS/owner_d" ] && [ -G "$ROOTFS/owner_d" ]
	[ -O "$ROOTFS/owner_e" ] && [ -G "$ROOTFS/owner_e" ]

	# Check the "user.rootlesscontainers" values against known values (this may
	# break if the rootlesscontainers.proto changes in the future -- so keep
	# this in mind if the tests start failing).
	# NOTE: We use getfattr(1) here rather than xattr(1) because getfattr(1)
	#	   actually can handle binary xattrs -- while xattr(1) just removes
	#	   the values.
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_a"
	[ "$status" -eq 0 ]
	[[ "$output" == "0x08e007107b" ]] # 992:123
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_b"
	[ "$status" -eq 0 ]
	[[ "$output" == "0x08ffffffff0f10c803" ]] # noop:456
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_c"
	[ "$status" -eq 0 ]
	[[ "$output" == "0x086210ffffffff0f" ]] # 98:noop
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_d"
	[ "$status" -eq 0 ]
	[[ "$output" == "0x08ec0310da01" ]] # 492:218
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_e"
	[ "$status" -eq 0 ]
	[[ "$output" == "0x087b10c803" ]] # 123:456

	# Changing it should affect the second unpack. This is a pre-computed value
	# equal to "3195:2318" serialised as a protobuf payload.
	setfattr -n "user.rootlesscontainers" -v "0x08fb18108e12" "$ROOTFS/owner_d"
	# Removing it should make it be owned by root.
	setfattr -x "user.rootlesscontainers" "$ROOTFS/owner_e"

	# Repack.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Root-ful unpack again to check the changes.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check the owners. a...c should be unchanged ...
	sane_run stat -c '%u:%g' "$ROOTFS/owner_a"
	[ "$status" -eq 0 ]
	[[ "$output" == "992:123" ]]
	sane_run stat -c '%u:%g' "$ROOTFS/owner_b"
	[ "$status" -eq 0 ]
	[[ "$output" == "0:456" ]]
	sane_run stat -c '%u:%g' "$ROOTFS/owner_c"
	[ "$status" -eq 0 ]
	[[ "$output" == "98:0" ]]
	# ... while d...e will be modified.
	sane_run stat -c '%u:%g' "$ROOTFS/owner_d"
	[ "$status" -eq 0 ]
	[[ "$output" == "3195:2318" ]]
	sane_run stat -c '%u:%g' "$ROOTFS/owner_e"
	[ "$status" -eq 0 ]
	[[ "$output" == "0:0" ]]

	# Make sure we don't have any user.rootlesscontainers xattrs now (they
	# shouldn't be exposed to users or added to the tar layers).
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_a"
	[ "$status" -ne 0 ]
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_b"
	[ "$status" -ne 0 ]
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_c"
	[ "$status" -ne 0 ]
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_d"
	[ "$status" -ne 0 ]
	sane_run _getfattr user.rootlesscontainers "$ROOTFS/owner_e"
	[ "$status" -ne 0 ]

	image-verify "${IMAGE}"
}
