#!/bin/bash
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2020 SUSE LLC
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

# Set up the coverage directory.
COVERAGE="${COVERAGE:-}"

# -coverprofile= truncates the target file, so we need to create a
# temporary file for each execution of the coverage-generating umoci
# binary, which will then be collated after all the tests are run.
export COVERAGE_DIR="$(mktemp -dt umoci-coverage.XXXXXX)"

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

if [ -n "${TRAVIS:-}" ]
then
	coverage_tags=integration
	[[ "$(id -u)" == 0 ]] || coverage_tags+=",rootless"

	# There are far too many coverage files if we just try to upload them all
	# (Codecov seems to struggle significiantly to process them). So we
	# pre-merge them -- but because of cmdline limits we conservatively place
	# them in a directory and use xargs.
	tmp_coverage="$(mktemp -t umoci-coverage-merged.XXXXXX)"
	find "$COVERAGE_DIR" -type f -print0 | xargs -0 "$ROOT/hack/collate.awk" >"$tmp_coverage"

	# Upload the merged coverage file.
	"$ROOT/hack/ci-codecov.sh" codecov -cZ -f "$tmp_coverage" -F "$coverage_tags"
elif [ -n "$COVERAGE" ]
then
	# If running locally, collate the coverage information.
	touch "$COVERAGE"
	"$ROOT/hack/collate.awk" "$COVERAGE_DIR/"* "$COVERAGE" | sponge "$COVERAGE"
fi
rm -rf "$COVERAGE_DIR"
