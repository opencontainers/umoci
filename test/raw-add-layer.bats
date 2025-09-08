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

@test "umoci raw add-layer" {
	# Create layer1.
	LAYER="$(setup_tmpdir)"
	echo "layer1" > "$LAYER/file"
	mkdir "$LAYER/dir1"
	echo "layer1" > "$LAYER/dir1/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer1.tar" "$LAYER" .
	[ "$status" -eq 0 ]

	# Create layer2.
	LAYER="$(setup_tmpdir)"
	echo "layer2" > "$LAYER/file"
	mkdir "$LAYER/dir2" "$LAYER/dir3"
	echo "layer2" > "$LAYER/dir2/file"
	echo "layer2" > "$LAYER/dir3/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer2.tar" "$LAYER" .
	[ "$status" -eq 0 ]

	# Create layer3.
	LAYER="$(setup_tmpdir)"
	echo "layer3" > "$LAYER/file"
	mkdir "$LAYER/dir2"
	echo "layer3" > "$LAYER/dir2/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer3.tar" "$LAYER" .
	[ "$status" -eq 0 ]

	# Add layers to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR/layer1.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR/layer2.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR/layer3.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the created image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure the layers were extracted in-order.
	sane_run cat "$ROOTFS/file"
	[ "$status" -eq 0 ]
	[[ "$output" == *"layer3"* ]]
	sane_run cat "$ROOTFS/dir1/file"
	[ "$status" -eq 0 ]
	[[ "$output" == *"layer1"* ]]
	sane_run cat "$ROOTFS/dir2/file"
	[ "$status" -eq 0 ]
	[[ "$output" == *"layer3"* ]]
	sane_run cat "$ROOTFS/dir3/file"
	[ "$status" -eq 0 ]
	[[ "$output" == *"layer2"* ]]

	image-verify "${IMAGE}"
}

OCI_MEDIATYPE_LAYER="application/vnd.oci.image.layer.v1.tar"

@test "umoci raw add-layer --compress=gzip" {
	LAYER="$(setup_tmpdir)"
	echo "layer" > "$LAYER/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer.tar" "$LAYER" .

	# Add layer to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=gzip "$UMOCI_TMPDIR/layer.tar"
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

@test "umoci raw add-layer --compress=zstd" {
	LAYER="$(setup_tmpdir)"
	echo "layer" > "$LAYER/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer.tar" "$LAYER" .

	# Add layer to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=zstd "$UMOCI_TMPDIR/layer.tar"
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

@test "umoci raw add-layer --compress=none" {
	LAYER="$(setup_tmpdir)"
	echo "layer" > "$LAYER/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer.tar" "$LAYER" .

	# Add layer to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=none "$UMOCI_TMPDIR/layer.tar"
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

@test "umoci raw add-layer --compress=auto" {
	LAYER="$(setup_tmpdir)"
	echo "layer" > "$LAYER/file"
	sane_run tar cvfC "$UMOCI_TMPDIR/layer.tar" "$LAYER" .

	# Add zstd layer to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=zstd "$UMOCI_TMPDIR/layer.tar"
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
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=auto "$UMOCI_TMPDIR/layer.tar"
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
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR/layer.tar"
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

@test "umoci raw add-layer [invalid arguments]" {
	LAYERFILE="$UMOCI_TMPDIR/file"
	touch "$LAYERFILE"{,-extra}

	# Missing --image and layer argument.
	umoci raw add-layer
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing layer argument.
	umoci raw add-layer --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	umoci raw add-layer "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci raw add-layer --image ":${TAG}" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci raw add-layer --image "${IMAGE}-doesnotexist:${TAG}" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci raw add-layer --image "${IMAGE}:" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci raw add-layer --image "${IMAGE}:${TAG}-doesnotexist" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci raw add-layer --image "${IMAGE}:${INVALID_TAG}" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image destination tag.
	umoci raw add-layer --image "${IMAGE}:${TAG}" --tag "" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image destination tag.
	umoci raw add-layer --image "${IMAGE}:${TAG}" --tag "${INVALID_TAG}" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci raw add-layer --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$LAYERFILE" "$LAYERFILE-extra"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent layer file.
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$LAYERFILE-doesnotexist"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Using a directory as a layer.
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$UMOCI_TMPDIR"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --compress=... has to be a valid value.
	umoci raw add-layer --image "${IMAGE}:${TAG}" --compress=invalid "$LAYERFILE"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}
