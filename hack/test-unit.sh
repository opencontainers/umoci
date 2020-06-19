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

GO="${GO:-go}"
PROJECT="${PROJECT:-github.com/opencontainers/umoci}"

# Set up the root and coverage directories.
export ROOT="$(readlink -f "$(dirname "$(readlink -f "$BASH_SOURCE")")/..")"

# Run the tests.
extra_args=()
if [ -n "$COVERAGE" ]
then
	# If we have to generate a coverage file, make sure the coverage covers the
	# entire project and not just the package being tested.
	extra_args+=("-covermode=count" "-coverprofile=$COVERAGE" "-coverpkg=$PROJECT/...")
fi
"$GO" test -v -cover "${extra_args[@]}" "$PROJECT/..." 2>/dev/null
