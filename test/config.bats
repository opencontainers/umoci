#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016 SUSE LLC.
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

BUNDLE_A="$BATS_TMPDIR/bundle.a"
BUNDLE_B="$BATS_TMPDIR/bundle.b"

function setup() {
	setup_image
}

function teardown() {
	teardown_image
	rm -rf "$BUNDLE_A"
	rm -rf "$BUNDLE_B"
}

@test "umoci config" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Make sure that the config was unchanged.
	# First clean the config.
	jq -SM '.' "$BUNDLE_A/config.json" >"$BATS_TMPDIR/a-config.json"
	jq -SM '.' "$BUNDLE_B/config.json" >"$BATS_TMPDIR/b-config.json"
	sane_run diff -u "$BATS_TMPDIR/a-config.json" "$BATS_TMPDIR/b-config.json"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "umoci config --config.user [numeric]" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.user="1337:8888"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 8888 ]
}

# TODO: Add a test to make sure that --config.user is resolved on unpacking.
# TODO: Add further tests for --config.user resolution (and additional_gids).

@test "umoci config --config.workingdir" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.workingdir "/a/fake/directory"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.cwd' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" = '"/a/fake/directory"' ]
}

@test "umoci config --clear=config.env" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --clear=config.env
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure that only $HOME was set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"HOME="* ]]
	[ "${#lines[@]}" -eq 1 ]
}

@test "umoci config --config.env" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.env "VARIABLE1=test" --config.env "VARIABLE2=what"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Set the variables.
	export $output
	[[ "$VARIABLE1" == "test" ]]
	[[ "$VARIABLE2" == "what" ]]
}

@test "umoci config --config.memory.*" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.memory.limit 1000 --config.memory.swap 2000
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure memory.limit and memory.reservation are set.
	sane_run jq -SMr '.linux.resources.memory.limit' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1000 ]
	sane_run jq -SMr '.linux.resources.memory.reservation' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1000 ]

	# Make sure memory.swap was set.
	sane_run jq -SMr '.linux.resources.memory.swap' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2000 ]
}

@test "umoci config --config.cpu.shares" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.cpu.shares 1024
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure memory.limit and memory.reservation are set.
	sane_run jq -SMr '.linux.resources.cpu.shares' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1024 ]
}

# TODO: Something about volume.
