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

set -Eeuo pipefail

COVERAGE="$1"

function coverage() {
	go tool cover -func <(egrep -v 'vendor|third_party' "$@")
}

# Sort the coverage of all of the functions. This helps eye-ball what parts of
# the project don't have enough test coverage.
coverage "$COVERAGE" | egrep -v '^total:' | sort -k 3gr

# Now find the coverage percentage, to check against the hardcoded lower limit.
TOTAL="$(coverage "$COVERAGE" | grep '^total:' | sed -E 's|^[^[:digit:]]*([[:digit:]]+\.[[:digit:]]+)%$|\1|')"
CUTOFF="80.0"

echo "=== TOTAL COVERAGE: ${TOTAL}% ==="
if (( "$(echo "$TOTAL < $CUTOFF" | bc)" ))
then
	echo "ERROR: Test coverage has fallen below hard cutoff of $CUTOFF."
	echo "       Failing this CI run, as test coverage is a hard limit."
	exit 64
fi
