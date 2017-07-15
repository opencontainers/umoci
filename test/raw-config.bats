#!/usr/bin/env bats -t
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

load helpers

function setup() {
	setup_image
}

function teardown() {
	teardown_tmpdirs
	teardown_image
}

@test "umoci raw runtime-config" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Make sure that the config was unchanged.
	# First clean the config.
	jq -SM '.' "$BUNDLE_A/config.json" >"$BATS_TMPDIR/a-config.json"
	jq -SM '.' "$BUNDLE_B/config.json" >"$BATS_TMPDIR/b-config.json"
	sane_run diff -u "$BATS_TMPDIR/a-config.json" "$BATS_TMPDIR/b-config.json"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "umoci raw runtime-config [missing args]" {
	umoci config
	[ "$status" -ne 0 ]
}

@test "umoci raw runtime-config --config.user 'user'" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# We *don't* want the users to be set because we don't have a rootfs.

	sane_run jq -SM '.process.user.uid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SM '.process.user.gid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group'" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "emptygroup:x:2222:" >> "$BUNDLE_A/rootfs/etc/group"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# We *don't* want the users to be set because we don't have a rootfs.

	sane_run jq -SM '.process.user.uid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SM '.process.user.gid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 0 ]
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group' --rootfs" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	image-verify "${IMAGE}"

	# Unpack the image.
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE_A"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE_A"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "emptygroup:x:2222:" >> "$BUNDLE_A/rootfs/etc/group"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" --rootfs "$BUNDLE_A/rootfs" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2222 ]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	export $output
	[[ "$HOME" == "/my home dir " ]]

	# Modify /etc/passwd and /etc/group.
	sed -i -e 's|^testuser:x:1337:8888:test user:/my home dir :|testuser:x:3333:2321:a:/another  home:|' "$BUNDLE_A/rootfs/etc/passwd"
	sed -i -e 's|^emptygroup:x:2222:|emptygroup:x:4444:|' "$BUNDLE_A/rootfs/etc/group"

	# Unpack the image.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" --rootfs "$BUNDLE_A/rootfs" "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 3333 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 4444 ]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]
	export $output
	[[ "$HOME" == "/another  home" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.user 'user:group' [non-existent user]" {
	BUNDLE="$(setup_tmpdir)"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Generate config.json.
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
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.user="1337:8888"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
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
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.workingdir "/a/fake/directory"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.cwd' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" = '"/a/fake/directory"' ]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.env" {
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --clear=config.env
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Make sure that PATH and TERM are set.
	sane_run jq -SMr '.process.env | length' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == 2 ]]

	sane_run jq -SMr '.process.env[] | startswith("PATH=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[*]}" == *"true"* ]]

	sane_run jq -SMr '.process.env[] | startswith("TERM=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[*]}" == *"true"* ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.env" {
	BUNDLE="$(setup_tmpdir)"

	# Modify env.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.env "VARIABLE1=unused"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the env again.
	umoci config --image "${IMAGE}:${TAG}-new" --config.env "VARIABLE1=test" --config.env "VARIABLE2=what"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
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
	export $output
	[[ "$VARIABLE1" == "test" ]]
	[[ "$VARIABLE2" == "what" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.{entrypoint or cmd}" {
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"

	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.entrypoint "/here is some values/" --config.cmd "-c" --config.cmd "ls -la" --config.cmd="kek"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clear the entrypoint.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-noentry" --clear=config.entrypoint
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-noentry" "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "-c;ls -la;kek;" ]]

	# Clear the cmd.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-nocmd" --clear=config.cmd
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-nocmd" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;/here is some values/;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.cmd" {
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.cmd "cat" --config.cmd "/this is a file with spaces" --config.cmd "-v"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "cat;/this is a file with spaces;-v;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --clear=config.[entrypoint+cmd]" {
	BUNDLE="$(setup_tmpdir)"

	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.entrypoint "/here is some values/" --config.cmd "-c" --config.cmd "ls -la" --config.cmd="kek"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clear the entrypoint and entrypoint.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-nocmdentry" --clear=config.entrypoint --clear=config.cmd
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-nocmdentry" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Ensure that the final args is empty.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	# TODO: This is almost certainly not going to be valid when config.json
	#       conversion is part of the spec.
	[[ "$output" == "sh;" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.[entrypoint+cmd]" {
	BUNDLE="$(setup_tmpdir)"

	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.cmd "-c" --config.cmd "ls -la"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
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
	BUNDLE_A="$(setup_tmpdir)"
	BUNDLE_B="$(setup_tmpdir)"
	BUNDLE_C="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[*]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$'

	# Make sure we're appending.
	umoci config --image "${IMAGE}:${TAG}" --config.volume "/another volume"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[*]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$'
	printf -- '%s\n' "${lines[*]}" | grep '^/another volume$'

	# Now clear the volumes
	umoci config --image "${IMAGE}:${TAG}" --clear=config.volume --config.volume "/..final_volume"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/volume$' )
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$' )
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/another volume$' )
	printf -- '%s\n' "${lines[*]}" | grep '^/\.\.final_volume$'

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --[os+architecture]" {
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	# XXX: We can't test anything other than --os=linux because our generator bails for non-Linux OSes.
	umoci config --image "${IMAGE}:${TAG}" --os "linux" --architecture "mips64"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	# Check that OS was set properly.
	# XXX: This has been removed, we need to add annotations for this.
	#      See: https://github.com/opencontainers/image-spec/pull/711
	#sane_run jq -SMr '.platform.os' "$BUNDLE/config.json"
	#[ "$status" -eq 0 ]
	#[[ "$output" == "linux" ]]

	# Check that arch was set properly.
	# XXX: This has been removed, we need to add annotations for this.
	#      See: https://github.com/opencontainers/image-spec/pull/711
	#sane_run jq -SMr '.platform.arch' "$BUNDLE/config.json"
	#[ "$status" -eq 0 ]
	#[[ "$output" == "mips64" ]]

	image-verify "${IMAGE}"
}

@test "umoci raw runtime-config --config.label" {
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--clear=config.labels --clear=manifest.annotations \
		--config.label="com.cyphar.test=1" --config.label="com.cyphar.empty="
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
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
	BUNDLE="$(setup_tmpdir)"

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--config.stopsignal="SIGUSR1"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	umoci raw runtime-config --image "${IMAGE}:${TAG}-new" "$BUNDLE/config.json"
	[ "$status" -eq 0 ]

	sane_run jq -SMr '.annotations["org.opencontainers.image.stopSignal"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${output}" == "SIGUSR1" ]]

	image-verify "${IMAGE}"
}
