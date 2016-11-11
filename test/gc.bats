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

BUNDLE="$BATS_TMPDIR/bundle"

function setup() {
	setup_image
}

function teardown() {
	teardown_image
	rm -rf "$BUNDLE"
}

@test "umoci gc [consistent]" {
	# Initial gc.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	nblobs="${#lines[@]}"

	# Redo the gc.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nblobs" ]
}

@test "umoci gc" {
	# Initial gc.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	nblobs="${#lines[@]}"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE"
	[ "$status" -eq 0 ]

	# Change the rootfs.
	rm -r "$BUNDLE/rootfs/usr/bin"
	rm -r "$BUNDLE/rootfs/etc"

	# Repack the image under a new tag.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE" --tag "${TAG}-new"
	[ "$status" -eq 0 ]

	# Make sure the number of blobs has changed.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "$nblobs" -ne "${#lines[@]}" ]
	nblobs="${#lines[@]}"

	# Make sure it is the same after doing a gc, because we used a new tag.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nblobs" ]

	# Delete the old reference.
	rm "$IMAGE/refs/$TAG"

	# Now do a gc which should delete some blobs.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -lt "$nblobs" ]
}

@test "umoci gc [empty]" {
	# Initial gc.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -ne 0 ]

	# Remove refs.
	rm "$IMAGE"/refs/*

	# Do a gc, which should remove all blobs.
	umoci gc --image "$IMAGE"
	[ "$status" -eq 0 ]

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 0 ]
}
