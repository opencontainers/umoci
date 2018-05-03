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
	image-verify "${IMAGE}"

	# fail with too few arguments
	umoci insert --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]

	# ...and too many
	umoci insert --image "${IMAGE}:${TAG}" asdf 123 456
	[ "$status" -ne 0 ]

	INSERTDIR=$(setup_tmpdir)
	mkdir -p "${INSERTDIR}/test"
	touch "${INSERTDIR}/test/a"
	touch "${INSERTDIR}/test/b"
	chmod +x "${INSERTDIR}/test/b"

	# do a few inserts
	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/a" /tester/a
	[ "$status" -eq 0 ]

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/b" /tester/b
	[ "$status" -eq 0 ]

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test" /recursive
	[ "$status" -eq 0 ]

	# ...and check to make sure it worked
	BUNDLE=$(setup_tmpdir)
	umoci unpack --image "${IMAGE}:${TAG}" "${BUNDLE}"
	[ -f "${BUNDLE}/rootfs/tester/a" ]
	[ "$(stat --format=%f ${INSERTDIR}/test/b)" == "$(stat --format=%f ${BUNDLE}/rootfs/tester/b)" ]
	[ -f "${BUNDLE}/rootfs/recursive/a" ]
	[ -f "${BUNDLE}/rootfs/recursive/b" ]
}

@test "umoci insert rootless" {
	image-verify "${IMAGE}"

	BUNDLE=$(setup_tmpdir)
	umoci unpack --image "${IMAGE}:${TAG}" "${BUNDLE}"

	mkdir -p $BUNDLE/rootfs/some/path
	chmod 000 $BUNDLE/rootfs/some/path
	umoci repack "${IMAGE}:${TAG}" "${BUNDLE}"

	INSERTDIR=$(setup_tmpdir)
	mkdir -p "${INSERTDIR}/test"
	touch "${INSERTDIR}/test/a"

	umoci insert --image "${IMAGE}:${TAG}" "${INSERTDIR}/test/a" /some/path/a
	[ "$status" -eq 0 ]

	BUNDLE_B=$(setup_tmpdir)
	umoci unpack --image "${IMAGE}:${TAG}" "${BUNDLE_B}"

	[ -f "${BUNDLE_B}/rootfs/some/path/a" ]
}
