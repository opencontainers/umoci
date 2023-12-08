#!/bin/bash
# release.sh: configurable signed-artefact release script
# Copyright (C) 2016-2020 SUSE LLC
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

set -Eeuo pipefail
# shellcheck source=./readlinkf.sh
source "$(dirname "${BASH_SOURCE[0]}")/readlinkf.sh"

## --->
# Project-specific options and functions. In *theory* you shouldn't need to
# touch anything else in this script in order to use this elsewhere.
project="umoci"
root="$(readlinkf_posix "$(dirname "${BASH_SOURCE[0]}")/..")"

# These functions allow you to configure how the defaults are computed.
function get_os()      { go env GOOS ; }
function get_arch()    { go env GOARCH ; }
function get_version() { cat "$root/VERSION" ; }

# Any pre-configuration steps should be done here -- for instance ./configure.
function setup_project() { true ; }

# This function takes an output path as an argument, where the built
# (preferably static) binary should be placed.
function build_project() {
	tmprootfs="$(mktemp -dt "$project-build.XXXXXX")"

	make -C "$root" GOOS="$GOOS" GOARCH="$GOARCH" BUILD_DIR="$tmprootfs" COMMIT_NO= "$project.static"
	mv "$tmprootfs/$project.static" "$1"
	rm -rf "$tmprootfs"
}
# End of the easy-to-configure portion.
## <---

# Print usage information.
function usage() {
	echo "usage: release.sh [-h] [-v <version>] [-c <commit>] [-o <output-dir>]" >&2
	echo "                       [-H <hashcmd>] [-S <gpg-key>]" >&2
}

# Log something to stderr.
function log() {
	echo "[*]" "$@" >&2
}

# Log something to stderr and then exit with 0.
function quit() {
	log "$@"
	exit 0
}

# Conduct a sanity-check to make sure that GPG provided with the given
# arguments can sign something. Inability to sign things is not a fatal error.
function gpg_cansign() {
	gpg "$@" --clear-sign </dev/null >/dev/null
}

# When creating releases we need to build (ideally static) binaries, an archive
# of the current commit, and generate detached signatures for both.
keyid=""
version=""
targets=("$(get_os)/$(get_arch)")
commit="HEAD"
hashcmd="sha256sum"
while getopts ":a:c:H:h:o:S:t:v:" opt; do
	case "$opt" in
		a)
			targets+=("$(get_os)/$OPTARG")
			;;
		c)
			commit="$OPTARG"
			;;
		H)
			hashcmd="$OPTARG"
			;;
		h)
			usage ; exit 0
			;;
		o)
			outputdir="$OPTARG"
			;;
		S)
			keyid="$OPTARG"
			;;
		t)
			targets+=("$OPTARG")
			;;
		v)
			version="$OPTARG"
			;;
		:)
			echo "Missing argument: -$OPTARG" >&2
			usage ; exit 1
			;;
		\?)
			echo "Invalid option: -$OPTARG" >&2
			usage ; exit 1
			;;
	esac
done

# Run project setup first...
( set -x ; setup_project )

# Generate the defaults for version and so on *after* argument parsing and
# setup_project, to avoid calling get_version() needlessly.
version="${version:-$(get_version)}"
outputdir="${outputdir:-release/$version}"

log "[[ $project ]]"
log "targets: ${targets[*]}"
log "version: $version"
log "commit: $commit"
log "output_dir: $outputdir"
log "key: ${keyid:-(default)}"
log "hash_cmd: $hashcmd"

# Make explicit what we're doing.
set -x

# Make the release directory.
rm -rf "$outputdir" && mkdir -p "$outputdir"

# Build project.
for target in "${targets[@]}"; do
	target="${target//\//.}"
	os="$(cut -d. -f1 <<<"$target")"
	arch="$(cut -d. -f2 <<<"$target")"
	GOOS="$os" GOARCH="$arch" build_project "$outputdir/$project.$target"
done

# Generate new archive.
git archive --format=tar --prefix="$project-$version/" "$commit" | xz > "$outputdir/$project.tar.xz"

# Generate sha256 checksums for both.
( cd "$outputdir" ; "$hashcmd" "$project".* > "$project.$hashcmd" ; )

# Set up the gpgflags.
gpgflags=()
[[ -z "$keyid" ]] || gpgflags+=("--default-key=$keyid")
gpg_cansign "${gpgflags[@]}" || quit "Could not find suitable GPG key, skipping signing step."

# Sign everything.
for target in "${targets[@]}"; do
	target="${target//\//.}"
	gpg "${gpgflags[@]}" --detach-sign --armor "$outputdir/$project.$target"
done
gpg "${gpgflags[@]}" --detach-sign --armor "$outputdir/$project.tar.xz"
gpg "${gpgflags[@]}" --clear-sign --armor \
	--output "$outputdir/$project.$hashcmd"{.tmp,} && \
	mv "$outputdir/$project.$hashcmd"{.tmp,}
