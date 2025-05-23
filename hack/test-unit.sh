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

GO="${GO:-go}"
PROJECT="${PROJECT:-github.com/opencontainers/umoci}"

extra_args=("$@")

GOCOVERDIR="${GOCOVERDIR:-}"
[ -z "$GOCOVERDIR" ] || mkdir -p "$GOCOVERDIR"

# NOTE: As part of the <https://github.com/golang/go/issues/73842> workaround,
# GOCOVERDIR needs to be an absolute path.
[[ "$GOCOVERDIR" =~ ^/ ]] || GOCOVERDIR="$PWD/$GOCOVERDIR"

# If we have to generate a coverage file, make sure the coverage covers the
# entire project and not just the package being tested. This mirrors
# ${TEST_BUILD_FLAGS} from the Makefile.
extra_args+=("-covermode=count" "-coverpkg=$PROJECT/...")
if [ -n "$GOCOVERDIR" ]
then
	extra_args+=("-test.gocoverdir=$GOCOVERDIR")
fi

# Run the tests.
#
# NOTE: To work around <https://github.com/golang/go/issues/73842>, we need to
# run "go test" inside each subpackage directory to actually get the tests to
# run properly.
pkgdirs="$("$GO" list -f "{{ .Dir }}" "$ROOT/...")"
while IFS= read -r pkgdir
do
	pushd "$pkgdir"
	"${GO}" test -v -cover "${extra_args[@]}" .
	popd
done <<<"$pkgdirs"
