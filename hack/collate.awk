#!/usr/bin/awk -f
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

# collate.awk allows you to collate a bunch of Go coverprofiles for a given
# binary (generated with -test.coverprofile), so that the statistics actually
# make sense. The input to this function is just the concatenated versions of
# the coverage reports, and the output is the combined coverage report.
#
# NOTE: This will _only_ work on coverage binaries compiles with
# -covermode=count. The other modes aren't supported.

{
	# Every coverage file in the set will start with a "mode:" header. Just make
	# sure they're all set to "count".
	if ($1 == "mode:") {
		if ($0 != "mode: count") {
			print "Invalid coverage mode", $2 > "/dev/stderr"
			exit 1
		}
		next
	}

	# The format of all other lines is as follows.
	#   <file>:<startline>.<startcol>,<endline>.<endcol> <numstmt> <count>
	# We only care about the first field and the count.
	statements[$1] = $2
	counts[$1] += $3
}

END {
	print "mode: count"
	for (block in statements) {
		print block, statements[block], counts[block]
	}
}
