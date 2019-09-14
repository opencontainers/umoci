#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2019 SUSE LLC.
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

@test "umoci raw unpack" {
	# Unpack the image.
	new_bundle_rootfs
	umoci raw unpack --image "${IMAGE}:${TAG}" "$ROOTFS"
	[ "$status" -eq 0 ]

	# We need to make sure these files *do not* exist.
	! [ -f "$BUNDLE/config.json" ]
	[ -d "$ROOTFS" ]

	# Check that the image appears about right.
	# NOTE: Since we could be using different images, this will be fairly
	#	   generic.
	[ -e "$BUNDLE/rootfs/bin/sh" ]
	[ -e "$BUNDLE/rootfs/etc/passwd" ]
	[ -e "$BUNDLE/rootfs/etc/group" ]

	image-verify "${IMAGE}"
}

@test "umoci raw unpack [missing args]" {
	ROOTFS="$(setup_tmpdir)"

	umoci raw unpack --image="${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	umoci raw unpack "$ROOTFS"
	[ "$status" -ne 0 ]
}

@test "umoci raw unpack [too many args]" {
	umoci raw unpack --image "${IMAGE}:${TAG}" too many arguments
	[ "$status" -ne 0 ]
	! [ -d too ]
	! [ -d many ]
	! [ -d arguments ]
}

@test "umoci raw unpack [cross-check with umoci unpack]" {
	# Unpack the bundle
	BUNDLE_A="$(setup_tmpdir)"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Unpack the rootfs
	BUNDLE_B="$(setup_tmpdir)" && ROOTFS_B="$BUNDLE_B/rootfs"
	umoci raw unpack --image "${IMAGE}:${TAG}" "$ROOTFS_B"
	[ "$status" -eq 0 ]

	# Ensure that gomtree suceeds on the new unpacked rootfs.
	gomtree -p "$ROOTFS_B" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	image-verify "${IMAGE}"
}
