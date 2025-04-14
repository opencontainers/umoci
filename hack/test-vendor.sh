#!/bin/bash
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

set -Eeuxo pipefail
source "$(dirname "$BASH_SOURCE")/readlinkf.sh"

# Generate a hash-of-hashes for the entire vendor/ tree.
function gethash() {
	(
		cd "$1"
		find . -type f -not -path "./modules.txt" | \
			xargs sha256sum | sort -k2 | sha256sum | awk '{ print $1 }'
	)
}

# Figure out root directory.
ROOT="$(readlinkf_posix "$(dirname "$BASH_SOURCE")/..")"
STASHED_ROOT="$(mktemp -dt umoci-vendor.XXXXXX)"

# Stash away old vendor tree, and restore it on-exit.
mv "$ROOT/vendor" "$STASHED_ROOT/vendor"
trap 'rm -rf "$ROOT/vendor" ; mv "$STASHED_ROOT/vendor" "$ROOT/vendor" ; rm -rf "$STASHED_ROOT"' ERR EXIT

# Try to re-generate vendor/.
go clean -modcache
go mod verify
go mod vendor

# Make sure that none of the packages we have listed are unused.
if (go mod tidy -v 2>&1 | grep '^unused')
then
	echo "Unused modules found in go.mod."
	exit 1
fi

# See whether something has changed.
oldhash="$(gethash "$STASHED_ROOT/vendor")"
newhash="$(gethash "$ROOT/vendor")"

# Verify the hashes match.
diff -qr "$STASHED_ROOT/vendor" "$ROOT/vendor" || :
[[ "$oldhash" == "$newhash" ]] || ( echo "ERROR: vendor/ does not match go.mod -- run 'go mod vendor'" ; exit 1 )
