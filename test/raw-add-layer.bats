#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2018 SUSE LLC.
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

@test "umoci raw add-layer" {
	image-verify "${IMAGE}"

	LAYERS_DIR="$(setup_tmpdir)"

	# Create layer1.
	LAYER="$(setup_tmpdir)"
	echo "layer1" > "$LAYER/file"
	mkdir "$LAYER/dir1"
	echo "layer1" > "$LAYER/dir1/file"
	tar cvfC "$LAYERS_DIR/layer1.tar" "$LAYER" .

	# Create layer2.
	LAYER="$(setup_tmpdir)"
	echo "layer2" > "$LAYER/file"
	mkdir "$LAYER/dir2" "$LAYER/dir3"
	echo "layer2" > "$LAYER/dir2/file"
	echo "layer2" > "$LAYER/dir3/file"
	tar cvfC "$LAYERS_DIR/layer2.tar" "$LAYER" .

	# Create layer3.
	LAYER="$(setup_tmpdir)"
	echo "layer3" > "$LAYER/file"
	mkdir "$LAYER/dir2"
	echo "layer3" > "$LAYER/dir2/file"
	tar cvfC "$LAYERS_DIR/layer3.tar" "$LAYER" .

	# Add layers to the image.
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	#image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$LAYERS_DIR/layer1.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$LAYERS_DIR/layer2.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	umoci raw add-layer --image "${IMAGE}:${TAG}" "$LAYERS_DIR/layer3.tar"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the created image.
	BUNDLE="$(setup_tmpdir)" && ROOTFS="$BUNDLE/rootfs"
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

@test "umoci raw add-layer [missing args]" {
	DIR="$(setup_tmpdir)"

	# Missing layer.
	umoci raw add-layer --image="${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	# Missing image.
	touch "$DIR/file"
	umoci raw add-layer "$DIR/file"
	[ "$status" -ne 0 ]

	# Using a directory as an image file.
	umoci raw add-layer --image="${IMAGE}:${TAG}" "$DIR"
	[ "$status" -ne 0 ]
}

@test "umoci raw add-layer [too many args]" {
	DIR="$(setup_tmpdir)"
	touch "$DIR/file"{1..3}

	umoci raw add-layer --image "${IMAGE}:${TAG}" "$DIR/file"{1..3}
	[ "$status" -ne 0 ]
}
