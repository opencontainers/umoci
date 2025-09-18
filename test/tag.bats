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

@test "umoci list" {
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

@test "umoci list [invalid arguments]" {
	umoci list
	[ "$status" -ne 0 ]

	umoci ls
	[ "$status" -ne 0 ]

	# Missing --layout argument.
	umoci ls
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty layout path.
	umoci ls --layout ""
	[ "$status" -ne 0 ]

	# Layout path contains a ":".
	umoci ls --layout "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	# Unknown flag argument.
	umoci ls --this-is-an-invalid-argument --layout "${IMAGE}"
	[ "$status" -ne 0 ]

	# Too many positional arguments.
	umoci ls --layout "${IMAGE}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
}

@test "umoci tag" {
	NEW_TAG="${TAG}-newtag"

	# Get blob and mediatype that a tag references.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Add a new tag.
	umoci tag --image "${IMAGE}:${TAG}" "${NEW_TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that the new tag is the same.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Compare the stats -- aside from the refname annotation, they should be
	# identical.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	oldOutput="$(jq -rM 'del(.manifest.descriptor.annotations["org.opencontainers.image.ref.name"])' <<<"$output")"
	umoci stat --image "${IMAGE}:${NEW_TAG}" --json
	[ "$status" -eq 0 ]
	newOutput="$(jq -rM 'del(.manifest.descriptor.annotations["org.opencontainers.image.ref.name"])' <<<"$output")"

	[[ "$oldOutput" == "$newOutput" ]]

	image-verify "${IMAGE}"
}

@test "umoci tag [invalid arguments]" {
	NEW_TAG="${TAG}-newtag"

	# Missing --image and tag arguments.
	umoci tag
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	umoci tag "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing tag argument.
	umoci tag --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci tag --image ":${TAG}" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci tag --image "${IMAGE}-doesnotexist:${TAG}" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci tag --image "${IMAGE}:" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci tag --image "${IMAGE}:${TAG}-doesnotexist" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci tag --image "${IMAGE}:${INVALID_TAG}" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci tag --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$NEW_TAG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci tag --image "${IMAGE}:${TAG}" "$NEW_TAG" \
		this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci tag [clobber]" {
	NEW_TAG="${TAG}-newtag"

	# Get blob and mediatype that a tag references.
	umoci list --layout "${IMAGE}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make a copy of the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${NEW_TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the configuration.
	umoci config --author="Someone" --image "${IMAGE}:${NEW_TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clobber the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${NEW_TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Compare the stats -- aside from the refname annotation, they should be
	# identical.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	oldOutput="$(jq -rM 'del(.manifest.descriptor.annotations["org.opencontainers.image.ref.name"])' <<<"$output")"
	umoci stat --image "${IMAGE}:${NEW_TAG}" --json
	[ "$status" -eq 0 ]
	newOutput="$(jq -rM 'del(.manifest.descriptor.annotations["org.opencontainers.image.ref.name"])' <<<"$output")"

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
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]

	# ... like, really gone.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -ne 0 ]

	image-verify "${IMAGE}"
}

@test "umoci remove [invalid arguments]" {
	# Missing --image argument.
	umoci remove
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	umoci rm
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci rm --image ":${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci rm --image "${IMAGE}-doesnotexist:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image tag.
	umoci rm --image "${IMAGE}:"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image tag.
	umoci rm --image "${IMAGE}:${INVALID_TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci rm --this-is-an-invalid-argument --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci rm --image "${IMAGE}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}
