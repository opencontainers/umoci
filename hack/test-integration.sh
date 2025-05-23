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

export ROOT="$(readlinkf_posix "$(dirname "$BASH_SOURCE")/..")"

export GOCOVERDIR="${GOCOVERDIR:-}"
[ -z "$GOCOVERDIR" ] || mkdir -p "$GOCOVERDIR"

# Create a temporary symlink for umoci, since the --help tests require the
# binary have the name "umoci". This is all just to make the Makefile and
# test/helpers.bash nicer.
UMOCI_DIR="$(mktemp -dt umoci.XXXXXX)"
export UMOCI="$UMOCI_DIR/umoci"
ln -s "$ROOT/umoci.cover" "$UMOCI"

# TODO: This really isn't that nice of an interface...
tests=()
if [[ -z "$TESTS" ]]
then
	tests=("$ROOT/test/"*.bats)
else
	for f in $TESTS; do
		tests+=("$ROOT/test/$f.bats")
	done
fi

# Run the tests.
bats --jobs "+8" --tap "${tests[@]}"
