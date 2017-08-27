#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016, 2017 SUSE LLC.
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
	touch "$BUNDLE_A/setuid"  && chmod u+xs  "$BUNDLE_A/setuid"
	touch "$BUNDLE_A/setgid"  && chmod g+xs  "$BUNDLE_A/setgid"
	touch "$BUNDLE_A/setugid" && chmod ug+xs "$BUNDLE_A/setugid"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Check that the set{uid,gid} bits were preserved.
	[ -u "$BUNDLE_A/setuid" ]
	[ -g "$BUNDLE_A/setgid" ]
	[ -u "$BUNDLE_A/setugid" ] && [ -g "$BUNDLE_A/setugid" ]

	image-verify "${IMAGE}"
}
