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

@test "unpack [consistent results]" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Wait a beat.
	sleep 5s

	# Unpack it again.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Ensure that gomtree suceeds on the new unpacked bundle.
	gomtree -p "$BUNDLE_B/rootfs" -f "$BUNDLE_A"/sha256:*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}
