#!/usr/bin/env bats -t
# umoci: Umoci modifies Open Containers' Images
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

@test "umoci --version" {
	umoci --version
	[ "$status" -eq 0 ]
	[[ "$output" =~ "umoci version "+ ]]

	umoci -v
	[ "$status" -eq 0 ]
	[[ "$output" =~ "umoci version "+ ]]
}

@test "umoci --help" {
	umoci help
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ "NAME:"+ ]]
	[[ "${lines[1]}" =~ "umoci - umoci modifies Open Container images"+ ]]

	umoci h
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ "NAME:"+ ]]
	[[ "${lines[1]}" =~ "umoci - umoci modifies Open Container images"+ ]]

	umoci --help
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ "NAME:"+ ]]
	[[ "${lines[1]}" =~ "umoci - umoci modifies Open Container images"+ ]]

	umoci -h
	[ "$status" -eq 0 ]
	[[ "${lines[0]}" =~ "NAME:"+ ]]
	[[ "${lines[1]}" =~ "umoci - umoci modifies Open Container images"+ ]]
}

@test "umoci command --help" {

	umoci config --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci config"+ ]]

	umoci config -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci config"+ ]]

	umoci unpack --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci unpack"+ ]]

	umoci unpack -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci unpack"+ ]]

	umoci repack --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci repack"+ ]]

	umoci repack -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci repack"+ ]]

	umoci new --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci new"+ ]]

	umoci new -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci new"+ ]]

	umoci tag --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci tag"+ ]]

	umoci tag -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci tag"+ ]]

	umoci raw --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw"+ ]]

	umoci raw -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw"+ ]]

	umoci raw runtime-config --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw runtime-config"+ ]]

	umoci raw runtime-config -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw runtime-config"+ ]]

	umoci raw config --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw runtime-config"+ ]]

	umoci raw config -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci raw runtime-config"+ ]]

	umoci remove --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci remove"+ ]]

	umoci remove -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci remove"+ ]]

	umoci rm --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci remove"+ ]]

	umoci rm -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci remove"+ ]]

	umoci stat --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci stat"+ ]]

	umoci stat -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci stat"+ ]]

	umoci gc --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci gc"+ ]]

	umoci gc -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci gc"+ ]]

	umoci init --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci init"+ ]]

	umoci init -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci init"+ ]]

	umoci list --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci list"+ ]]

	umoci list -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci list"+ ]]

	umoci ls --help
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci list"+ ]]

	umoci ls -h
	[ "$status" -eq 0 ]
	[[ "${lines[1]}" =~ "umoci list"+ ]]
}
