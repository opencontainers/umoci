#!/bin/bash
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

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlink -f "$BASH_SOURCE")")
UMOCI="${UMOCI:-${INTEGRATION_ROOT}/../umoci}"
GOMTREE="/usr/bin/gomtree" # For some reason $(whence ...) and $(where ...) are broken.

# The source OCI image path, which we will make a copy of for each test.
SOURCE_IMAGE="${SOURCE_IMAGE:-/image}"
SOURCE_TAG="${SOURCE_TAG:-latest}"

# Where we're going to copy the images and bundle to.
IMAGE="${BATS_TMPDIR}/image"
TAG="${SOURCE_TAG}"

# We need to store the coverage outputs somewhere.
COVERAGE_DIR="${COVERAGE_DIR:-}"

# Are we rootless?
ROOTLESS="$(id -u)"

# Allows a test to specify what things it requires. If the environment can't
# support it, the test is skipped with a message.
function requires() {
	for var in "$@"; do
		case $var in
			root)
				if [ "$ROOTLESS" -ne 0 ]; then
					skip "test requires ${var}"
				fi
				;;
			*)
				fail "BUG: Invalid requires ${var}."
				;;
		esac
	done
}

function image-verify() {
	oci-image-validate --type "imageLayout" "$@"
	return $?
}

function bundle-verify() {
	args=()

	for arg in "$@"; do
		args+=( --path="$arg" )
	done

	oci-runtime-tool validate "${args[@]}"
	return $?
}

function umoci() {
	local args=()
	if [ "$COVER" -eq 1 ]; then
		if [ "$COVERAGE_DIR" ]; then
			args+=("-test.coverprofile=$(mktemp -p "$COVERAGE_DIR" umoci.cov.XXXXXX)")
		fi
		args+=("__DEVEL--i-heard-you-like-tests")
	fi

	# Set the first argument (the subcommand).
	args+=("$1")

	# We're rootless if we're asked to unpack something.
	if [[ "$ROOTLESS" != 0 && "$1" == "unpack" ]]; then
		args+=("--rootless")
	fi

	shift
	args+=("$@")
	sane_run "$UMOCI" "${args[@]}"

	if [ "$COVER" -eq 1 ]; then
		# Because this is running as a -test.cover test, we need to remove the last
		# two lines.
		if [ "$status" -eq 0 ]; then
			export output="$(echo "$output" | head -n-2)"
			unset 'lines[${#lines[@]}-1]'
			unset 'lines[${#lines[@]}-1]'
		fi
	fi
}

function gomtree() {
	local args=("$@")

	# We're rootless. Note that this is _not_ available from the upstream
	# version of go-mtree. It's a feature I implemented in the library for
	# umoci's support, and is currently being proposed in
	# https://github.com/vbatts/go-mtree/pull/96. I would not hold my breath
	# that it'll be merged any time soon.
	if [[ "$ROOTLESS" != 0 ]]; then
		args+=("-rootless")
	fi

	sane_run "$GOMTREE" -K sha256digest "${args[@]}"
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

# setup_bundle creates a new temporary bundle directory and returns its name.
# Note that if "$ROOTLESS" is true, then removing this bundle might be harder
# than expected -- so tests should not really attempt to clean up bundles.
function setup_bundle() {
	echo "$(mktemp -d --tmpdir="$BATS_TMPDIR" umoci-integration-bundle.XXXXXXXX)"
}
