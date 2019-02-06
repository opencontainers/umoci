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

GO="${GO:-go}"
PROJECT="${PROJECT:-github.com/openSUSE/umoci}"

# Set up the root and coverage directories.
export ROOT="$(readlink -f "$(dirname "$(readlink -f "$BASH_SOURCE")")/..")"
export COVERAGE_DIR=$(mktemp --tmpdir -d umoci-coverage.XXXXXX)

# Run the tests and collate the results.
for pkg in $(go list $PROJECT/...); do
	$GO test -v -cover -covermode=count -coverprofile="$(mktemp --tmpdir=$COVERAGE_DIR cov.XXXXX)" -coverpkg=$PROJECT/... $pkg 2>/dev/null
done
[ "$COVERAGE" ] && $ROOT/hack/collate.awk $COVERAGE_DIR/* $COVERAGE | sponge $COVERAGE

# Clean up the coverage directory.
rm -rf "$COVERAGE_DIR"
