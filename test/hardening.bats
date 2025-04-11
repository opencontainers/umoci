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

@test "umoci unpack [blob hash hardening]" {
	readarray -t allBlobs < <( cd "$IMAGE" && find "./blobs" -type f )
	for blob in "${allBlobs[@]}"
	do
		# Get a clean image.
		NEW_IMAGE="$(setup_tmpdir)"
		cp -rT "$IMAGE" "$NEW_IMAGE"

		blobHash="$(basename "$blob")" # sha256

		# Corrupt our blob such that the digest is changed within the expected
		# lengths (this takes priority over length issues) while also not
		# causing other verification to fail.
		case "$(file -bi "$NEW_IMAGE/$blob")" in
		*gzip*)
			# Re-compress it with a worse compression ratio. This, combined
			# with the fact that different gzip implementations produce
			# different output, means we will almost certainly get a new hash
			# *without* invalidating the DiffID.
			gzip -d <"$NEW_IMAGE/$blob" | gzip -3 | sponge "$NEW_IMAGE/$blob"

			# Make sure it's actually a different hash.
			[ "$(sha256sum "$NEW_IMAGE/$blob" | grep -o "$blobHash" | wc -l)" -eq 1 ]
			;;
		*)
			# Add a single whitespace byte at the *start* of the file which
			# will invalidate the prefix hash.
			( printf ' ' ; cat "$NEW_IMAGE/$blob" ) | sponge "$NEW_IMAGE/$blob"
			;;
		esac

		# Now let's try to extract it.
		new_bundle_rootfs
		umoci unpack --image "${NEW_IMAGE}:${TAG}" "$BUNDLE"
		[ "$status" -ne 0 ]
		echo "$output" | grep "verified reader digest mismatch"

		# TODO: When "umoci stat" grows recursive information output, use that.
		# TODO: Add more operations to check (repack might be complicated).
	done
}

@test "umoci unpack [descriptor size hardening]" {
	readarray -t allBlobs < <( cd "$IMAGE" && find "./blobs" -type f )
	for blob in "${allBlobs[@]}"
	do
		# Get a clean image.
		NEW_IMAGE="$(setup_tmpdir)"
		cp -rT "$IMAGE" "$NEW_IMAGE"

		blobHash="$(basename "$blob")" # sha256

		# Corrupt our blob such that the length is changed, and the length
		# issue will be triggered (this means the prefix-digest or whole-file
		# digest should be correct).
		case "$(file -bi "$NEW_IMAGE/$blob")" in
		*gzip*)
			# TODO: Figure out how to do this with gzip.
			continue
			;;
		*)
			# Add a single NUL byte at the *end* of the file which won't
			# invalidate the prefix hash.
			( cat "$NEW_IMAGE/$blob"; printf '\0x00' ) | sponge "$NEW_IMAGE/$blob"
			;;
		esac

		# TODO: Add more variants of this attack, such as modifying the length
		#       of descriptors. Though this might be quite hard to do in a
		#       comprehensive way with jq.

		# Now let's try to extract it.
		new_bundle_rootfs
		umoci unpack --image "${NEW_IMAGE}:${TAG}" "$BUNDLE"
		[ "$status" -ne 0 ]
		echo "$output" | grep "verified reader size mismatch"

		# TODO: When "umoci stat" grows recursive information output, use that.
		# TODO: Add more operations to check (repack might be complicated).
	done
}
