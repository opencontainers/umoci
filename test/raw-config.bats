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

@test "umoci raw runtime-config" {
	# Unpack the image.
	new_bundle_rootfs && BUNDLE_A="$BUNDLE"
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# We need to make sure the config exists.
	[ -f "$BUNDLE/config.json" ]

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE"
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure that the config was unchanged.
	# First clean the config.
	jq -SM '.' "$BUNDLE_A/config.json" >"$UMOCI_TMPDIR/a-config.json"
	jq -SM '.' "$BUNDLE_B/config.json" >"$UMOCI_TMPDIR/b-config.json"
	sane_run diff -u "$UMOCI_TMPDIR/a-config.json" "$UMOCI_TMPDIR/b-config.json"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "umoci raw runtime-config [invalid arguments]" {
	new_bundle_rootfs
	BUNDLE_CONFIG="$BUNDLE/config.json"

	# Missing --image and config argument.
	umoci raw runtime-config
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing --image argument.
	umoci raw runtime-config "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Missing config argument.
	umoci raw runtime-config --image "${IMAGE}:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci raw runtime-config --image ":${TAG}" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci raw runtime-config --image "${IMAGE}-doesnotexist:${TAG}" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci raw runtime-config --image "${IMAGE}:" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-doesnotexist" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci raw runtime-config --image "${IMAGE}:${INVALID_TAG}" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Unknown flag argument.
	umoci raw runtime-config --this-is-an-invalid-argument \
		--image="${IMAGE}:${TAG}" "$BUNDLE_CONFIG"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_CONFIG" \
		this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	! [ -e "$BUNDLE/config.json" ]
}

@test "umoci raw runtime-config --config.user 'user'" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$ROOTFS/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$ROOTFS/etc/group"
	echo "group:x:9001:testuser" >> "$ROOTFS/etc/group"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# We *don't* want the users to be set because we don't have a rootfs.

	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group'" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$ROOTFS/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$ROOTFS/etc/group"
	echo "group:x:9001:testuser" >> "$ROOTFS/etc/group"
	echo "emptygroup:x:2222:" >> "$ROOTFS/etc/group"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# We *don't* want the users to be set because we don't have a rootfs.

	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group' --rootfs" {
	# Unpack the image.
	new_bundle_rootfs && OLD_ROOTFS="$ROOTFS"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$ROOTFS/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$ROOTFS/etc/group"
	echo "group:x:9001:testuser" >> "$ROOTFS/etc/group"
	echo "emptygroup:x:2222:" >> "$ROOTFS/etc/group"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" --rootfs "$OLD_ROOTFS" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2222 ]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	(
		export "${lines[@]}"
		[[ "$HOME" == "/my home dir " ]]
	)

	# Modify /etc/passwd and /etc/group.
	sed -i -e 's|^testuser:x:1337:8888:test user:/my home dir :|testuser:x:3333:2321:a:/another  home:|' "$OLD_ROOTFS/etc/passwd"
	sed -i -e 's|^emptygroup:x:2222:|emptygroup:x:4444:|' "$OLD_ROOTFS/etc/group"

	# Unpack the image.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" --rootfs "$OLD_ROOTFS" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 3333 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 4444 ]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	(
		export "${lines[@]}"
		[[ "$HOME" == "/another  home" ]]
	)

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group' [non-existent user]" {
	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# We *don't* want the users to be set because we don't have a rootfs.

	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user [numeric]" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.user="1337:8888"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 8888 ]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.workingdir" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.workingdir "/a/fake/directory"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.cwd' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" = '"/a/fake/directory"' ]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.env" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --clear=config.env
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure that PATH and TERM are set.
	sane_run jq -SMr '.process.env | length' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == 2 ]]

	sane_run jq -SMr '.process.env[] | startswith("PATH=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[@]}" == *"true"* ]]

	sane_run jq -SMr '.process.env[] | startswith("TERM=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[@]}" == *"true"* ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.env" {
	# Modify env.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.env "VARIABLE1=unused"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the env again.
	umoci config --image "${IMAGE}:${TAG}-new" --config.env "VARIABLE1=test" --config.env "VARIABLE2=what"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure environment was set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure that they are all unique.
	numDefs="${#lines[@]}"
	numVars="$(echo "$output" | cut -d= -f1 | sort -u | wc -l)"
	[ "$numDefs" -eq "$numVars" ]

	# Set the variables.
	(
		export "${lines[@]}"
		[[ "$VARIABLE1" == "test" ]]
		[[ "$VARIABLE2" == "what" ]]
	)

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.{entrypoint or cmd}" {
	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.entrypoint "/here is some values/" --config.cmd "-c" --config.cmd "ls -la" --config.cmd="kek"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clear the entrypoint.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-noentry" --clear=config.entrypoint
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-noentry" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "-c;ls -la;kek;" ]]

	# Clear the cmd.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-nocmd" --clear=config.cmd
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-nocmd" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;/here is some values/;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.cmd" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.cmd "cat" --config.cmd "/this is a file with spaces" --config.cmd "-v"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "cat;/this is a file with spaces;-v;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.[entrypoint+cmd]" {
	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.entrypoint "/here is some values/" --config.cmd "-c" --config.cmd "ls -la" --config.cmd="kek"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clear the entrypoint and entrypoint.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-nocmdentry" --clear=config.entrypoint --clear=config.cmd
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-nocmdentry" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is empty.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	# TODO: This is almost certainly not going to be valid when config.json
	#	   conversion is part of the spec.
	[[ "$output" == "sh;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.[entrypoint+cmd]" {
	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.cmd "-c" --config.cmd "ls -la"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;-c;ls -la;" ]]

	image-verify "${IMAGE}"
}

# XXX: This test is somewhat dodgy (since we don't actually set anything other than the destination for a volume).
@test "umoci raw runtime-config --config.volume" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[@]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[@]}" | grep '^/some nutty/path name/ here$'

	# Make sure we're appending.
	umoci config --image "${IMAGE}:${TAG}" --config.volume "/another volume"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[@]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[@]}" | grep '^/some nutty/path name/ here$'
	printf -- '%s\n' "${lines[@]}" | grep '^/another volume$'

	# Now clear the volumes
	umoci config --image "${IMAGE}:${TAG}" --clear=config.volume --config.volume "/..final_volume"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	! ( printf -- '%s\n' "${lines[@]}" | grep '^/volume$' )
	! ( printf -- '%s\n' "${lines[@]}" | grep '^/some nutty/path name/ here$' )
	! ( printf -- '%s\n' "${lines[@]}" | grep '^/another volume$' )
	printf -- '%s\n' "${lines[@]}" | grep '^/\.\.final_volume$'

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --[os+architecture]" {
	# Modify none of the configuration.
	# XXX: We can't test anything other than --os=linux because our generator bails for non-Linux OSes.
	umoci config --image "${IMAGE}:${TAG}" --os "linux" --architecture "mips64"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Check that OS was set properly.
	# XXX: This has been removed, we need to add annotations for this.
	#	  See: https://github.com/opencontainers/image-spec/pull/711
	#sane_run jq -SMr '.platform.os' "$BUNDLE/config.json"
	#[ "$status" -eq 0 ]
	#[[ "$output" == "linux" ]]

	# Check that arch was set properly.
	# XXX: This has been removed, we need to add annotations for this.
	#	  See: https://github.com/opencontainers/image-spec/pull/711
	#sane_run jq -SMr '.platform.arch' "$BUNDLE/config.json"
	#[ "$status" -eq 0 ]
	#[[ "$output" == "mips64" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.label" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--clear=config.labels --clear=manifest.annotations \
		--config.label="com.cyphar.test=1" --config.label="com.cyphar.empty="
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	sane_run jq -SMr '.annotations["com.cyphar.test"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "1" ]]

	sane_run jq -SMr '.annotations["com.cyphar.empty"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.stopsignal" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--config.stopsignal="SIGUSR1"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	sane_run jq -SMr '.annotations["org.opencontainers.image.stopSignal"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${output}" == "SIGUSR1" ]]

	image-verify "${IMAGE}"
}
