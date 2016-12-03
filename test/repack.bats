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

@test "umoci repack" {
	BUNDLE_A="$(setup_bundle)"
	BUNDLE_B="$(setup_bundle)"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure the files we're creating don't exist.
	! [ -e "$BUNDLE_A/rootfs/newfile" ]
	! [ -e "$BUNDLE_A/rootfs/newdir" ]
	! [ -e "$BUNDLE_A/rootfs/newdir/anotherfile" ]
	! [ -e "$BUNDLE_A/rootfs/newdir/link" ]

	# Create them.
	echo "first file" > "$BUNDLE_A/rootfs/newfile"
	mkdir "$BUNDLE_A/rootfs/newdir"
	echo "subfile" > "$BUNDLE_A/rootfs/newdir/anotherfile"
	# this currently breaks go-mtree but I've backported a patch to fix it in openSUSE
	ln -s "this is a dummy symlink" "$BUNDLE_A/rootfs/newdir/link"

	# Repack the image under a new tag.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}-new"
	[ "$status" -eq 0 ]

	# Unpack it again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Ensure that gomtree suceeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$BUNDLE_B/rootfs/newfile" ]
	[ -d "$BUNDLE_B/rootfs/newdir" ]
	[ -f "$BUNDLE_B/rootfs/newdir/anotherfile" ]
	[ -L "$BUNDLE_B/rootfs/newdir/link" ]
}

@test "umoci repack [whiteout]" {
	BUNDLE_A="$(setup_bundle)"
	BUNDLE_B="$(setup_bundle)"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure the files we're deleting exist.
	[ -d "$BUNDLE_A/rootfs/etc" ]
	[ -L "$BUNDLE_A/rootfs/bin/sh" ]
	[ -e "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Remove them.
	chmod +w "$BUNDLE_A/rootfs/etc/." && rm -rf "$BUNDLE_A/rootfs/etc"
	chmod +w "$BUNDLE_A/rootfs/bin/." && rm "$BUNDLE_A/rootfs/bin/sh"
	chmod +w "$BUNDLE_A/rootfs/usr/bin/." && rm "$BUNDLE_A/rootfs/usr/bin/env"

	# Repack the image under a new tag.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}-new"
	[ "$status" -eq 0 ]

	# Unpack it again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Ensure that gomtree suceeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	! [ -e "$BUNDLE_A/rootfs/etc" ]
	! [ -e "$BUNDLE_A/rootfs/bin/sh" ]
	! [ -e "$BUNDLE_A/rootfs/usr/bin/env" ]
}

@test "umoci repack [replace]" {
	BUNDLE_A="$(setup_bundle)"
	BUNDLE_B="$(setup_bundle)"

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure the files we're replacing exist.
	[ -d "$BUNDLE_A/rootfs/etc" ]
	[ -L "$BUNDLE_A/rootfs/bin/sh" ]
	[ -e "$BUNDLE_A/rootfs/usr/bin/env" ]

	# Replace them.
	chmod +w "$BUNDLE_A/rootfs/etc/." && rm -rf "$BUNDLE_A/rootfs/etc"
	echo "different" > "$BUNDLE_A/rootfs/etc"
	chmod +w "$BUNDLE_A/rootfs/bin/." && rm "$BUNDLE_A/rootfs/bin/sh"
	mkdir "$BUNDLE_A/rootfs/bin/sh"
	chmod +w "$BUNDLE_A/rootfs/usr/bin/." && rm "$BUNDLE_A/rootfs/usr/bin/env"
	# this currently breaks go-mtree but I've backported a patch to fix it in openSUSE
	ln -s "a \\really //weird _00..:=path " "$BUNDLE_A/rootfs/usr/bin/env"

	# Repack the image under the same tag.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Unpack it again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Ensure that gomtree suceeds on the old bundle, which is what this was
	# generated from.
	gomtree -p "$BUNDLE_A/rootfs" -f "$BUNDLE_B"/sha256_*.mtree
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Just for sanity, check that everything looks okay.
	[ -f "$BUNDLE_A/rootfs/etc" ]
	[ -d "$BUNDLE_A/rootfs/bin/sh" ]
	[ -L "$BUNDLE_A/rootfs/usr/bin/env" ]
}

# TODO: Test hardlinks once we fix the hardlink issue. https://github.com/cyphar/umoci/issues/29
