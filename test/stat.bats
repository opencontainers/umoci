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

@test "umoci stat --json" {
	image-verify "${IMAGE}"

	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]

	statFile="$(setup_tmpdir)/stat"
	echo "$output" > "$statFile"

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
	image-verify "${IMAGE}"

	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	# We should have some history information.
	echo "$output" | grep 'LAYER'
	echo "$output" | grep 'CREATED'
	echo "$output" | grep 'CREATED BY'
	echo "$output" | grep 'SIZE'
	echo "$output" | grep 'COMMENT'

	image-verify "${IMAGE}"
}

@test "umoci stat [missing args]" {
	umoci stat
	[ "$status" -ne 0 ]
}

# TODO: Add a test to make sure that empty_layer and layer are mutually
#       exclusive. Unfortunately, jq doesn't provide an XOR operator...
