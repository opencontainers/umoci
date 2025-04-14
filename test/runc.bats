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
	setup_runc
}

function teardown() {
	teardown_tmpdirs
	teardown_image
	teardown_runc
}

@test "umoci + runc [smoke test]" {
	CTR_NAME="umoci-test-$RANDOM"
	LOGFILE="$(setup_tmpdir)/umoci-runc-debug.log"

	# Modify the image so that we run a command that's easy to check.
	umoci config --image "${IMAGE}:${TAG}" \
		--config.entrypoint "/bin/echo" --config.cmd "hello umoci"
	[ "$status" -eq 0 ]
	image-verify "$IMAGE"

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Bats runs without stdin, which causes runc to be quite unhappy. See
	# <https://github.com/opencontainers/runc/issues/2485> for more details.
	jq '.process.terminal = false' "$BUNDLE/config.json" | sponge "$BUNDLE/config.json"

	# Run the container.
	runc --debug --log "$LOGFILE" run -b "$BUNDLE" "$CTR_NAME"
	# Get the logfile contents.
	echo "=== [runc --debug logfile]" >&2
	cat "$LOGFILE" >&2
	echo "===" >&2
	[ "$status" -eq 0 ]
	[[ "$output" == "hello umoci" ]]
}
