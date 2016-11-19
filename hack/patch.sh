#!/bin/bash
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

set -e

SCRIPTDIR="$(dirname $0)"
VENDOR="$SCRIPTDIR/../vendor"

# patch patches the given vendored project ($1) with the given patch file ($2).
patch() {
	local project="$1"
	local patch="$2"

	echo "[$project] patching with $patch" >&2
	command patch -d "$VENDOR/$project" -p1 <"$SCRIPTDIR/$patch"
}

patch github.com/vbatts/go-mtree gomtree-0001-entries-prepend-all-hierarchies-with-keyword-metadat.patch
