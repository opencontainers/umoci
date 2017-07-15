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

@test "umoci list" {
	image-verify "${IMAGE}"

	# Get list of tags.
	umoci ls --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	image-verify "${IMAGE}"

	# Get list of tags.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	nrefs="${#lines[@]}"
	image-verify "${IMAGE}"

	# Check how many refs there actually are.
	# Note that this is _very_ dodgy at the moment because of how complicated
	# the reference handling is now.
	sane_run jq -SMr '.manifests[] | .annotations["org.opencontainers.image.ref.name"] | strings' "$IMAGE/index.json"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$nrefs" ]

	image-verify "${IMAGE}"
}

@test "umoci list [missing args]" {
	umoci ls
	[ "$status" -ne 0 ]

	umoci list
	[ "$status" -ne 0 ]
}

@test "umoci tag" {
	# Get blob and mediatype that a tag references.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Add a new tag.
	umoci tag --image "${IMAGE}:${TAG}" "${TAG}-newtag"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that the new tag is the same.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Compare the stats.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	oldOutput="$output"
	umoci stat --image "${IMAGE}:${TAG}-newtag" --json
	[ "$status" -eq 0 ]
	newOutput="$output"

	[[ "$oldOutput" == "$newOutput" ]]

	image-verify "${IMAGE}"
}

@test "umoci tag [missing args]" {
	umoci tag --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	umoci tag new-tag
	[ "$status" -ne 0 ]
}

@test "umoci tag [clobber]" {
	# Get blob and mediatype that a tag references.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make a copy of the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${TAG}-newtag"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the configuration.
	umoci config --author="Someone" --image "${IMAGE}:${TAG}-newtag"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clobber the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${TAG}-newtag"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Compare the stats.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	oldOutput="$output"
	umoci stat --image "${IMAGE}:${TAG}-newtag" --json
	[ "$status" -eq 0 ]
	newOutput="$output"

	[[ "$oldOutput" == "$newOutput" ]]

	image-verify "${IMAGE}"
}

@test "umoci remove" {
	# How many tags?
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -gt 0 ]
	nrefs="${#lines[@]}"
	image-verify "${IMAGE}"

	# Remove the default tag.
	umoci rm --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure the tag is no longer there.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq "$(($nrefs - 1))" ]
	image-verify "${IMAGE}"

	# Check that the lines don't contain that tag.
	for tag in "${lines[@]}"; do
		[[ "$tag" != "${TAG}" ]]
	done

	# Make sure it's truly gone.
	umoci unpack --image "${IMAGE}:${TAG}" "$BATS_TMPDIR/notused"
	[ "$status" -ne 0 ]

	# ... like, really gone.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -ne 0 ]

	image-verify "${IMAGE}"
}

@test "umoci remove [missing args]" {
	umoci remove
	[ "$status" -ne 0 ]

	umoci rm
	[ "$status" -ne 0 ]
}
