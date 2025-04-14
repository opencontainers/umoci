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
	#       generic.
	[ -e "$BUNDLE/rootfs/bin/sh" ]
	[ -e "$BUNDLE/rootfs/etc/passwd" ]
	[ -e "$BUNDLE/rootfs/etc/group" ]

	image-verify "${IMAGE}"
}

@test "umoci raw unpack [invalid arguments]" {
	ROOTFS="$(setup_tmpdir)"

	# Missing --image and bundle argument.
	umoci raw unpack
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	new_bundle_rootfs
	umoci raw unpack "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing bundle argument.
	umoci raw unpack --image="${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci raw unpack --image ":${TAG}" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci raw unpack --image "${IMAGE}-doesnotexist:${TAG}" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci raw unpack --image "${IMAGE}:" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci raw unpack --image "${IMAGE}:${TAG}-doesnotexist" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci raw unpack --image "${IMAGE}:${INVALID_TAG}" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci raw unpack --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$ROOTFS"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci raw unpack --image "${IMAGE}:${TAG}" "$ROOTFS" \
		this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	! [ -d this-is-an-invalid-argument ]
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
	image-verify "${IMAGE}"

	# Ensure that gomtree succeeds on the new unpacked rootfs.
	gomtree -p "$ROOTFS_B" -f "$BUNDLE_A"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	image-verify "${IMAGE}"
}
