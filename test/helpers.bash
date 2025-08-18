#!/bin/bash
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

set -u
source "$(dirname "$BASH_SOURCE")/../hack/readlinkf.sh"

# Root directory of integration tests.
INTEGRATION_ROOT=$(dirname "$(readlinkf_posix "$BASH_SOURCE")")

# Binary paths.
UMOCI="${UMOCI:-${INTEGRATION_ROOT}/../umoci}"
# For some reason $(whence ...) and $(where ...) are broken.
RUNC="/usr/bin/runc"
GOMTREE="/usr/bin/gomtree"

# Used as a poor man's oci-image-tool validate.
DOCKER_METASCRIPT_DIR="${INTEGRATION_ROOT}/../hack/docker-meta-scripts"

# The source OCI image path, which we will make a copy of for each test.
SOURCE_IMAGE="${SOURCE_IMAGE:-/image}"
SOURCE_TAG="${SOURCE_TAG:-latest}"

# For "go build -cover" build umoci binaries.
GOCOVERDIR="${GOCOVERDIR:-}"

# Are we rootless?
IS_ROOTLESS="$(id -u)"

# Let's not store everything in /tmp -- that would just be messy.
TESTDIR_TMPDIR="$BATS_TMPDIR/umoci-integration"
mkdir -p "$TESTDIR_TMPDIR"

# Stores the set of tmpdirs that still have to be cleaned up. Calling
# teardown_tmpdirs will set this to an empty array (and all the tmpdirs
# contained within are removed).
export TESTDIR_LIST="$(mktemp "$TESTDIR_TMPDIR/umoci-integration-tmpdirs.XXXXXX")"

# INVALID_TAG is a sample invalid tag as per the OCI spec.
INVALID_TAG=".AZ94n18s"

# setup_tmpdir creates a new temporary directory and returns its name.  Note
# that if "$IS_ROOTLESS" is true, then removing this tmpdir might be harder
# than expected -- so tests should not really attempt to clean up tmpdirs.
function setup_tmpdir() {
	[[ -n "${UMOCI_TMPDIR:-}" ]] || UMOCI_TMPDIR="$TESTDIR_TMPDIR"
	mktemp -d "$UMOCI_TMPDIR/umoci-integration-tmpdir.XXXXXXXX" | tee -a "$TESTDIR_LIST"
}

# setup_tmpdirs just sets up the "built-in" tmpdirs.
function setup_tmpdirs() {
	declare -g UMOCI_TMPDIR="$(setup_tmpdir)"
}

# teardown_tmpdirs removes all tmpdirs created with setup_tmpdir.
function teardown_tmpdirs() {
	# Do nothing if TESTDIR_LIST doesn't exist.
	[ -e "$TESTDIR_LIST" ] || return

	# Remove all of the tmpdirs.
	while IFS= read -r tmpdir; do
		[ -e "$tmpdir" ] || continue
		chmod -R 0777 "$tmpdir"
		rm -rf "$tmpdir"
	done < "$TESTDIR_LIST"

	# Clear tmpdir list.
	rm -f "$TESTDIR_LIST"
}

# Where we're going to copy the images and bundle to.
IMAGE="$(setup_tmpdir)/image"
TAG="${SOURCE_TAG}"

function fail() {
	echo "FAILURE:" "$@" >&2
	false
}

# Allows a test to specify what things it requires. If the environment can't
# support it, the test is skipped with a message.
function requires() {
	for var in "$@"; do
		case $var in
			root)
				if [ "$IS_ROOTLESS" -ne 0 ]; then
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
	local ocidir="$1"

	# Smoke-test for our blobs.
	while IFS= read -r blob; do
		# Make sure tar layers are valid according to gnutar.
		mimetype="$(file --mime "$blob")"
		cat=
		case "$mimetype" in
			*application/x-tar*)
				cat="cat"
				;;
			*application/zstd*)
				cat="zstdcat"
				;;
			*application/gzip*)
				cat="zcat"
				;;
			*)
				continue
				;;
		esac
		"$cat" <"$blob" | file --mime - | grep "application/x-tar" || {
			fail "blob $blob is compressed but is not a tar archive?"
		}
		"$cat" <"$blob" | tar tvf - >/dev/null || {
			rc=$?
			file "$blob"
			echo "error untarring $blob: $rc"
			return "$rc"
		}
		echo "$blob: valid tar archive"
	done

	# Validate that all inodes are owned by the same uid:gid as the root
	# directory, which is the correct behaviour for us.
	owner="$(stat -c "%u:%g" "$ocidir")"
	allowners="$(find "$ocidir" -print0 | xargs -0 stat -c "%u:%g %n")"
	if grep -v "^$owner" <<<"$allowners" ; then
		echo "$allowners"
		fail "image $ocidir contains subpaths with incorrect owners"
	fi

	# Use the Docker meta-scripts (which use jq internally) to do some basic
	# validation of our image.
	BASHBREW_META_SCRIPTS="$DOCKER_METASCRIPT_DIR" \
		"$DOCKER_METASCRIPT_DIR/helpers/oci-validate.sh" "$ocidir"

	# oci-spec validation
	# FIXME: oci-image-tool broke due to image-spec repo changes, and so we
	#        have to skip this for now. The eventual plan is to move the
	#        validation to umoci itself.
	#oci-image-tool validate --type "imageLayout" "$ocidir"
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
	if [[ "$1" == "raw" ]]; then
		args+=("$1")
		shift 1
	fi

	# Set the first argument (the subcommand).
	# TODO: This doesn't correctly handle any global arguments which go before
	#       the subcommand. We should probably switch to getopt here.
	args+=("$1")

	# Set --rootless for commands that require it when we're rootless.
	if [[ "$IS_ROOTLESS" != 0 && "$1" =~ unpack|insert ]]; then
		args+=("--rootless")
	fi

	shift
	args+=("$@")
	sane_run "$UMOCI" "${args[@]}"
}

function gomtree() {
	local args=("$@")

	# We're rootless. Note that the "-rootless" flag is actually an out-of-tree
	# patch applied by openSUSE here:
	#   <https://build.opensuse.org/package/show/Virtualization:containers/go-mtree>.
	if [[ "$IS_ROOTLESS" != 0 ]]; then
		args+=("--rootless")
	fi

	sane_run "$GOMTREE" -K sha256digest "${args[@]}"
}

function runc() {
	sane_run "$RUNC" --root "$RUNC_ROOT" "$@"
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
	image-verify "${IMAGE}"

	# These are just used for diagnostics, so we ignore the status.
	sane_run df
	sane_run du -h -d 2 "$UMOCI_TMPDIR"
}

function teardown_image() {
	rm_rf "${IMAGE}"
}

function setup_runc() {
	declare -g RUNC_ROOT="$(setup_tmpdir)"
}

function retry() {
	local attempts="$1" delay="$2"
	shift 2

	local i
	for ((i = 0; i < attempts; i++)); do
		sane_run "$@"
		if [[ "$status" -eq 0 ]]; then
			return 0
		fi
		sleep "$delay"
	done

	echo "command \"$*\" failed $attempts times" >&2
	false
}

function is_container_dead() {
	runc state "$1"
	[ "$status" -ne 0 ] || [[ "$output" = *stopped* ]]
}

function teardown_runc() {
	for ctr in $(runc list -q)
	do
		runc kill "$ctr" KILL
		retry 10 1 eval "is_container_dead '$ctr'"
		runc delete -f "$ctr"
	done
}

# Generate a new $BUNDLE and $ROOTFS combination.
function new_bundle_rootfs() {
	declare -g BUNDLE="$(setup_tmpdir)"
	declare -g ROOTFS="$BUNDLE/rootfs"
}

# _getfattr is a sane wrapper around getfattr(1) which only extracts the value
# of the requested xattr (and removes any of the other crap that it spits out).
# The usage is "sane_run _getfattr <xattr name> <path>" and outputs the hex
# representation in a single line. Exit status is non-zero if the xattr isn't
# set.
function _getfattr() {
	# We only support single-file inputs.
	[ "$#" -eq 2 ] || return 1

	local xattr="$1"
	local path="$2"

	# Run getfattr.
	(
		set -o pipefail
		getfattr -e hex -n "$xattr" "$path" 2>/dev/null \
			| grep "^$xattr=" | sed "s|^$xattr=||g"
	)
	return $?
}

# A wrapper around "rm -rf" which works for unprivileged users (namely cases
# where a directory has mode 000 which causes a DAC refusal even though the
# user is the owner of the directory).
function rm_rf() {
	for path in "$@"
	do
		[ -e "$path" ] || continue
		dir="$(dirname "$path")"

		# We need to make sure the parent directory is searchable and writable
		# but we want to recover the original mode after we do the delete.
		old_mode="$(stat -c '%a' "$dir")"

		sane_run chmod 0777 "$dir"
		[ "$status" -eq 0 ]

		[ -d "$path" ] && find "$path" -type d -exec chmod +rwx {} \;
		rm -rf "$path"

		sane_run chmod "$old_mode" "$dir"
		[ "$status" -eq 0 ]
	done
}
