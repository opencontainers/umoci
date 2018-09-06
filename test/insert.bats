#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2018 Cisco
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

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/some" /rootless
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack after the inserts.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
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
}
