#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016, 2017, 2018 SUSE LLC.
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
	setup_image
}

function teardown() {
	teardown_tmpdirs
	teardown_image
}

@test "umoci unpack" {
	BUNDLE="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure these files properly exist.
	[ -f "$BUNDLE/config.json" ]
	[ -d "$BUNDLE/rootfs" ]

	# Check that the image appears about right.
	# NOTE: Since we could be using different images, this will be fairly
	#       generic.
	[ -e "$BUNDLE/rootfs/bin/sh" ]
	[ -e "$BUNDLE/rootfs/etc/passwd" ]
	[ -e "$BUNDLE/rootfs/etc/group" ]

	# Ensure that gomtree suceeds on the unpacked bundle.
	gomtree -p "$BUNDLE/rootfs" -f "$BUNDLE"/sha256_*.mtree
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

@test "umoci unpack [missing args]" {
	BUNDLE="$(setup_tmpdir)"

	umoci unpack --image="${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	umoci unpack "$BUNDLE"
	[ "$status" -ne 0 ]
}

@test "umoci unpack [config.json contains mount namespace]" {
	BUNDLE="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
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
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Wait a beat.
	sleep 5s

	# Unpack it again.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that gomtree suceeds on the new unpacked bundle.
	gomtree -p "$BUNDLE_B/rootfs" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack [setuid]" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make some files setuid and setgid.
	touch "$BUNDLE_A/rootfs/setuid"  && chmod u+xs  "$BUNDLE_A/rootfs/setuid"
	touch "$BUNDLE_A/rootfs/setgid"  && chmod g+xs  "$BUNDLE_A/rootfs/setgid"
	touch "$BUNDLE_A/rootfs/setugid" && chmod ug+xs "$BUNDLE_A/rootfs/setugid"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Check that the set{uid,gid} bits were preserved.
	[ -u "$BUNDLE_B/rootfs/setuid" ]
	[ -g "$BUNDLE_B/rootfs/setgid" ]
	[ -u "$BUNDLE_B/rootfs/setugid" ] && [ -g "$BUNDLE_B/rootfs/setugid" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack [setcap]" {
	# We need to setcap which requires root on quite a few kernels -- and we
	# don't support v3 capabilities yet (which allow us as an unprivileged user
	# to write capabilities).
	requires root

	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make some files setuid and setgid.
	touch "$BUNDLE_A/rootfs/setcap1" && setcap "cap_net_raw+eip" "$BUNDLE_A/rootfs/setcap1"
	touch "$BUNDLE_A/rootfs/setcap2" && setcap "cap_sys_admin,cap_setfcap+eip" "$BUNDLE_A/rootfs/setcap2"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image (as root).
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Ensure that the capability bits were preserved.
	sane_run getcap "$BUNDLE_B/rootfs/setcap1"
	[ "$status" -eq 0 ]
	[[ "$output" == *" = cap_net_raw+eip"* ]]
	sane_run getcap "$BUNDLE_B/rootfs/setcap2"
	[ "$status" -eq 0 ]
	[[ "$output" == *" = cap_sys_admin,cap_setfcap"* ]]

	# Unpack the image (as rootless).
	umoci unpack --rootless --image "${IMAGE}:${TAG}" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# TODO: Actually set capabilities as an unprivileged user and then test
	#       that the correct v3 capabilities were set.

	image-verify "${IMAGE}"
}

@test "umoci unpack [mknod]" {
	# We need to mknod which requires root on most kernels. Since Linux 4.18 it's
	# been possible for unprivileged users to mknod(2) but we can't use that here
	# (it requires owning the filesystem's superblock).
	requires root

	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Make some mknod.
	mknod "$BUNDLE_A/rootfs/block1" b 128 42  # 80:2a 61a4
	mknod "$BUNDLE_A/rootfs/block2" b 255 128 # ff:80 61a4
	mknod "$BUNDLE_A/rootfs/char1"  c 133 37  # 85:25 21a4
	mknod "$BUNDLE_A/rootfs/char2"  c 253 97  # fd:61 21a4
	mkfifo "$BUNDLE_A/rootfs/fifo"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image (as root).
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Check that all of the bits were preserved.
	[ -b "$BUNDLE_B/rootfs/block1" ]
	[[ "$(stat -c '%t:%T' "$BUNDLE_B/rootfs/block1")" == *"80:2a"* ]]
	[ -b "$BUNDLE_B/rootfs/block2" ]
	[[ "$(stat -c '%t:%T' "$BUNDLE_B/rootfs/block2")" == *"ff:80"* ]]
	[ -c "$BUNDLE_B/rootfs/char1" ]
	[[ "$(stat -c '%t:%T' "$BUNDLE_B/rootfs/char1")" == *"85:25"* ]]
	[ -c "$BUNDLE_B/rootfs/char2" ]
	[[ "$(stat -c '%t:%T' "$BUNDLE_B/rootfs/char2")" == *"fd:61"* ]]
	[ -p "$BUNDLE_B/rootfs/fifo" ]

	# Unpack the image (as rootless).
	umoci unpack --rootless --image "${IMAGE}:${TAG}" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# At the least, check that the files exist.
	[ -e "$BUNDLE_C/rootfs/block1" ]
	[ -e "$BUNDLE_C/rootfs/block2" ]
	[ -e "$BUNDLE_C/rootfs/char1" ]
	[ -e "$BUNDLE_C/rootfs/char2" ]
	# But the FIFOs should be preserved.
	[ -p "$BUNDLE_C/rootfs/fifo" ]

	image-verify "${IMAGE}"
}

@test "umoci unpack --keep-dirlinks" {
    BUNDLE="$(setup_tmpdir)"

    image-verify "${IMAGE}"
    umoci unpack --image "${IMAGE}:${TAG}" "${BUNDLE}"
    mkdir "${BUNDLE}/rootfs/dir"
    touch "${BUNDLE}/rootfs/dir/a"
    ln -s dir "${BUNDLE}/rootfs/link"
    ln -s link "${BUNDLE}/rootfs/link2"
    ln -s loop2 "${BUNDLE}/rootfs/loop1"
    ln -s loop1 "${BUNDLE}/rootfs/loop2"
    umoci repack --refresh-bundle --image "${IMAGE}:${TAG}" "${BUNDLE}"
	[ "$status" -eq 0 ]

    rm "${BUNDLE}/rootfs/link"
    mkdir "${BUNDLE}/rootfs/link"
    touch "${BUNDLE}/rootfs/link/b"
    umoci repack --refresh-bundle --image "${IMAGE}:${TAG}" "${BUNDLE}"
	[ "$status" -eq 0 ]

    chmod -R 0777 "${BUNDLE}"
    rm -rf "${BUNDLE}"
    umoci unpack --keep-dirlinks --image "${IMAGE}:${TAG}" "${BUNDLE}"
    [ "$status" -eq 0 ]
    bundle-verify "${BUNDLE}"

    ls -al "${BUNDLE}/rootfs"
    echo "${output}"

    [ -f "${BUNDLE}/rootfs/dir/a" ]
    [ -f "${BUNDLE}/rootfs/dir/b" ]
    [ -L "${BUNDLE}/rootfs/link" ]
    [ -L "${BUNDLE}/rootfs/link2" ]
    [ "$(readlink ${BUNDLE}/rootfs/link)" = "dir" ]
    [ "$(readlink ${BUNDLE}/rootfs/link2)" = "link" ]
}
