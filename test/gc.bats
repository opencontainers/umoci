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

@test "umoci gc [missing args]" {
	umoci gc
	[ "$status" -ne 0 ]
}

@test "umoci gc [consistent]" {
	image-verify "${IMAGE}"

	# Initial gc.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	nblobs="${#lines[@]}"

	# Redo the gc.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nblobs" ]

	image-verify "${IMAGE}"
}

@test "umoci gc" {
	BUNDLE="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Initial gc.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	nblobs="${#lines[@]}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Change the rootfs. We need to chmod because of fedora.
	chmod +w "$BUNDLE/rootfs/usr/bin/." && rm -rf "$BUNDLE/rootfs/usr/bin"
	chmod +w "$BUNDLE/rootfs/etc/." && rm -rf "$BUNDLE/rootfs/etc"

	# Repack the image under a new tag.
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure the number of blobs has changed.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "$nblobs" -ne "${#lines[@]}" ]
	nblobs="${#lines[@]}"

	# Make sure it is the same after doing a gc, because we used a new tag.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nblobs" ]

	# Delete the old reference.
	umoci rm --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Now do a gc which should delete some blobs.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that another gc run does nothing.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -lt "$nblobs" ]

	image-verify "${IMAGE}"
}

@test "umoci gc [empty]" {
	image-verify "${IMAGE}"

	# Initial gc.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -ne 0 ]

	# Remove refs.
	umoci ls --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	image-verify "${IMAGE}"

	for line in "${lines[*]}"; do
		umoci rm --image "${IMAGE}:${line}"
		[ "$status" -eq 0 ]
		image-verify "${IMAGE}"
	done

	# Do a gc, which should remove all blobs.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Check how many blobs there were.
	sane_run find "$IMAGE/blobs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 0 ]

	image-verify "${IMAGE}"
}

@test "umoci gc [internal]" {
	image-verify "${IMAGE}"

	# Initial gc.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Create unused directories.
	touch "${IMAGE}/.internal"
	touch "${IMAGE}/  magical file   "
	mkdir "${IMAGE}/  __ internal __ directory"
	touch "${IMAGE}/  __ internal __ directory/.abc"

	# Do a gc, which should remove the temporary files/directories.
	umoci gc --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure it's gone.
	! [ -e "${IMAGE}/.internal" ]
	! [ -e "${IMAGE}/  magical file   " ]
	! [ -e "${IMAGE}/  __ internal __ directory" ]
	! [ -e "${IMAGE}/  __ internal __ directory/.abc" ]

	image-verify "${IMAGE}"
}
