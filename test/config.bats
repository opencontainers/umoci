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

# TODO: Add "umoci stat" calls to double-check the config is set properly...

@test "umoci config" {
	# Unpack the image.
	new_bundle_rootfs && BUNDLE_A="$BUNDLE"
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# We need to make sure the config exists.
	[ -f "$BUNDLE/config.json" ]

	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image again.
	new_bundle_rootfs && BUNDLE_B="$BUNDLE"
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the config was unchanged.
	# First clean the config.
	jq -SM '.' "$BUNDLE_A/config.json" >"$UMOCI_TMPDIR/a-config.json"
	jq -SM '.' "$BUNDLE_B/config.json" >"$UMOCI_TMPDIR/b-config.json"
	sane_run diff -u "$UMOCI_TMPDIR/a-config.json" "$UMOCI_TMPDIR/b-config.json"
	[ "$status" -eq 0 ]
	[ -z "$output" ]

	# Make sure that the history was modified.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SM '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SM '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# The final layer should be an empty_layer now.
	[[ "$(echo "$output" | jq -SM '.history[-1].empty_layer')" == "true" ]]

	image-verify "${IMAGE}"
}

@test "umoci config [invalid arguments]" {
	# Missing --image argument.
	umoci config
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Too many positional arguments.
	umoci config --image "${IMAGE}:${TAG}" this-is-an-invalid-argument
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image path.
	umoci config --image ":${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image path.
	umoci config --image "${IMAGE}-doesnotexist:${TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image source tag.
	umoci config --image "${IMAGE}:"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Non-existent image source tag.
	umoci config --image "${IMAGE}:${TAG}-doesnotexist"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image source tag.
	umoci config --image "${IMAGE}:${INVALID_TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Empty image destination tag.
	umoci config --image "${IMAGE}:${TAG}" --tag ""
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Invalid image destination tag.
	umoci config --image "${IMAGE}:${TAG}" --tag "${INVALID_TAG}"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --no-history conflicts with --history.* flags.
	umoci config --image "${IMAGE}:${TAG}" \
		--no-history --history.author "Violet Beauregarde"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# --history.created has to be an ISO-8601 date.
	umoci config --image "${IMAGE}:${TAG}" \
		--history.created "invalid date"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
}

@test "umoci config --config.user 'user'" {
	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Modify /etc/passwd and /etc/group.
	echo "testuser:x:1337:8888:test user:/my home dir :/bin/sh" >> "$ROOTFS/etc/passwd"
	echo "testgroup:x:2581:root,testuser" >> "$ROOTFS/etc/group"
	echo "group:x:9001:testuser" >> "$ROOTFS/etc/group"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 8888 ]

	# Make sure additionalGids were set.
	sane_run jq -SMr '.process.user.additionalGids | length' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	if [ "$IS_ROOTLESS" -eq 0 ]; then
		[[ "$output" == 2 ]]

		# Check the actual values.
		sane_run jq -SMr '.process.user.additionalGids[]' "$BUNDLE/config.json"
		[ "$status" -eq 0 ]
		printf -- '%s\n' "${lines[@]}" | grep '^9001$'
		printf -- '%s\n' "${lines[@]}" | grep '^2581$'
	else
		# In rootless containers additionalGids should be empty.
		[[ "$output" == 0 ]]
	fi

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	(
		export "${lines[@]}"
		[[ "$HOME" == "/my home dir " ]]
	)

	image-verify "${IMAGE}"
}

@test "umoci config --config.user 'user:group'" {
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

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *"User: testuser:emptygroup"* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.User' <<<"$output"
	[[ "$output" == "testuser:emptygroup" ]]

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.uid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 1337 ]

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.user.gid' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" -eq 2222 ]

	# Make sure additionalGids were not set.
	sane_run jq -SMr '.process.user.additionalGids' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "null" ]]

	# Check that HOME is set.
	sane_run jq -SMr '.process.env[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	(
		export "${lines[@]}"
		[[ "$HOME" == "/my home dir " ]]
	)

	image-verify "${IMAGE}"
}

@test "umoci config --config.user 'user:group' [parsed from rootfs]" {
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

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *"User: testuser:emptygroup"* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.User' <<<"$output"
	[[ "$output" == "testuser:emptygroup" ]]

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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
	sed -i -e 's|^testuser:x:1337:8888:test user:/my home dir :|testuser:x:3333:2321:a:/another  home:|' "$ROOTFS/etc/passwd"
	sed -i -e 's|^emptygroup:x:2222:|emptygroup:x:4444:|' "$ROOTFS/etc/group"

	# Repack the image.
	umoci repack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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

@test "umoci config --config.user 'user:group' [non-existent user]" {
	# Modify the user.
	umoci config --image "${IMAGE}:${TAG}" --config.user="testuser:emptygroup"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *"User: testuser:emptygroup"* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.User' <<<"$output"
	[[ "$output" == "testuser:emptygroup" ]]

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -ne 0 ]

	image-verify "${IMAGE}"
}

@test "umoci config --config.user [numeric]" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.user="1337:8888"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *"User: 1337:8888"* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.User' <<<"$output"
	[[ "$output" == "1337:8888" ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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

@test "umoci config --config.workingdir" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.workingdir "/a/fake/directory"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Working Directory: /a/fake/directory"* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.WorkingDir' <<<"$output"
	[[ "$output" == "/a/fake/directory" ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure numeric config was actually set.
	sane_run jq -SM '.process.cwd' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[ "$output" = '"/a/fake/directory"' ]

	image-verify "${IMAGE}"
}

@test "umoci config --clear=config.env" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --clear=config.env
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	grep -v 'Environment:' <<<"$output"
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -SMr '.config.blob.config.Env | length' <<<"$output"
	[[ "$output" == "0" ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that HOME, PATH and TERM are set
	sane_run jq -SMr '.process.env | length' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == 3 ]]

	sane_run jq -SMr '.process.env[] | startswith("HOME=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[@]}" == *"true"* ]]

	sane_run jq -SMr '.process.env[] | startswith("PATH=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[@]}" == *"true"* ]]

	sane_run jq -SMr '.process.env[] | startswith("TERM=")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[@]}" == *"true"* ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.env" {
	# Modify env.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --config.env "VARIABLE1=unused"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Environment:"* ]]
	[[ "$output" == *"VARIABLE1=unused"* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -cM '.config.blob.config.Env | map(select(startswith("VARIABLE")))' <<<"$output"
	[[ "$output" == '["VARIABLE1=unused"]' ]]

	# Modify the env again.
	umoci config --image "${IMAGE}:${TAG}-new" --config.env "VARIABLE2=what" --config.env "VARIABLE1=test"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Environment:"* ]]
	[[ "$output" == *"VARIABLE1=test"* ]]
	[[ "$output" == *"VARIABLE2=what"* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -cM '.config.blob.config.Env | map(select(startswith("VARIABLE")))' <<<"$output"
	[[ "$output" == '["VARIABLE1=test","VARIABLE2=what"]' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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

@test "umoci config --clear=config.{entrypoint or cmd}" {
	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.entrypoint "/here is some values/" --config.cmd "-c" --config.cmd "ls -la" --config.cmd="kek"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Clear the entrypoint.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-noentry" --clear=config.entrypoint
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-noentry"
	[ "$status" -eq 0 ]
	grep -v 'Entrypoint:' <<<"$output"
	umoci stat --image "${IMAGE}:${TAG}-noentry" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config | [.Entrypoint, .Cmd] | map(length)' <<<"$output"
	[[ "$output" == "[0,3]" ]]

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-noentry" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "-c;ls -la;kek;" ]]

	# Clear the cmd.
	umoci config --image "${IMAGE}:${TAG}" --tag="${TAG}-nocmd" --clear=config.cmd
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-nocmd"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Entrypoint:"* ]]
	[[ "$output" == *'sh'* ]]
	[[ "$output" == *'"/here is some values/"'* ]]
	umoci stat --image "${IMAGE}:${TAG}-nocmd" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config | [.Entrypoint, .Cmd] | map(length)' <<<"$output"
	[[ "$output" == "[2,0]" ]]

	# Unpack the image.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-nocmd" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the final args is only cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;/here is some values/;" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.cmd" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.cmd "cat" --config.cmd "/this is a file with spaces" --config.cmd "-v"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *'cat'* ]]
	[[ "$output" == *'"/this is a file with spaces"'* ]]
	[[ "$output" == *'-v'* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config.Cmd' <<<"$output"
	[[ "$output" == '["cat","/this is a file with spaces","-v"]' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "cat;/this is a file with spaces;-v;" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --clear=config.[entrypoint+cmd]" {
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
	umoci unpack --image "${IMAGE}:${TAG}-nocmdentry" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the final args is empty.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	# TODO: This is almost certainly not going to be valid when config.json
	#	   conversion is part of the spec.
	[[ "$output" == "sh;" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.[entrypoint+cmd]" {
	# Modify the entrypoint+cmd.
	umoci config --image "${IMAGE}:${TAG}" --config.entrypoint "sh" --config.cmd "-c" --config.cmd "ls -la"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Entrypoint:"* ]]
	[[ "$output" == *'sh'* ]]
	[[ "$output" == *'-c'* ]]
	[[ "$output" == *'"ls -la"'* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	statJSON="$output"
	sane_run jq -cSMr '.config.blob.config.Entrypoint' <<<"$statJSON"
	[[ "$output" == '["sh"]' ]]
	sane_run jq -cSMr '.config.blob.config.Cmd' <<<"$statJSON"
	[[ "$output" == '["-c","ls -la"]' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Ensure that the final args is entrypoint+cmd.
	sane_run jq -SMr 'reduce .process.args[] as $arg (""; . + $arg + ";")' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "sh;-c;ls -la;" ]]

	image-verify "${IMAGE}"
}

# XXX: This test is somewhat dodgy (since we don't actually set anything other than the destination for a volume).
@test "umoci config --config.volume" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --config.volume /volume --config.volume "/some nutty/path name/ here"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Volumes: "/some nutty/path name/ here", /volume'* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config.Volumes' <<<"$output"
	[[ "$output" == '{"/some nutty/path name/ here":{},"/volume":{}}' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

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

@test "umoci config --[os+architecture]" {
	# Modify none of the configuration.
	# XXX: We can't test anything other than --os=linux because our generator
	#      bails for non-Linux OSes.
	umoci config --image "${IMAGE}:${TAG}" --os "linux" --architecture "mips64"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Platform:"* ]]
	[[ "$output" == *'OS: linux'* ]]
	[[ "$output" == *'Architecture: mips64'* ]]
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	statJSON="$output"
	sane_run jq -Mr '.config.blob.os' <<<"$statJSON"
	[[ "$output" == 'linux' ]]
	sane_run jq -Mr '.config.blob.architecture' <<<"$statJSON"
	[[ "$output" == 'mips64' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Check that OS was set properly.
	sane_run jq -SMr '.annotations["org.opencontainers.image.os"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "linux" ]]

	# Check that arch was set properly.
	sane_run jq -SMr '.annotations["org.opencontainers.image.architecture"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "mips64" ]]

	# Check that variant is *not* set.
	sane_run jq -SM '.annotations["org.opencontainers.image.variant"] == null' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "true" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --[author+created]" {
	# Modify everything.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --author="Aleksa Sarai <cyphar@cyphar.com>" --created="2016-03-25T12:34:02.655002+11:00"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the same data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *"Created: 2016-03-25T12:34:02.655002+11:00"* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -Mr '.config.blob.created' <<<"$output"
	[[ "$output" == '2016-03-25T12:34:02.655002+11:00' ]]

	# Make sure that --created doesn't work with a random string.
	umoci config --image "${IMAGE}:${TAG}" --created="not a date"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"
	umoci config --image "${IMAGE}:${TAG}" --created="Jan 04 2004"
	[ "$status" -ne 0 ]
	image-verify "${IMAGE}"

	# Make sure that the history was modified and the author is now me.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SMr '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SMr '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# The final layer should be an empty_layer now.
	[[ "$(echo "$output" | jq -SMr '.history[-1].empty_layer')" == "true" ]]
	# The author should've changed.
	[[ "$(echo "$output" | jq -SMr '.history[-1].author')" == "Aleksa Sarai <cyphar@cyphar.com>" ]]

	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the author gets filled when we extract as well.
	# NOTE: If this check breaks, it's because the image has a config.Labels
	#       entry that overrides the value.
	sane_run jq -SMr '.annotations["org.opencontainers.image.author"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "Aleksa Sarai <cyphar@cyphar.com>" ]]

	image-verify "${IMAGE}"
}

# XXX: We don't do any testing of --author and that the config is changed properly.
@test "umoci config --history.*" {
	# Modify something and set the history values.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--history.author="Not Aleksa <someone@else.com>" \
		--history.comment="/* Not a real comment. */" \
		--history.created_by="-- <bats> integration test --" \
		--history.created="2016-12-09T04:45:40+11:00" \
		--author="Aleksa Sarai <cyphar@cyphar.com>"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the correct author.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Author: "Aleksa Sarai <cyphar@cyphar.com>"'* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -Mr '.config.blob.author' <<<"$output"
	[[ "$output" == 'Aleksa Sarai <cyphar@cyphar.com>' ]]

	# Make sure that the history was modified.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	numLinesA="$(echo "$output" | jq -SMr '.history | length')"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	numLinesB="$(echo "$output" | jq -SMr '.history | length')"

	# Number of lines should be greater.
	[ "$numLinesB" -gt "$numLinesA" ]
	# The final layer should be an empty_layer now.
	[[ "$(echo "$output" | jq -SMr '.history[-1].empty_layer')" == "true" ]]
	# The author should've changed to --history.author.
	[[ "$(echo "$output" | jq -SMr '.history[-1].author')" == "Not Aleksa <someone@else.com>" ]]
	# The comment should be added.
	[[ "$(echo "$output" | jq -SMr '.history[-1].comment')" == "/* Not a real comment. */" ]]
	# The created_by should be set.
	[[ "$(echo "$output" | jq -SMr '.history[-1].created_by')" == "-- <bats> integration test --" ]]
	# The created should be set.
	[[ "$(echo "$output" | jq -SMr '.history[-1].created')" == "2016-12-09T04:45:40+11:00" ]]

	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the author gets filled when we extract as well.
	sane_run jq -SMr '.annotations["org.opencontainers.image.author"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "Aleksa Sarai <cyphar@cyphar.com>" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --no-history" {
	# Modify something and don't add a history entry.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" --no-history \
		--author="Aleksa Sarai <cyphar@cyphar.com>"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the correct author.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Author: "Aleksa Sarai <cyphar@cyphar.com>"'* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -Mr '.config.blob.author' <<<"$output"
	[[ "$output" == 'Aleksa Sarai <cyphar@cyphar.com>' ]]

	# Make sure we *did not* add a new history entry.
	umoci stat --image "${IMAGE}:${TAG}" --json
	[ "$status" -eq 0 ]
	hashA="$(jq '.history' <<<"$output" | sha256sum)"

	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	hashB="$(jq '.history' <<<"$output" | sha256sum)"

	# umoci-stat history output should be identical.
	[[ "$hashA" == "$hashB" ]]

	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	# Make sure that the author gets filled when we extract as well.
	sane_run jq -SMr '.annotations["org.opencontainers.image.author"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "Aleksa Sarai <cyphar@cyphar.com>" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.label" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--clear=config.labels --clear=manifest.annotations \
		--config.label="com.cyphar.test=1" --config.label="com.cyphar.empty="
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the correct data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Labels:'* ]]
	[[ "$output" == *'com.cyphar.empty: ""'* ]]
	[[ "$output" == *'com.cyphar.test: 1'* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config.Labels' <<<"$output"
	[[ "$output" == '{"com.cyphar.empty":"","com.cyphar.test":"1"}' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	sane_run jq -SMr '.annotations["com.cyphar.test"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "1" ]]

	sane_run jq -SMr '.annotations["com.cyphar.empty"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "$output" == "" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.exposedports" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--config.exposedports="2000" \
		--config.exposedports="8080/tcp" \
		--config.exposedports="1234/tcp"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the correct data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Exposed Ports: 1234/tcp, 2000, 8080/tcp'* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config.ExposedPorts' <<<"$output"
	[[ "$output" == '{"1234/tcp":{},"2000":{},"8080/tcp":{}}' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	sane_run jq -SMr '.annotations["org.opencontainers.image.exposedPorts"] | split(",")[]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" == "1234/tcp" ]]
	[[ "${lines[1]}" == "2000" ]]
	[[ "${lines[2]}" == "8080/tcp" ]]

	image-verify "${IMAGE}"
}

@test "umoci config --config.stopsignal" {
	# Modify none of the configuration.
	umoci config --image "${IMAGE}:${TAG}" --tag "${TAG}-new" \
		--config.stopsignal="SIGUSR1"
	[ "$status" -eq 0 ]
	image-verify "${IMAGE}"

	# Verify "umoci stat" shows the correct data.
	umoci stat --image "${IMAGE}:${TAG}-new"
	[ "$status" -eq 0 ]
	[[ "$output" == *'Stop Signal: SIGUSR1'* ]]
	umoci stat --image "${IMAGE}:${TAG}-new" --json
	[ "$status" -eq 0 ]
	sane_run jq -cSMr '.config.blob.config.StopSignal' <<<"$output"
	[[ "$output" == 'SIGUSR1' ]]

	# Unpack the image again.
	new_bundle_rootfs
	umoci unpack --image "${IMAGE}:${TAG}-new" "$BUNDLE"
	[ "$status" -eq 0 ]
	bundle-verify "$BUNDLE"

	sane_run jq -SMr '.annotations["org.opencontainers.image.stopSignal"]' "$BUNDLE/config.json"
	[ "$status" -eq 0 ]
	[[ "${output}" == "SIGUSR1" ]]

	image-verify "${IMAGE}"
}
