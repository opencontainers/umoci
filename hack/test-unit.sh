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

GO="${GO:-go}"
COVERAGE="${COVERAGE:-}"
PROJECT="${PROJECT:-github.com/opencontainers/umoci}"

extra_args=("$@")

# -coverprofile= truncates the target file, so we need to create a
# temporary file for this test run and collate it with the current
# $COVERAGE file.
COVERAGE_FILE="$(mktemp -t umoci-coverage.XXXXXX)"

# If we have to generate a coverage file, make sure the coverage covers the
# entire project and not just the package being tested. This mirrors
# ${TEST_BUILD_FLAGS} from the Makefile.
extra_args+=("-covermode=count" "-coverprofile=$COVERAGE_FILE" "-coverpkg=$PROJECT/...")

# Run the tests.
"$GO" test -v -cover "${extra_args[@]}" "$PROJECT/..." 2>/dev/null

if [ -n "${TRAVIS:-}" ]
then
	coverage_tags=unit
	[[ "$(id -u)" == 0 ]] || coverage_tags+=",rootless"

	# If we're running in Travis, upload the coverage files and don't bother
	# with the local coverage generation.
	"$ROOT/hack/ci-codecov.sh" codecov -cZ -f "$COVERAGE_FILE" -F "$coverage_tags"
elif [ -n "$COVERAGE" ]
then
	# If running locally, collate the coverage information.
	touch "$COVERAGE"
	"$ROOT/hack/collate.awk" "$COVERAGE_FILE" "$COVERAGE" | sponge "$COVERAGE"
fi
rm -f "$COVERAGE_FILE"
