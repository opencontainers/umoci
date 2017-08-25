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

@test "umoci unpack --uid-map --gid-map" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	image-verify "${IMAGE}"

	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:1337:65535" --gid-map "0:8888:65535" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Check that all of the files have a UID owner >=1337 and a GID owner >=8888.
	find "$BUNDLE_A/rootfs" | xargs stat -c '%u:%g' | awk -F: '{
		uid = $1;
		if (uid < 1337 || uid >= 1337 + 65535)
			exit 1;
		gid = $2;
		if (gid < 8888 || gid >= 8888 + 65535)
			exit 1;
	}'

	# Unpack the image with a differen uid and gid mapping.
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:8080:65535" --gid-map "0:7777:65535" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_B/config.json" ]

	# Check that all of the files have a UID owner >=8080 and a GID owner >=7777.
	find "$BUNDLE_B/rootfs" | xargs stat -c '%u:%g' | awk -F: '{
		uid = $1;
		if (uid < 8080 || uid >= 8080 + 65535)
			exit 1;
		gid = $2;
		if (gid < 7777 || gid >= 7777 + 65535)
			exit 1;
	}'

	image-verify "${IMAGE}"
}

@test "umoci repack [--uid-map --gid-map]" {
	# We do a bunch of remapping tricks, which we can't really do if we're not root.
	requires root

	image-verify "${IMAGE}"

	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" --uid-map "0:1337:65535" --gid-map "0:7331:65535" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Create a new file with a remapped owner.
	echo "new file" > "$BUNDLE_A/rootfs/new test file "
	chown "2000:8000" "$BUNDLE_A/rootfs/new test file "

	# Repack the image using the same mapping.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack it again with a different mapping.
	umoci unpack --image "${IMAGE}:${TAG}-new" --uid-map "0:4000:65535" --gid-map "0:4000:65535" "$BUNDLE_B"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_B"

	# Make sure that the test file is different.
	sane_run stat -c '%u:%g' "$BUNDLE_B/rootfs/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337 + 4000)):$((8000 - 7331 + 4000))" ]]

	# Redo the unpacking with no mapping.
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE_C"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_C"

	# Make sure that the test file was unpacked properly.
	sane_run stat -c '%u:%g' "$BUNDLE_C/rootfs/new test file "
	[ "$status" -eq 0 ]
	[[ "$output" == "$((2000 - 1337)):$((8000 - 7331))" ]]

	image-verify "${IMAGE}"
}
