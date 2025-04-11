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

@test "umoci --log" {
	IMAGE="$(setup_tmpdir)/image" TAG="latest"

	umoci --verbose init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	# All log levels.
	umoci --log=debug new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci --log=info new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci --log=warn new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci --log=error new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	umoci --log=fatal new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	# Invalid --log arguments.
	umoci --log=foobar new --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	umoci --log=debug --verbose new --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
}

@test "umoci --cpu-profile" {
	CPU_PROFILE="$(setup_tmpdir)/umoci.profile"

	# Do some simple operation on the image.
	umoci --cpu-profile "$CPU_PROFILE" list --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	# Make sure go tool pprof at least succeeds.
	sane_run go tool pprof -top "$UMOCI" "$CPU_PROFILE"
	[ "$status" -eq 0 ]
}
