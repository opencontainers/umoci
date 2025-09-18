#!/usr/bin/env bats -t
# SPDX-License-Identifier: Apache-2.0
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2025 SUSE LLC
# Copyright (C) 2018 Cisco Systems
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

@test "umoci insert" {
	# fail with too few arguments
	umoci insert --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# ...and too many
	umoci insert --image "${IMAGE}:${TAG}" asdf 123 456
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/test"
	touch "${INSERTDIR}/test/a"
	touch "${INSERTDIR}/test/b"
	chmod +x "${INSERTDIR}/test/b"
	echo "foo" > "${INSERTDIR}/test/smallfile"

	# Make sure rootless mode works.
	mkdir -p "${INSERTDIR}/some/path"
	touch "${INSERTDIR}/some/path/hidden"
	chmod 000 "${INSERTDIR}/some/path"

	# Do a few inserts.
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/a" /tester/a
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/b" /tester/b
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test" /recursive
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci insert --image "${IMAGE}:${TAG}" --tag "${TAG}-new" "${INSERTDIR}/some" /rootless
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/smallfile" /tester/smallfile
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack after the inserts.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# ... and check to make sure it worked.
	[ -f "$ROOTFS/tester/a" ]
	[[ "$(stat -c '%f' "${INSERTDIR}/test/b")" == "$(stat -c '%f' "$ROOTFS/tester/b")" ]]
	[ -f "$ROOTFS/recursive/a" ]
	[ -f "$ROOTFS/recursive/b" ]

	# ... as well as the rootless portion.
	[ -d "$ROOTFS/rootless/path" ]
	[[ "$(stat -c '%f' "${INSERTDIR}/some/path")" == "$(stat -c '%f' "$ROOTFS/rootless/path")" ]]
	chmod a+rwx "$ROOTFS/rootless/path"
	[ -f "$ROOTFS/rootless/path/hidden" ]

	image-verify "${IMAGE}"
}

@test "umoci insert [invalid arguments]" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	touch "$INSERTDIR/foobar"

	# Missing --image, source, and target argument.
	umoci insert
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image and target argument.
	umoci insert "$INSERTDIR"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing source and target argument.
	umoci insert --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing target argument.
	umoci insert --image "${IMAGE}:${TAG}" "$INSERTDIR"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci insert --this-is-an-invalid-argument \
		--image "${IMAGE}:${TAG}" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty positional arguments.
	umoci insert --image "${IMAGE}:${TAG}" "$INSERTDIR" ""
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	umoci insert --image "${IMAGE}:${TAG}" "" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty positional arguments (--whiteout).
	umoci insert --image "${IMAGE}:${TAG}" --whiteout ""
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci insert --image "${IMAGE}:${TAG}" "$INSERTDIR" /foo/bar this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments (--whiteout).
	umoci insert --image "${IMAGE}:${TAG}" --whiteout /foo/bar this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci insert --image ":${TAG}" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci insert --image "${IMAGE}-doesnotexist:${TAG}" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci insert --image "${IMAGE}:" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci insert --image "${IMAGE}:${TAG}-doesnotexist" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci insert --image "${IMAGE}:${INVALID_TAG}" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image destination tag.
	umoci insert --image "${IMAGE}:${TAG}" --tag "" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image destination tag.
	umoci insert --image "${IMAGE}:${TAG}" --tag "${INVALID_TAG}" "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Insertion of a file with a .wh. prefix must fail.
	umoci insert --image "${IMAGE}:${TAG}" "$INSERTDIR" /foo/bar/.wh.invalid-path
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Whiteout of a file with a .wh. prefix must fail.
	umoci insert --image "${IMAGE}:${TAG}" --whiteout /foo/bar/.wh.invalid-whiteout
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --compress=... has to be a valid value.
	umoci insert --image "${IMAGE}:${TAG}" --compress=invalid "$INSERTDIR" /foo/bar
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci insert --opaque" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# Insert our /etc.
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that the /etc/foo is there.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that it's merged!
	[ -f "$ROOTFS/etc/shadow" ]
	[ -f "$ROOTFS/etc/foo" ]

	# Now make it opaque to make sure it isn't included.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/bar"
	touch "${INSERTDIR}/should_be_fine"

	# Insert our /etc.
	umoci insert --image "${IMAGE}:${TAG}" --opaque "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	# And try to make a file opaque just to see what happens (should be nothing).
	umoci insert --image "${IMAGE}:${TAG}" --opaque "${INSERTDIR}/should_be_fine" /should_be_fine
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that now only /etc/bar is around.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that it's _not_ merged!
	! [ -f "$ROOTFS/etc/shadow" ]
	! [ -f "$ROOTFS/etc/foo" ]
	# And that bar is there.
	[ -f "$ROOTFS/etc/bar" ]
	# And that should_be_fine is around.
	[ -f "$ROOTFS/should_be_fine" ]

	image-verify "${IMAGE}"
}

@test "umoci insert --whiteout" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	touch "${INSERTDIR}/rm_file"
	mkdir "${INSERTDIR}/rm_dir"

	# Add our things.
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/rm_file" /rm_file
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/rm_dir" /rm_dir
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack after the inserts.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	[ -d "$ROOTFS/etc" ]
	[ -d "$ROOTFS/rm_dir" ]
	[ -f "$ROOTFS/rm_file" ]

	# Directory whiteout.
	umoci insert --image "${IMAGE}:${TAG}" --whiteout /rm_dir
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# (Another) directory whiteout.
	umoci insert --image "${IMAGE}:${TAG}" --whiteout /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# File whiteout.
	umoci insert --image "${IMAGE}:${TAG}" --whiteout /rm_file
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack after the inserts.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	! [ -d "$ROOTFS/etc" ]
	! [ -d "$ROOTFS/rm_dir" ]
	! [ -f "$ROOTFS/rm_file" ]

	image-verify "${IMAGE}"
}

@test "umoci insert --history.*" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# umoci-insert will overwrite the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${TAG}-new"
	[ "$status" -eq 0 ]

	# Insert something into the image, setting history values.
	now="$(date --iso-8601=seconds --utc)"
	umoci insert --image "${IMAGE}:${TAG}-new" \
		--history.author="Some Author <jane@blogg.com>" \
		--history.comment="Made a_small_change." \
		--history.created_by="insert ${INSERTDIR}" \
		--history.created="$now" "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure that the history was modified.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SMr '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SMr '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# The final layer should not be an empty_layer now.
	[[ "$(echo "$output" | jq -SMr '.history[-1].empty_layer')" == "null" ]]
	# The author should've changed to --history.author.
	[[ "$(echo "$output" | jq -SMr '.history[-1].author')" == "Some Author <jane@blogg.com>" ]]
	# The comment should be added.
	[[ "$(echo "$output" | jq -SMr '.history[-1].comment')" == "Made a_small_change." ]]
	# The created_by should be set.
	[[ "$(echo "$output" | jq -SMr '.history[-1].created_by')" == "insert ${INSERTDIR}" ]]
	# The created should be set.
	[[ "$(date --iso-8601=seconds --utc --date="$(echo "$output" | jq -SMr '.history[-1].created')")" == "$now" ]]

	image-verify "${IMAGE}"
}

@test "umoci insert --no-history" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# umoci-insert will overwrite the tag.
	umoci tag --image "${IMAGE}:${TAG}" "${TAG}-new"
	[ "$status" -eq 0 ]

	# Insert something into the image, but with no history change.
	umoci insert --no-history --image "${IMAGE}:${TAG}-new" "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Make sure we *did not* add a new history entry.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	hashA="$(jq '.history' <<<"$output" | sha256sum)"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	hashB="$(jq '.history' <<<"$output" | sha256sum)"

	# umoci-stat history output should be identical.
	[[ "$hashA" == "$hashB" ]]

	image-verify "${IMAGE}"
}

OCI_MEDIATYPE_LAYER="application/vnd.oci.image.layer.v1.tar"

@test "umoci insert --compress=gzip" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# Add layer to the image.
	umoci insert --image "${IMAGE}:${TAG}" --compress=gzip "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+gzip"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/gzip"* ]]
}

@test "umoci insert --compress=zstd" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# Add layer to the image.
	umoci insert --image "${IMAGE}:${TAG}" --compress=zstd "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]
}

@test "umoci insert --compress=none" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# Add layer to the image.
	umoci insert --image "${IMAGE}:${TAG}" --compress=none "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/x-tar"* ]] # x-tar means no compression
}

@test "umoci insert --compress=auto" {
	# Some things to insert.
	INSERTDIR="$(setup_tmpdir)"
	mkdir -p "${INSERTDIR}/etc"
	touch "${INSERTDIR}/etc/foo"

	# Add zstd layer to the image.
	umoci insert --image "${IMAGE}:${TAG}" --compress=zstd "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]

	# Add another zstd layer to the image, by making use of the auto selection.
	umoci insert --image "${IMAGE}:${TAG}" --compress=auto "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]

	# Add yet another zstd layer to the image, to show that --compress=auto is
	# the default.
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/etc" /etc
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	stat_json="$output"

	# Make sure that the last layer had the expected compression based on the
	# mediatype.
	expected_mediatype="${OCI_MEDIATYPE_LAYER}+zstd"
	layer_mediatype="$(jq -SMr '.history[-1].layer.mediaType' <<<"$stat_json")"
	[[ "$layer_mediatype" == "$expected_mediatype" ]]

	# Make sure that the actual blob seems to be a gzip
	layer_hash="$(jq -SMr '.history[-1].layer.digest' <<<"$stat_json" | tr : /)"
	sane_run file -i "$IMAGE/blobs/$layer_hash"
	[ "$status" -eq 0 ]
	[[ "$output" == *"application/zstd"* ]]
}
