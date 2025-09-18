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

function digest_to_path() {
	layout="$1"
	digest="$2"
	echo "$1/blobs/$(tr : / <<<"$2")"
}

@test "umoci stat --json" {
	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]

	statFile="$(setup_tmpdir)/stat"
	echo "$output" > "$statFile"

	# .manifest.descriptor should describe a config blob
	sane_run jq -SMr '.manifest.descriptor.mediaType' "$statFile"
	[ "$status" -eq 0 ]
	[[ "$output" == "application/vnd.oci.image.manifest.v1+json" ]]

	# .manifest.blob should match .manifest.descriptor data
	sane_run jq -SMr '.manifest.descriptor.digest' "$statFile"
	[ "$status" -eq 0 ]
	manifest_digest="$output"
	sane_run jq -SMr '.manifest.blob' "$statFile"
	[ "$status" -eq 0 ]
	manifest_data="$output"
	sane_run jq -SMr '.' "$(digest_to_path "$IMAGE" "$manifest_digest")"
	[ "$status" -eq 0 ]
	[[ "$output" == "$manifest_data" ]]

	# .config.descriptor should describe a config blob
	sane_run jq -SMr '.config.descriptor.mediaType' "$statFile"
	[ "$status" -eq 0 ]
	[[ "$output" == "application/vnd.oci.image.config.v1+json" ]]

	# .config.blob should match .config.descriptor data
	sane_run jq -SMr '.config.descriptor.digest' "$statFile"
	[ "$status" -eq 0 ]
	config_digest="$output"
	sane_run jq -SMr '.config.blob' "$statFile"
	[ "$status" -eq 0 ]
	config_data="$output"
	sane_run jq -SMr '.' "$(digest_to_path "$IMAGE" "$config_digest")"
	[ "$status" -eq 0 ]
	[[ "$output" == "$config_data" ]]

	# .history should have at least one entry.
	sane_run jq -SMr '.history | length' "$statFile"
	[ "$status" -eq 0 ]
	[ "$output" -ge 1 ]

	# There should be at least one non-empty_layer.
	sane_run jq -SMr '[.history[] | .empty_layer == null] | any' "$statFile"
	[ "$status" -eq 0 ]
	[[ "$output" == "true" ]]

	image-verify "${IMAGE}"
}

# We can't really test the output for non-JSON output, but we can smoke test it.
@test "umoci stat [smoke]" {
	# Set some values to make sure they show up in stat properly.
	umoci config --image "${IMAGE}:${TAG}" \
		--config.user "foobar" \
		--manifest.annotation "org.opencontainers.umoci.test=foo"
	[ "$status" -eq 0 ]

	# Make sure that stat looks about right.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	# We should have some manifest information.
	echo "$output" | grep "== MANIFEST =="
	echo "$output" | grep "Media Type: application/vnd.oci.image.manifest.v1+json"
	echo "$output" | grep "org.opencontainers.umoci.test: foo"
	echo "$output" | grep "org.opencontainers.image.ref.name: ${TAG}"

	# We should have some config information.
	echo "$output" | grep "== CONFIG =="
	echo "$output" | grep "Media Type: application/vnd.oci.image.config.v1+json"
	echo "$output" | grep "User: foobar"

	# We should have some history information.
	echo "$output" | grep "== HISTORY =="
	echo "$output" | grep 'LAYER'
	echo "$output" | grep 'CREATED'
	echo "$output" | grep 'CREATED BY'
	echo "$output" | grep 'SIZE'
	echo "$output" | grep 'COMMENT'

	image-verify "${IMAGE}"
}

BLANK_IMAGE_STAT="$(cat <<EOF
== MANIFEST ==
Schema Version: 2
Media Type: application/vnd.oci.image.manifest.v1+json
Config:
	Descriptor:
		Media Type: application/vnd.oci.image.config.v1+json
		Digest: sha256:e5101a46118c740a7709af8eaeec19cbc50a567f4fe7741f8420af39a3779a77
		Size: 135B
Descriptor:
	Media Type: application/vnd.oci.image.manifest.v1+json
	Digest: sha256:98a4b5d5fe4ea076a0a9059075dad54741e055fd0fa016903a8e2b858dcbad80
	Size: 249B
	Annotations:
		org.opencontainers.image.ref.name: latest

== CONFIG ==
Created: 2025-09-05T13:05:10.12345+10:00
Author: ""
Platform:
	OS: $(go env GOOS)
	Architecture: $(go env GOARCH)
Image Config:
	User: ""
	Command:
Descriptor:
	Media Type: application/vnd.oci.image.config.v1+json
	Digest: sha256:e5101a46118c740a7709af8eaeec19cbc50a567f4fe7741f8420af39a3779a77
	Size: 135B

== HISTORY ==
LAYER CREATED CREATED BY SIZE COMMENT
EOF
)"

@test "umoci stat [blank image output snapshot]" {
	IMAGE="$(setup_tmpdir)/image" TAG="latest"
	STATFILE_DIR="$(setup_tmpdir)"

	expected="${STATFILE_DIR}/expected"
	cat >"$expected" <<<"$BLANK_IMAGE_STAT"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	umoci config --no-history --created="2025-09-05T13:05:10.12345+10:00" --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	got="${STATFILE_DIR}/got"
	cat >"$got" <<<"$output"

	sane_run diff -u "$expected" "$got"
	[ "$status" -eq 0 ]
}

IMAGE_STAT="$(cat <<EOF
== MANIFEST ==
Schema Version: 2
Media Type: application/vnd.oci.image.manifest.v1+json
Config:
	Descriptor:
		Media Type: application/vnd.oci.image.config.v1+json
		Digest: sha256:01a6fc95c8afce1ebfaca585848ff2c42fe89ea0f5913c8e8fd6a0f8b691cd39
		Size: 1.107kB
Layers:
	Descriptor:
		Media Type: application/vnd.oci.image.layer.v1.tar+gzip
		Digest: sha256:088ca2b11e89a40e143aeb9d4564d0ffb69d26a380b3341af07fa28c2dbdaece
		Size: 238B
		Annotations:
			ci.umo.uncompressed_blob_size: 4608
	Descriptor:
		Media Type: application/vnd.oci.image.layer.v1.tar+zstd
		Digest: sha256:92226702a21491c696a910e56da759a2d6fe979211a7ad91c60ca715b89bc059
		Size: 119B
		Annotations:
			ci.umo.uncompressed_blob_size: 2048
Annotations:
	ci.umo.abc: "foobar\tbaz"
	ci.umo.xyz: hello world
Descriptor:
	Media Type: application/vnd.oci.image.manifest.v1+json
	Digest: sha256:58fa9705ecfc0ec7d1e8631a729c6cbab46206233d58e46fc55346ca9d25ed43
	Size: 737B
	Annotations:
		org.opencontainers.image.ref.name: latest

== CONFIG ==
Created: 2025-09-05T13:05:10.12345+10:00
Author: Aleksa Sarai <cyphar@cyphar.com>
Platform:
	OS: gnu+linux
	Architecture: riscv64
Image Config:
	User: foo:bar
	Entrypoint:
		/bin/sh
		-c
	Command:
		true
	Working Directory: /tmp
	Environment:
		HOME=/
		SHELL=/bin/false
		AAA=1234
		"ESCAPED=a\tb\nc\vd\re"
	Stop Signal: SIGKILL
	Exposed Ports: 80/tcp, 8080/udp
	Volumes: /tmp, /var, "/with, comma"
Descriptor:
	Media Type: application/vnd.oci.image.config.v1+json
	Digest: sha256:01a6fc95c8afce1ebfaca585848ff2c42fe89ea0f5913c8e8fd6a0f8b691cd39
	Size: 1.107kB

== HISTORY ==
LAYER                                                                   CREATED                         CREATED BY            SIZE   COMMENT
sha256:088ca2b11e89a40e143aeb9d4564d0ffb69d26a380b3341af07fa28c2dbdaece 1997-03-25T13:40:00Z            umoci insert          238B   basic insert
sha256:92226702a21491c696a910e56da759a2d6fe979211a7ad91c60ca715b89bc059 1997-03-25T13:42:00Z            umoci insert --opaque 119B   whiteout /
<none>                                                                  2025-09-05T13:05:10.12345+10:00 umoci config          <none> dummy configuration
EOF
)"

@test "umoci stat [output snapshot]" {
	IMAGE="$(setup_tmpdir)/image" TAG="latest"
	STATFILE_DIR="$(setup_tmpdir)"

	expected="${STATFILE_DIR}/expected"
	cat >"$expected" <<<"$IMAGE_STAT"

	umoci init --layout "${IMAGE}"
	[ "$status" -eq 0 ]

	umoci new --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	umoci config --no-history --image "${IMAGE}:${TAG}" \
		--author="Aleksa Sarai <cyphar@cyphar.com>" \
		--created="2025-09-05T13:05:10.12345+10:00" \
		--os="gnu+linux" \
		--architecture="riscv64"
	[ "$status" -eq 0 ]

	layer="$(setup_tmpdir)"
	echo "dummy data" >"$layer/file"
	ln -s "foo" "$layer/link"
	mkdir -p "$layer/foo/bar/baz"
	find "$layer" -print0 | xargs -0 touch -h -d "1997-03-25T13:40:00"

	umoci insert --image "${IMAGE}:${TAG}" \
		--history.author='Aleksa Sarai <cyphar@cyphar.com>' \
		--history.comment="basic insert" \
		--history.created="1997-03-25T13:40:00Z" \
		"$layer" /
	[ "$status" -eq 0 ]

	layer="$(setup_tmpdir)"
	find "$layer" -print0 | xargs -0 touch -h -d "1997-03-25T13:42:00"

	umoci insert --image "${IMAGE}:${TAG}" \
		--compress=zstd \
		--history.author=$'Foo Bar <foo\tbar\nbaz@fake.email>' \
		--history.comment="whiteout /" \
		--history.created_by="umoci insert --opaque" \
		--history.created="1997-03-25T13:42:00Z" \
		--opaque "$layer" /foo
	[ "$status" -eq 0 ]

	umoci config --image "${IMAGE}:${TAG}" \
		--config.user="foo:bar" \
		--config.exposedports=80/tcp \
		--config.exposedports=8080/udp \
		--config.env="HOME=/" \
		--config.env="SHELL=/bin/false" \
		--config.env="AAA=1234" \
		--config.env=$'ESCAPED=a\tb\nc\vd\re' \
		--config.entrypoint="/bin/sh" \
		--config.entrypoint="-c" \
		--config.cmd="true" \
		--config.volume="/tmp" \
		--config.volume="/var" \
		--config.volume="/with, comma" \
		--config.workingdir="/tmp" \
		--config.stopsignal="SIGKILL" \
		--manifest.annotation=$'ci.umo.abc=foobar\tbaz' \
		--manifest.annotation="ci.umo.xyz=hello world" \
		--history.created="2025-09-05T13:05:10.12345+10:00" \
		--history.author="Hello World <another dummy email@foo.com>" \
		--history.comment="dummy configuration"
	[ "$status" -eq 0 ]

	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]

	got="${STATFILE_DIR}/got"
	cat >"$got" <<<"$output"

	sane_run diff -u "$expected" "$got"
	[ "$status" -eq 0 ]
}

@test "umoci stat [invalid arguments]" {
	# Missing --image argument.
	umoci stat
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci stat --image "${IMAGE}:${TAG}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci stat --image ":${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci stat --image "${IMAGE}-doesnotexist:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci stat --image "${IMAGE}:"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci stat --image "${IMAGE}:${TAG}-doesnotexist"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci stat --image "${IMAGE}:${INVALID_TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci stat --this-is-an-invalid-argument --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci stat --image "${IMAGE}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

# TODO: Add a test to make sure that empty_layer and layer are mutually
#	   exclusive. Unfortunately, jq doesn't provide an XOR operator...
