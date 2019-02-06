#!/bin/bash
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2019 SUSE LLC.
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

export COVER="${COVER:-0}"

# Set up the root and coverage directories.
export ROOT="$(readlink -f "$(dirname "$(readlink -f "$BASH_SOURCE")")/..")"
if [ "$COVER" -eq 1 ]; then
	export COVERAGE_DIR=$(mktemp --tmpdir -d umoci-coverage.XXXXXX)
fi

if [ "$COVER" -eq 1 ]; then
	# Create a temporary symlink for umoci, since the --help tests require the
	# binary have the name "umoci". This is all just to make the Makefile nicer.
	UMOCI_DIR="$(mktemp --tmpdir -d umoci.XXXXXX)"
	export UMOCI="$UMOCI_DIR/umoci"
	ln -s "$ROOT/umoci.cover" "$UMOCI"
fi

# Run the tests and collate the results.
tests=()
if [[ -z "$TESTS" ]]; then
	tests=("$ROOT/test/"*.bats)
else
	for f in $TESTS; do
		tests+=("$ROOT/test/$f.bats")
	done
fi
bats --jobs "+1" --tap "${tests[@]}"
if [ "$COVER" -eq 1 ]; then
	[ "$COVERAGE" ] && "$ROOT/hack/collate.awk" "$COVERAGE_DIR/"* "$COVERAGE" | sponge "$COVERAGE"
fi

# Clean up the coverage directory.
rm -rf "$COVERAGE_DIR"
