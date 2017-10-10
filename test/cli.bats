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

@test "umoci autocompletion" {
	run cat autocomplete/umoci_commands.txt
	[ "$status" -eq 0 ]
	[ "${lines[0]}" = "config" ]
	[ "${lines[1]}" = "unpack" ]
	[ "${lines[2]}" = "repack" ]
	[ "${lines[3]}" = "gc" ]
	[ "${lines[4]}" = "init" ]
	[ "${lines[5]}" = "new" ]
	[ "${lines[6]}" = "tag" ]
	[ "${lines[7]}" = "remove" ]
	[ "${lines[8]}" = "rm" ]
	[ "${lines[9]}" = "list" ]
	[ "${lines[10]}" = "ls" ]
	[ "${lines[11]}" = "stat" ]
	[ "${lines[12]}" = "raw" ]
	[ "${lines[13]}" = "help" ]
	[ "${lines[14]}" = "h" ]
}
