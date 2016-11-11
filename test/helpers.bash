#!/bin/bash
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

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")
UMOCI="${INTEGRATION_ROOT}/../umoci"
GOMTREE="/usr/bin/gomtree" # For some reason $(whence ...) and $(where ...) are broken.

# The source OCI image path, which we will make a copy of for each test.
SOURCE_IMAGE="${SOURCE_IMAGE:-/image}"
SOURCE_TAG="${SOURCE_TAG:-latest}"

# Where we're going to copy the images and bundle to.
IMAGE="${BATS_TMPDIR}/image"
TAG="${SOURCE_TAG}"

function umoci() {
	sane_run "$UMOCI" "$@"
}

function gomtree() {
	sane_run "$GOMTREE" "$@"
}

function sane_run() {
	local cmd="$1"
	shift

	run "$cmd" "$@"

	# Some debug information to make life easier.
	echo "$(basename "$cmd") $@ (status=$status)" >&2
	echo "$output" >&2
}

function setup_image() {
	cp -r "${SOURCE_IMAGE}" "${IMAGE}"
}

function teardown_image() {
	rm -rf "${IMAGE}"
}
