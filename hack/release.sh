#!/bin/bash
# Copyright (C) 2017 SUSE LLC.
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

set -e

root="$(readlink -f "$(dirname "${BASH_SOURCE}")/..")"

function usage() {
	echo "usage: release.sh [-S <gpg-key-id>] [-c <commit-ish>] [-r <release-dir>] [-v <version>]" >&2
	exit 1
}

function log() {
	echo "[*] $*" >&2
}

# When creating releases we need to build static binaries, an archive of the
# current commit, and generate detached signatures for both.
keyid=""
commit="HEAD"
version=""
releasedir=""
while getopts "S:c:r:v:" opt; do
	case "$opt" in
		S)
			keyid="$OPTARG"
			;;
		c)
			commit="$OPTARG"
			;;
		r)
			releasedir="$OPTARG"
			;;
		v)
			version="$OPTARG"
			;;
		\:)
			echo "Missing argument: -$OPTARG" >&2
			usage
			;;
		\?)
			echo "Invalid option: -$OPTARG" >&2
			usage
			;;
	esac
done

[[ "$version" ]] || set version="$(git describe HEAD)"
[[ "$releasedir" ]] || set releasedir="release-$version"

log "creating umoci release in '$releasedir'"
log "  version: $version"
log "   commit: $commit"
log "      key: ${keyid:-DEFAULT}"

# Make explicit what we're doing.
set -x

# Make the release directory.
rm -rf "$releasedir" && mkdir -p "$releasedir"

# Build umoci.
make -C "$root" BUILD_DIR="$releasedir" COMMIT_NO= umoci.static
mv "$releasedir"/umoci.{static,amd64}

# Generate new archive.
git archive --format=tar --prefix="umoci-$version/" "$commit" | xz > "$releasedir/umoci.tar.xz"

# Sign everything.
[[ "$keyid" ]] && set gpgflags="--default-key '$keyid'"
gpg $gpgflags --detach-sign --armor "$releasedir/umoci.amd64"
gpg $gpgflags --detach-sign --armor "$releasedir/umoci.tar.xz"
