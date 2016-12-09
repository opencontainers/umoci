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

function setup() {
	setup_image
}

function teardown() {
	teardown_image
}

# FIXME: This test is __WAY__ too slow.
@test "umoci unpack --uid-map --gid-map" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	image-verify "$IMAGE"

	BUNDLE_A="$(setup_bundle)"
	BUNDLE_B="$(setup_bundle)"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A" --uid-map "1337:0:65535" --gid-map "8888:0:65535"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Check that all of the files have a UID owner >=1337 and a GID owner >=8888.
	find "$BUNDLE_A/rootfs" -exec stat -c '%u:%g' {} \; | while read -r line; do
		uid=$(echo "$line" | cut -d: -f1)
		gid=$(echo "$line" | cut -d: -f2)
		[ "$uid" -ge 1337 ] && [ "$uid" -lt "$((1337 + 65535))" ]
		[ "$gid" -ge 8888 ] && [ "$gid" -lt "$((8888 + 65535))" ]
	done

	# Unpack the image with a differen uid and gid mapping.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_B" --uid-map "8080:0:65535" --gid-map "7777:0:65535"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_B/config.json" ]

	# Check that all of the files have a UID owner >=8080 and a GID owner >=7777.
	find "$BUNDLE_B/rootfs" -exec stat -c '%u:%g' {} \; | while read -r line; do
		uid=$(echo "$line" | cut -d: -f1)
		gid=$(echo "$line" | cut -d: -f2)
		[ "$uid" -ge 8080 ] && [ "$uid" -lt "$((8080 + 65535))" ]
		[ "$gid" -ge 7777 ] && [ "$gid" -lt "$((7777 + 65535))" ]
	done

	image-verify "$IMAGE"
}

# FIXME: It would be nice if we implemented this test with a manual chown.
@test "umoci repack [with unpack --uid-map --gid-map]" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	image-verify "$IMAGE"

	BUNDLE_A="$(setup_bundle)"
	BUNDLE_B="$(setup_bundle)"
	BUNDLE_C="$(setup_bundle)"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A" --uid-map "1337:0:65535" --gid-map "7331:0:65535"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Create a new file with a remapped owner.
	echo "new file" > "$BUNDLE_A/rootfs/new test file "
	chown "2000:8000" "$BUNDLE_A/rootfs/new test file "

	# Repack the image using the same mapping.
	umoci repack --image "$IMAGE" --bundle "$BUNDLE_A" --tag "${TAG}-new"
	[ "$status" -eq 0 ]
	image-verify "$IMAGE"

	# Unpack it again with a different mapping.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_B" --uid-map "4000:0:65535" --gid-map "4000:0:65535"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Make sure that the test file is different.
	sane_run stat -c '%u:%g' "$BUNDLE_B/rootfs/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337 + 4000)):$((8000 - 7331 + 4000))" ]]

	# Redo the unpacking with no mapping.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Make sure that the test file was unpacked properly.
	sane_run stat -c '%u:%g' "$BUNDLE_C/rootfs/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337)):$((8000 - 7331))" ]]

	image-verify "$IMAGE"
}
