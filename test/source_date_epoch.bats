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

@test "umoci new with SOURCE_DATE_EPOCH" {
	IMAGE_DIR="$(setup_tmpdir)/image"

	umoci init --layout "${IMAGE_DIR}"
	[ "$status" -eq 0 ]

	export SOURCE_DATE_EPOCH=1234567890 # Feb 13, 2009 23:31:30 UTC
	umoci new --image "${IMAGE_DIR}:test-new"
	[ "$status" -eq 0 ]

	created_time=$(cat "${IMAGE_DIR}"/blobs/sha256/* | jq -r 'select(.created) | .created')
	[ "$created_time" = "2009-02-13T23:31:30Z" ]

	unset SOURCE_DATE_EPOCH
}


@test "umoci new without SOURCE_DATE_EPOCH uses current time" {
	IMAGE_DIR="$(setup_tmpdir)/image"

	unset SOURCE_DATE_EPOCH

	umoci init --layout "${IMAGE_DIR}"
	[ "$status" -eq 0 ]

	before_time=$(date -u +%s)

	umoci new --image "${IMAGE_DIR}:test-current"
	[ "$status" -eq 0 ]

	after_time=$(date -u +%s)

	created_time=$(cat "${IMAGE_DIR}"/blobs/sha256/* | jq -r 'select(.created) | .created')
	created_timestamp=$(date -d "$created_time" +%s)

	[ "$created_timestamp" -ge "$before_time" ]
	[ "$created_timestamp" -le "$((after_time + 5))" ]
}