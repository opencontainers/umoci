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

@test "umoci unpack" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure these files properly exist.
	[ -f "$BUNDLE/config.json" ]
	[ -d "$ROOTFS" ]

	# Check that the image appears about right.
	# NOTE: Since we could be using different images, this will be fairly
	#	   generic.
	[ -e "$ROOTFS/bin/sh" ]
	[ -e "$ROOTFS/etc/passwd" ]
	[ -e "$ROOTFS/etc/group" ]

	# Ensure that gomtree succeeds on the unpacked bundle.
	gomtree -p "$ROOTFS" -f "$BUNDLE"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Make sure that unpack fails without a bundle path.
	umoci unpack --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	# ... or with too many
	umoci unpack --image "${IMAGE}:${TAG}" too many arguments
	[ "$status" -ne 0 ]
	! [ -d too ]
	! [ -d many ]
	! [ -d arguments ]

	image-verify "${IMAGE}"
}

@test "umoci unpack [invalid arguments]" {
	# Missing --image and bundle argument.
	umoci unpack
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	new_bundle_rootfs
	umoci unpack "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing bundle argument.
	umoci unpack --image="${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci unpack --image ":${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci unpack --image "${IMAGE}-doesnotexist:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci unpack --image "${IMAGE}:" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci unpack --image "${IMAGE}:${INVALID_TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci unpack --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE" \
		this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci unpack [config.json contains mount namespace]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that we have a mount namespace enabled.
	sane_run jq -SM 'any(.linux.namespaces[] | .type; . == "mount")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "true" ]]

	image-verify "${IMAGE}"
}

@test "umoci unpack [consistent results]" {
	# Unpack the image.
	new_bundle_rootfs && BUNDLE_A="$BUNDLE" ROOTFS_A="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Wait a beat.
	sleep 5s

	# Unpack it again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE" ROOTFS_B="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that gomtree cross-succeeds.
	gomtree -p "$ROOTFS_A" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
	gomtree -p "$ROOTFS_B" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack [setuid]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some files setuid and setgid.
	touch "$ROOTFS/setuid"  && chmod u+xs  "$ROOTFS/setuid"
	touch "$ROOTFS/setgid"  && chmod g+xs  "$ROOTFS/setgid"
	touch "$ROOTFS/setugid" && chmod ug+xs "$ROOTFS/setugid"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check that the set{uid,gid} bits were preserved.
	[ -u "$ROOTFS/setuid" ]
	[ -g "$ROOTFS/setgid" ]
	[ -u "$ROOTFS/setugid" ] && [ -g "$ROOTFS/setugid" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack [setcap]" {
	# We need to setcap which requires root on quite a few kernels -- and we
	# don't support v3 capabilities yet (which allow us as an unprivileged user
	# to write capabilities).
	requires root

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some files setuid and setgid.
	touch "$ROOTFS/setcap1" && setcap "cap_net_raw+eip" "$ROOTFS/setcap1"
	touch "$ROOTFS/setcap2" && setcap "cap_sys_admin,cap_setfcap+eip" "$ROOTFS/setcap2"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image (as root).
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the capability bits were preserved.
	sane_run getcap "$ROOTFS/setcap1"
	[ "$status" -eq 0 ]
	[[ "$output" == *" cap_net_raw=eip"* ]]
	sane_run getcap "$ROOTFS/setcap2"
	[ "$status" -eq 0 ]
	[[ "$output" == *" cap_sys_admin,cap_setfcap=eip"* ]]

	# Unpack the image (as rootless).
	new_bundle_rootfs
	umoci unpack --rootless --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# TODO: Actually set capabilities as an unprivileged user and then test
	#	   that the correct v3 capabilities were set.

	image-verify "${IMAGE}"
}

@test "umoci unpack [mknod]" {
	# We need to mknod which requires root on most kernels. Since Linux 4.18 it's
	# been possible for unprivileged users to mknod(2) but we can't use that here
	# (it requires owning the filesystem's superblock).
	requires root

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make some mknod.
	mknod "$ROOTFS/block1" b 128 42  # 80:2a 61a4
	mknod "$ROOTFS/block2" b 255 128 # ff:80 61a4
	mknod "$ROOTFS/char1"  c 133 37  # 85:25 21a4
	mknod "$ROOTFS/char2"  c 253 97  # fd:61 21a4
	mkfifo "$ROOTFS/fifo"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image (as root).
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check that all of the bits were preserved.
	[ -b "$ROOTFS/block1" ]
	[[ "$(stat -c '%t:%T' "$ROOTFS/block1")" == *"80:2a"* ]]
	[ -b "$ROOTFS/block2" ]
	[[ "$(stat -c '%t:%T' "$ROOTFS/block2")" == *"ff:80"* ]]
	[ -c "$ROOTFS/char1" ]
	[[ "$(stat -c '%t:%T' "$ROOTFS/char1")" == *"85:25"* ]]
	[ -c "$ROOTFS/char2" ]
	[[ "$(stat -c '%t:%T' "$ROOTFS/char2")" == *"fd:61"* ]]
	[ -p "$ROOTFS/fifo" ]

	# Unpack the image (as rootless).
	new_bundle_rootfs
	umoci unpack --rootless --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# At the least, check that the files exist.
	[ -e "$ROOTFS/block1" ]
	[ -e "$ROOTFS/block2" ]
	[ -e "$ROOTFS/char1" ]
	[ -e "$ROOTFS/char2" ]
	# But the FIFOs should be preserved.
	[ -p "$ROOTFS/fifo" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack --keep-dirlinks" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create some links for us to play with in the next layer.
	mkdir "$ROOTFS/dir"
	touch "$ROOTFS/dir/a"
	ln -s dir "$ROOTFS/link"
	ln -s link "$ROOTFS/link2"
	ln -s loop2 "$ROOTFS/loop1"
	ln -s loop3 "$ROOTFS/loop2"
	ln -s link2/loop4 "$ROOTFS/loop3"
	ln -s ../loop1 "$ROOTFS/dir/loop4"
	chmod 000 "$ROOTFS/dir"

	# Repack the image.
	umoci repack --refresh-bundle --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "$IMAGE"

	# Create a fake rootfs which contains entries inside symlinks.
	ROOTFS="$(setup_tmpdir)"
	mkdir "$ROOTFS/link"  # == /dir
	touch "$ROOTFS/link/b"
	mkdir "$ROOTFS/link2"  # == /link == /dir
	touch "$ROOTFS/link2/c"
	mkdir "$ROOTFS/loop1" # == /loop{1..4} ... (symlink loop)
	touch "$ROOTFS/loop1/broken"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer1.tar" "$ROOTFS" .
	[ "$status" -eq 0 ]

	# Insert our fake layer manually.
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR/layer1.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack our weird image.
	new_bundle_rootfs
	umoci unpack --keep-dirlinks --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Resolution of links without destroying the links themselves.
	chmod 755 "$ROOTFS/dir"
	[ -f "$ROOTFS/dir/a" ]
	[ -f "$ROOTFS/dir/b" ]
	[ -f "$ROOTFS/dir/c" ]
	[ -L "$ROOTFS/link" ]
	[ -L "$ROOTFS/link2" ]
	[ "$(readlink "$ROOTFS/link")" = "dir" ]
	[ "$(readlink "$ROOTFS/link2")" = "link" ]
	# ... but symlink loops have to be broken.
	[ -d "$ROOTFS/loop1" ]
	[ -f "$ROOTFS/loop1/broken" ]
	[ -L "$ROOTFS/loop2" ]
	[ -L "$ROOTFS/loop3" ]
	[ -L "$ROOTFS/dir/loop4" ]
	[ "$(readlink "$ROOTFS/loop2")" = "loop3" ]
	[ "$(readlink "$ROOTFS/loop3")" = "link2/loop4" ]
	[ "$(readlink "$ROOTFS/dir/loop4")" = "../loop1" ]
}

@test "umoci unpack [mixed compression]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Create a few layers with different compression algorithms.

	# zstd layer
	touch "$ROOTFS/zstd1"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=zstd "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# gzip layer
	touch "$ROOTFS/gzip1"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=gzip "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# plain layer
	touch "$ROOTFS/plain1"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=none "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# zstd layer
	touch "$ROOTFS/zstd2"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=zstd "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# plain layer
	touch "$ROOTFS/plain2"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle --compress=none "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# zstd layer (auto)
	touch "$ROOTFS/zstd3"
	umoci repack --image "${IMAGE}:${TAG}" --refresh-bundle "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Re-extract the latest image and make sure all of the files were correctly
	# extracted.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	[ -f "$ROOTFS/zstd1" ]
	[ -f "$ROOTFS/gzip1" ]
	[ -f "$ROOTFS/plain1" ]
	[ -f "$ROOTFS/zstd2" ]
	[ -f "$ROOTFS/plain2" ]
	[ -f "$ROOTFS/zstd3" ]
}
