#!/bin/bash
# umoci: Umoci Modifies Open Containers' Images
# Copyright (C) 2016-2021 SUSE LLC
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

CODECOV_DIR="$(mktemp -dt umoci-codecov.XXXXXX)"
#trap 'rm -rf $CODECOV_DIR' EXIT

export ROOT="$(readlinkf_posix "$(dirname "$BASH_SOURCE")/..")"

# Download the codecov-bash uploader from GitHub and check the SHA512.
CODECOV_VERSION="1.0.6"
CODECOV_REPOURL="https://raw.githubusercontent.com/codecov/codecov-bash/$CODECOV_VERSION"

echo "WARNING: Downloading and executing codecov-bash, which will upload data." >&2
sleep 1s

cmd="$1"
shift

"$ROOT/hack/resilient-curl.sh" -sSL "$CODECOV_REPOURL/$cmd" -o "$CODECOV_DIR/$cmd"
"$ROOT/hack/resilient-curl.sh" -sSL "$CODECOV_REPOURL/SHA512SUM" -o "$CODECOV_DIR/SHA512SUM"

pushd "$CODECOV_DIR" >/dev/null
sha512sum -c --ignore-missing --quiet ./SHA512SUM || exit 1
chmod +x "$cmd"
popd >/dev/null

exec "$CODECOV_DIR/$cmd" "$@"
