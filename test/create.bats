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

@test "umoci init [missing args]" {
	umoci init
	[ "$status" -ne 0 ]
}

@test "umoci init --layout [empty]" {
	# Setup up $NEWIMAGE.
	NEWIMAGE="$(setup_tmpdir)"
	rm -rf "$NEWIMAGE"

	# Create a new image with no tags.
	umoci init --layout "$NEWIMAGE"
	[ "$status" -eq 0 ]
	image-verify "$NEWIMAGE"

	# Make sure that there's no references or blobs.
	sane_run find "$NEWIMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 0 ]
	# Note that this is _very_ dodgy at the moment because of how complicated
	# the reference handling is now.
	# XXX: Make sure to update this for 1.0.0-rc6 where the refname changed.
	sane_run jq -SMr '.manifests[]? | .annotations["org.opencontainers.ref.name"] | strings' "$NEWIMAGE/index.json"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 0 ]

	# Make sure that the required files exist.
	[ -f "$NEWIMAGE/oci-layout" ]
	[ -d "$NEWIMAGE/blobs" ]
	[ -d "$NEWIMAGE/blobs/sha256" ]
	[ -f "$NEWIMAGE/index.json" ]

	# Make sure that attempting to create a new image will fail.
	umoci init --layout "$NEWIMAGE"
	[ "$status" -ne 0 ]
	image-verify "$NEWIMAGE"

	image-verify "$NEWIMAGE"
}

@test "umoci new [missing args]" {
	umoci new
	[ "$status" -ne 0 ]
}

@test "umoci new --image" {
	BUNDLE="$(setup_tmpdir)"

	# Setup up $NEWIMAGE.
	NEWIMAGE="$(setup_tmpdir)"
	rm -rf "$NEWIMAGE"

	# Create a new image with no tags.
	umoci init --layout "$NEWIMAGE"
	[ "$status" -eq 0 ]
	image-verify "$NEWIMAGE"

	# Create a new image with another tag.
	umoci new --image "${NEWIMAGE}:latest"
	[ "$status" -eq 0 ]
	# XXX: oci-image-tool validate doesn't like empty images (without layers)
	#image-verify "$NEWIMAGE"

	# Modify the config.
	umoci config --image "${NEWIMAGE}" --config.user "1234:1332"
	[ "$status" -eq 0 ]
	# XXX: oci-image-tool validate doesn't like empty images (without layers)
	#image-verify "$NEWIMAGE"

	# Unpack the image.
	umoci unpack --image "${NEWIMAGE}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the rootfs is empty.
	sane_run find "$BUNDLE/rootfs"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 1 ]

	# Make sure that the config applied.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1234 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1332 ]

	# Make sure additionalGids were not set.
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	# Check that the history looks sane.
	umoci stat --image "${NEWIMAGE}" --json
	[ "$status" -eq 0 ]
	# There should be no non-empty_layers.
	[[ "$(echo "$output" | jq -SM '[.history[] | .empty_layer == null] | any')" == "false" ]]

	# XXX: oci-image-tool validate doesn't like empty images (without layers)
	#image-verify "$NEWIMAGE"
}
