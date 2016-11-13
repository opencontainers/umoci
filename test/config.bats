#!/usr/bin/env bats -t
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016 SUSE LLC.
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

BUNDLE_A="$BATS_TMPDIR/bundle.a"
BUNDLE_B="$BATS_TMPDIR/bundle.b"
BUNDLE_C="$BATS_TMPDIR/bundle.c"

function setup() {
	setup_image
}

function teardown() {
	teardown_image
	rm -rf "$BUNDLE_A"
	rm -rf "$BUNDLE_B"
	rm -rf "$BUNDLE_C"
}

@test "umoci config" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "$TAG" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# We need to make sure the config exists.
	[ -f "$BUNDLE_A/config.json" ]

	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Make sure that the config was unchanged.
	# First clean the config.
	jq -SM '.' "$BUNDLE_A/config.json" >"$BATS_TMPDIR/a-config.json"
	jq -SM '.' "$BUNDLE_B/config.json" >"$BATS_TMPDIR/b-config.json"
	sane_run diff -u "$BATS_TMPDIR/a-config.json" "$BATS_TMPDIR/b-config.json"
	[ "$status" -eq 0 ]
	[ -z "$output" ]
}

@test "umoci config --config.user 'user'" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"

	# Repack the image.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Modify the user.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.user="testuser"
	[ "$status" -eq 0 ]

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 8888 ]

	# Make sure additionalGids were set.
	sane_run jq -SMr '.process.user.additionalGids[]' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "${#lines[@]}" -eq 2 ]

	# Check mounts.
	printf -- '%s\n' "${lines[*]}" | grep '^9001$'
	printf -- '%s\n' "${lines[*]}" | grep '^2581$'

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	export $output
	[[ "$HOME" == "/my home dir " ]]
}

@test "umoci config --config.user 'user:group'" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "emptygroup:x:2222:" >> "$BUNDLE_A/rootfs/etc/group"

	# Repack the image.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Modify the user.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2222 ]

	# Make sure additionalGids were not set.
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]
	export $output
	[[ "$HOME" == "/my home dir " ]]
}

@test "umoci config --config.user 'user:group' [parsed from rootfs]" {
	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$BUNDLE_A/rootfs/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "group:x:9001:testuser" >> "$BUNDLE_A/rootfs/etc/group"
	echo "emptygroup:x:2222:" >> "$BUNDLE_A/rootfs/etc/group"

	# Repack the image.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A" --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Modify the user.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
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
	sed -i -e 's|^testuser:x:1337:8888:test user:/my home dir :|testuser:x:3333:2321:a:/another  home:|' "$BUNDLE_B/rootfs/etc/passwd"
	sed -i -e 's|^emptygroup:x:2222:|emptygroup:x:4444:|' "$BUNDLE_B/rootfs/etc/group"

	# Repack the image.
	umoci repack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B" --tag "${TAG}"
	[ "$status" -eq 0 ]

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_C"
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
}

@test "umoci config --config.user 'user:group' [non-existent user]" {
	# Modify the user.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]

	# Unpack the image.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
	[ "$status" -ne 0 ]
}

@test "umoci config --config.user [numeric]" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.user="1337:8888"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 8888 ]
}

# TODO: Add a test to make sure that --config.user is resolved on unpacking.
# TODO: Add further tests for --config.user resolution (and additional_gids).

@test "umoci config --config.workingdir" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.workingdir "/a/fake/directory"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.cwd' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" = '"/a/fake/directory"' ]
}

@test "umoci config --clear=config.env" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --clear=config.env
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure that only $HOME was set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == *"HOME="* ]]
	[ "${#lines[@]}" -eq 1 ]
}

@test "umoci config --config.env" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.env "VARIABLE1=test" --config.env "VARIABLE2=what"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure numeric config was actually set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Set the variables.
	export $output
	[[ "$VARIABLE1" == "test" ]]
	[[ "$VARIABLE2" == "what" ]]
}

@test "umoci config --config.memory.*" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.memory.limit 1000 --config.memory.swap 2000
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure memory.limit and memory.reservation are set.
	sane_run jq -SMr '.linux.resources.memory.limit' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1000 ]
	sane_run jq -SMr '.linux.resources.memory.reservation' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1000 ]

	# Make sure memory.swap was set.
	sane_run jq -SMr '.linux.resources.memory.swap' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2000 ]
}

@test "umoci config --config.cpu.shares" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}-new" --config.cpu.shares 1024
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}-new" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Make sure memory.limit and memory.reservation are set.
	sane_run jq -SMr '.linux.resources.cpu.shares' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1024 ]
}

@test "umoci config --config.cmd" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.cmd "cat" --config.cmd "/this is a file with spaces" --config.cmd "-v"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "cat;/this is a file with spaces;-v;" ]]
}

@test "umoci config --config.[entrypoint+cmd]" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.entrypoint "sh" --config.cmd "-c" --config.cmd "ls -la"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;-c;ls -la;" ]]
}

# XXX: This test is somewhat dodgy (since we don't actually set anything other than the destination for a volume).
@test "umoci config --config.volume" {
	# Modify none of the configuration.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[*]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$'

	# Make sure we're appending.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --config.volume "/another volume"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_B"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_B/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	printf -- '%s\n' "${lines[*]}" | grep '^/volume$'
	printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$'
	printf -- '%s\n' "${lines[*]}" | grep '^/another volume$'

	# Now clear the volumes
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --clear=config.volume --config.volume "/..final_volume"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_C"
	[ "$status" -eq 0 ]

	# Get set of mounts
	sane_run jq -SMr '.mounts[] | .destination' "$BUNDLE_C/config.json"
	[ "$status" -eq 0 ]

	# Check mounts.
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/volume$' )
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/some nutty/path name/ here$' )
	! ( printf -- '%s\n' "${lines[*]}" | grep '^/another volume$' )
	printf -- '%s\n' "${lines[*]}" | grep '^/\.\.final_volume$'
}

@test "umoci config --[os+architecture]" {
	# Modify none of the configuration.
	# XXX: We can't test anything other than --os=linux because our generator bails for non-Linux OSes.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --os "linux" --architecture "aarch9001"
	[ "$status" -eq 0 ]

	# Unpack the image again.
	umoci unpack --image "$IMAGE" --from "${TAG}" --bundle "$BUNDLE_A"
	[ "$status" -eq 0 ]

	# Check that OS was set properly.
	sane_run jq -SMr '.platform.os' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "linux" ]]

	# Check that arch was set properly.
	sane_run jq -SMr '.platform.arch' "$BUNDLE_A/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "aarch9001" ]]
}

# XXX: This doesn't do any actual testing of the results of any of these flags.
# This needs to be fixed after we implement raw-cat or something like that.
@test "umoci config --[author+created+history]" {
	# Modify everything.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --author="Aleksa Sarai <asarai@suse.com>" --created="2016-03-25T12:34:02.655002+11:00" \
	             --clear=history --history '{"created_by": "ls -la", "comment": "should work", "author": "me", "empty_layer": false, "created": "2016-03-25T12:34:02.655002+11:00"}' \
	             --history '{"created_by": "ls -la", "author": "me", "empty_layer": false}'
	[ "$status" -eq 0 ]

	# Make sure that --history doesn't work with a random string.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --history "some random string"
	[ "$status" -ne 0 ]
	# FIXME It turns out that Go's JSON parser will ignore unknown keys...
	#umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --history '{"unknown key": 12, "created_by": "ls -la", "comment": "should not work"}'
	#[ "$status" -ne 0 ]

	# Make sure that --created doesn't work with a random string.
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --created="not a date"
	[ "$status" -ne 0 ]
	umoci config --image "$IMAGE" --from "$TAG" --tag "${TAG}" --created="Jan 04 2004"
	[ "$status" -ne 0 ]
}
