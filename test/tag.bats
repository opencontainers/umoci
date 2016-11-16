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

@test "umoci tag list" {
	# Get list of tags.
	umoci tag --image "$IMAGE" ls
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]

	# Get list of tags.
	umoci tag --image "$IMAGE" list
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	nrefs="${#lines[@]}"

	# Check how many refs there actually are.
	sane_run find "$IMAGE/refs" -type f
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nrefs" ]
}

@test "umoci tag add" {
	# Get digest and mediatype that a tag references.
	umoci tag --image "$IMAGE" list
	[ "$status" -eq 0 ]

	# FIXME: This should really be "umoci tag stat" or something.
	for line in "${lines[@]}"; do
		tag="$(echo $line | awk '{ print $1 }')"
		mediatype="$(echo $line | awk '{ print $2 }')"
		digest="$(echo $line | awk '{ print $3 }')"
		[[ "$tag" != "$TAG" ]] || break
	done
	[[ "$tag" == "$TAG" ]]

	# Add a new tag.
	umoci tag --image "$IMAGE" add --tag "${TAG}-newtag" --blob "$digest" --media-type "$mediatype"
	[ "$status" -eq 0 ]

	# Make sure that the new tag is the same.
	umoci tag --image "$IMAGE" list
	[ "$status" -eq 0 ]

	# FIXME: As above this should be "umoci tag stat".
	for line in "${lines[@]}"; do
		_tag="$(echo $line | awk '{ print $1 }')"
		_mediatype="$(echo $line | awk '{ print $2 }')"
		_digest="$(echo $line | awk '{ print $3 }')"
		[[ "$_tag" != "${TAG}-newtag" ]] || break
	done
	[[ "$_tag" == "${TAG}-newtag" ]]
	[[ "$_mediatype" == "$mediatype" ]]
	[[ "$_digest" == "$digest" ]]
}

@test "umoci tag rm" {
	# How many tags?
	umoci tag --image "$IMAGE" list
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	nrefs="${#lines[@]}"

	# Remove the default tag.
	umoci tag --image "$IMAGE" rm --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Make sure the tag is no longer there.
	umoci tag --image "$IMAGE" list
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$(($nrefs - 1))" ]

	# FIXME: This should really be "umoci tag stat" or something.
	# Check that the lines don't contain that tag.
	for line in "${lines[@]}"; do
		tag="$(echo $line | awk '{ print $1 }')"
		[[ "$tag" != "$TAG" ]]
	done

	# Make sure it's truly gone.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BATS_TMPDIR/notused"
	[ "$status" -ne 0 ]
}
