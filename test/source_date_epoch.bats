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

@test "umoci new [w/SOURCE_DATE_EPOCH]" {
	IMAGE="$(setup_tmpdir)/image" TAG="test-new"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	target_epoch="$(date -ud 'now - 6 months' +%s)"
	target_date="$(date -ud "@$target_epoch" +%FT%TZ)" # --iso-8601 doesn't use "Z"

	# Create blank image.
	export SOURCE_DATE_EPOCH="$target_epoch"
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	unset SOURCE_DATE_EPOCH

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	# Make sure the default config created time uses SOURCE_DATE_EPOCH too.
	config_created_time="$(jq -SMr '.config.blob.created' <<<"$output")"
	[[ "$config_created_time" == "$target_date" ]]
}

@test "umoci config [SOURCE_DATE_EPOCH]" {
	IMAGE="$(setup_tmpdir)/image" TAG="test-new"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	target_epoch="$(date -ud 'now - 6 months' +%s)"
	target_date="$(date -ud "@$target_epoch" +%FT%TZ)" # --iso-8601 doesn't use "Z"

	export SOURCE_DATE_EPOCH="$target_epoch"
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci config --config.user="foo:bar" --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	unset SOURCE_DATE_EPOCH

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	# Make sure the default config created time uses SOURCE_DATE_EPOCH too.
	config_created_time="$(jq -SMr '.config.blob.created' <<<"$output")"
	[[ "$config_created_time" == "$target_date" ]]
	# Make sure that the history entry uses SOURCE_DATE_EPOCH too.
	history_created_time="$(jq -SMr '.history[-1].created' <<<"$output")"
	[[ "$history_created_time" == "$target_date" ]]
}

@test "umoci config --created --history.created [SOURCE_DATE_EPOCH]" {
	IMAGE="$(setup_tmpdir)/image" TAG="test-new"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	dummy_epoch="$(date -ud 'now - 6 months' +%s)"
	dummy_date="$(date -ud "@$dummy_epoch" +%FT%TZ)" # --iso-8601 doesn't use "Z"

	target_config_date="1997-03-25T13:40:00+01:00"
	target_history_date="1997-03-25T13:40:00+01:00"

	export SOURCE_DATE_EPOCH="$dummy_epoch"
	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci config --image "${IMAGE}:${TAG}" \
		--config.user="foo:bar" \
		--created="$target_config_date" \
		--history.created="$target_history_date"
	[ "$status" -eq 0 ]
	unset SOURCE_DATE_EPOCH

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	# If --created or --history.created are explicitly set, SOURCE_DATE_EPOCH
	# is ignored.
	config_created_time="$(jq -SMr '.config.blob.created' <<<"$output")"
	[[ "$config_created_time" == "$target_config_date" ]]
	history_created_time="$(jq -SMr '.history[-1].created' <<<"$output")"
	[[ "$history_created_time" == "$target_history_date" ]]
}

@test "umoci new [w/o SOURCE_DATE_EPOCH]" {
	IMAGE="$(setup_tmpdir)/image" TAG="test-new"

	unset SOURCE_DATE_EPOCH

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	before_epoch="$(date -u +%s)"

	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	after_epoch="$(date -u +%s)"

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	# Make sure that in the absense of SOURCE_DATE_EPOCH you get a reasonable
	# time value (i.e., the current time).
	config_created_time="$(jq -SMr '.config.blob.created' <<<"$output")"
	config_created_epoch="$(date -u -d "$config_created_time" +%s)"
	[ "$config_created_epoch" -ge "$before_epoch" ]
	[ "$config_created_epoch" -le "$after_epoch" ]
}

@test "umoci repack [SOURCE_DATE_EPOCH]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	target_epoch="$(date -ud 'now - 20 years' +%s)"
	target_date="$(date -ud "@$target_epoch" +%FT%TZ)" # --iso-8601 doesn't use "Z"

	untouched_file_epoch="$(stat -c '%Y' "$ROOTFS/bin/sh")"

	touch "$ROOTFS/etc/passwd"
	touched_file_epoch="$(stat -c '%Y' "$ROOTFS/etc/passwd")"
	echo "newfile" >"$ROOTFS/newfile"

	echo "oldfile" >"$ROOTFS/oldfile"
	touch -d "1997-03-25T13:40:00" "$ROOTFS/oldfile"
	old_file_epoch="$(stat -c '%Y' "$ROOTFS/oldfile")"

	# Make sure that the untouched file from the lower layer is newer than
	# SOURCE_DATE_EPOCH (to make sure we aren't modifying the mtimes of
	# untouched files).
	[ "$untouched_file_epoch" -gt "$target_epoch" ]
	[ "$touched_file_epoch" -gt "$target_epoch" ]
	[ "$old_file_epoch" -lt "$target_epoch" ]

	export SOURCE_DATE_EPOCH="$target_epoch"
	umoci repack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	unset SOURCE_DATE_EPOCH

	# Unpack it again (without SOURCE_DATE_EPOCH).
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Files newer than SOURCE_DATE_EPOCH should be set to SOURCE_DATE_EPOCH.
	[ "$(stat -c '%Y' "$ROOTFS/etc/passwd")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/newfile")" -eq "$target_epoch" ]

	# Old files should remain the same.
	[ "$(stat -c '%Y' "$ROOTFS/oldfile")" -eq "$old_file_epoch" ]

	# Files from lower layer should also remain the same.
	[ "$(stat -c '%Y' "$ROOTFS/bin/sh")" -eq "$untouched_file_epoch" ]

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	# Make sure that the history entry uses SOURCE_DATE_EPOCH too.
	history_created_time="$(jq -SMr '.history[-1].created' <<<"$output")"
	[[ "$history_created_time" == "$target_date" ]]
}

@test "umoci insert [SOURCE_DATE_EPOCH]" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	target_epoch="$(date -ud 'now - 4 weeks' +%s)"
	target_date="$(date -ud "@$target_epoch" +%FT%TZ)" # --iso-8601 doesn't use "Z"

	insert_dir="$(setup_tmpdir)"
	mkdir -p "$insert_dir/foo/bar/baz"
	ln -s foo "$insert_dir/link"

	new_file_date="$(date -ud 'now + 3 years' +%Y%m%dT%H:%M:%S)"
	echo "newerfile" >"$insert_dir/newer"
	touch -d "$new_file_date" "$insert_dir/newer"

	old_file_date="$(date -ud 'now - 2 years' +%Y%m%dT%H:%M:%S)"
	old_file_epoch="$(date -ud "$old_file_date" +%s)"
	echo "olderfile" >"$insert_dir/older"
	touch -d "$old_file_date" "$insert_dir/older"

	export SOURCE_DATE_EPOCH="$target_epoch"
	umoci insert --image "${IMAGE}:${TAG}" "$insert_dir" /dummy
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"
	unset SOURCE_DATE_EPOCH

	# Unpack it (without SOURCE_DATE_EPOCH).
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Files newer than SOURCE_DATE_EPOCH should be set to SOURCE_DATE_EPOCH.
	[ "$(stat -c '%Y' "$ROOTFS/dummy")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/dummy/foo")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/dummy/foo/bar")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/dummy/foo/bar/baz")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/dummy/link")" -eq "$target_epoch" ]
	[ "$(stat -c '%Y' "$ROOTFS/dummy/newer")" -eq "$target_epoch" ]

	# Old files should remain the same.
	[ "$(stat -c '%Y' "$ROOTFS/dummy/older")" -eq "$old_file_epoch" ]

	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	# Make sure that the history entry uses SOURCE_DATE_EPOCH too.
	history_created_time="$(jq -SMr '.history[-1].created' <<<"$output")"
	[[ "$history_created_time" == "$target_date" ]]
}
