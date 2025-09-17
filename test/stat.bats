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

function digest_to_path() {
	layout="$1"
	digest="$2"
	echo "$1/blobs/$(tr : / <<<"$2")"
}

@test "umoci stat --json" {
	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]

	statFile="$(setup_tmpdir)/stat"
	echo "$output" > "$statFile"

	# .config.descriptor should describe a config blob
	sane_run jq -SMr '.config.descriptor.mediaType' "$statFile"
	[ "$status" -eq 0 ]
	[[ "$output" == "application/vnd.oci.image.config.v1+json" ]]

	# .config.blob should match .config.descriptor data
	sane_run jq -SMr '.config.descriptor.digest' "$statFile"
	[ "$status" -eq 0 ]
	config_digest="$output"
	sane_run jq -SMr '.config.blob' "$statFile"
	[ "$status" -eq 0 ]
	config_data="$output"
	sane_run jq -SMr '.' "$(digest_to_path "$IMAGE" "$config_digest")"
	[ "$status" -eq 0 ]
	[[ "$output" == "$config_data" ]]

	# .history should have at least one entry.
	sane_run jq -SMr '.history | length' "$statFile"
	[ "$status" -eq 0 ]
	[ "$output" -ge 1 ]

	# There should be at least one non-empty_layer.
	sane_run jq -SMr '[.history[] | .empty_layer == null] | any' "$statFile"
	[ "$status" -eq 0 ]
	[[ "$output" == "true" ]]

	image-verify "${IMAGE}"
}

# We can't really test the output for non-JSON output, but we can smoke test it.
@test "umoci stat [smoke]" {
	# Set some values to make sure they show up in stat properly.
	umoci config --config.user "foobar" --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	# We should have some config information.
	echo "$output" | grep "== CONFIG =="
	echo "$output" | grep "Media Type: application/vnd.oci.image.config.v1+json"
	echo "$output" | grep "User: foobar"

	# We should have some history information.
	echo "$output" | grep "== HISTORY =="
	echo "$output" | grep 'LAYER'
	echo "$output" | grep 'CREATED'
	echo "$output" | grep 'CREATED BY'
	echo "$output" | grep 'SIZE'
	echo "$output" | grep 'COMMENT'

	image-verify "${IMAGE}"
}

BLANK_IMAGE_STAT="$(cat <<EOF
== CONFIG ==
Created: 2025-09-05T13:05:10.12345+10:00
Author: ""
Image Config:
	User: ""
	Command:
Descriptor:
	Media Type: application/vnd.oci.image.config.v1+json
	Digest: sha256:e5101a46118c740a7709af8eaeec19cbc50a567f4fe7741f8420af39a3779a77
	Size: 135B

== HISTORY ==
LAYER CREATED CREATED BY SIZE COMMENT
EOF
)"

@test "umoci stat [blank image output snapshot]" {
	IMAGE="$(setup_tmpdir)/image" TAG="latest"
	STATFILE_DIR="$(setup_tmpdir)"

	expected="${STATFILE_DIR}/expected"
	cat >"$expected" <<<"$BLANK_IMAGE_STAT"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	umoci config --no-history --created="2025-09-05T13:05:10.12345+10:00" --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	got="${STATFILE_DIR}/got"
	cat >"$got" <<<"$output"

	sane_run diff -u "$expected" "$got"
	[ "$status" -eq 0 ]
}

@test "umoci stat [invalid arguments]" {
	# Missing --image argument.
	umoci stat
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci stat --image "${IMAGE}:${TAG}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci stat --image ":${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci stat --image "${IMAGE}-doesnotexist:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci stat --image "${IMAGE}:"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci stat --image "${IMAGE}:${TAG}-doesnotexist"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci stat --image "${IMAGE}:${INVALID_TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci stat --this-is-an-invalid-argument --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci stat --image "${IMAGE}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

# TODO: Add a test to make sure that empty_layer and layer are mutually
#	   exclusive. Unfortunately, jq doesn't provide an XOR operator...
