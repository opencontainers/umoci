#!/bin/bash
# release.sh: configurable signed-artefact release script
# Copyright (C) 2016-2020 SUSE LLC
#
# This Source Code Form is subject to the terms of the Mozilla Public
# License, v2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

set -Eeuo pipefail
source "$(dirname "$BASH_SOURCE")/readlinkf.sh"

## --->
# Project-specific options and functions. In *theory* you shouldn't need to
# touch anything else in this script in order to use this elsewhere.
project="umoci"
root="$(readlinkf_posix "$(dirname "${BASH_SOURCE}")/..")"


# This function takes an output path as an argument, where the built
# (preferably static) binary should be placed.
function build_project() {
	builddir="$(dirname "$1")"
	shift
	tmprootfs="$(mktemp -dt "$project-build.XXXXXX")"

	for osarch in $@; do 
		IFS='/' read -ra OSARCH <<< "$osarch"
		local OS=${OSARCH[0]}
		local ARCH=${OSARCH[1]}
		log "building for $OS/$ARCH"

		set_cross_env $OS $ARCH
		make -C "$root" BUILD_DIR="$tmprootfs" COMMIT_NO=  "$project.static"
		mv "$tmprootfs/$project.static" "$builddir/${project}_${OS}_$ARCH"
		rm -rf "$tmprootfs"
	done
}
# End of the easy-to-configure portion.
## <---

# Print usage information.
function usage() {
	echo "usage: release_build.sh [-a <os/cross-arch>]... [-c <commit-ish>] [-H <hashcmd>]" >&2
	echo "                        [-r <release-dir>] [-v <version>]" >&2
	exit 1
}

# Log something to stderr.
function log() {
	echo "[*]" "$@" >&2
}

# Log something to stderr and then exit with 0.
function bail() {
	log "$@"
	exit 0
}


function set_cross_env() {
	GOOS="$1"
	GOARCH="$2"
	# it might be set from prev iterations 
	unset GOARM

	case $2 in
	armel)
		GOARCH=arm
		GOARM=6
		;;
	armhf)
		GOARCH=arm
		GOARM=7
		;;
	# since we already checked the archs we do not need to declare
	# a case for them
	esac
	export GOOS GOARCH GOARM
}

# used to validate arch and os strings
supported_oses=(linux darwin freebsd)
supported_archs=(amd64 arm64 armhf armel  386 s390x)

function assert_os_arch_support() {
	IFS='/' read -ra OSARCH <<< "$1"
	if [[ ! "${supported_oses[*]}" =~ "${OSARCH[0]}" ]]; then
		bail "unsupported os $1. expected one of: ${supported_oses[@]}"
	fi
	if [[ ! "${supported_archs[*]}" =~ "${OSARCH[1]}" ]]; then
		bail "unsupported arch $1. expected one of: ${supported_archs[@]}"
	fi
}



# When creating releases we need to build (ideally static) binaries, an archive
# of the current commit, and generate detached signatures for both.
keyid=""
version=""
commit="HEAD"
hashcmd="sha256sum"
# define os/arch targets to build for
declare -a osarch


while getopts "a:c:H:hr:v:" opt; do
	case "$opt" in
	a)
		assert_os_arch_support "$OPTARG"
		osarch+=($OPTARG)
		;;
	c)
		commit="$OPTARG"
		;;
	H)
		hashcmd="$OPTARG"
		;;
	h)
		usage
		;;
	r)
		releasedir="$OPTARG"
		;;
	v)
		version="$OPTARG"
		;;
	:)
		echo "Missing argument: -$OPTARG" >&2
		usage
		;;
	\?)
		echo "Invalid option: -$OPTARG" >&2
		usage
		;;
	esac
done


# Generate the defaults for version and so on *after* argument parsing and
# setup_project, to avoid calling get_version() needlessly.
version="${version:-$(<"$root/VERSION")}"
releasedir="${releasedir:-release/$version}"
hashcmd="${hashcmd:-sha256sum}"
# Suffixes of files to checksum/sign.
suffixes=(tar.xz ${supported_archs[@]})

log "creating $project release in '$releasedir'"
log "  version: $version"
log "   commit: $commit"
log "     hash: $hashcmd"

# Make explicit what we're doing.
#set -x

# Make the release directory.
rm -rf "$releasedir" && mkdir -p "$releasedir"

# Build project.
build_project "$releasedir/$project" "${osarch[@]}"

# Generate new archive.
git archive --format=tar --prefix="$project-$version/" "$commit" | xz > "$releasedir/$project.tar.xz"

# Generate sha256 checksums for both.
(
	cd "$releasedir"
	"$hashcmd" $(tree -fai | grep ${suffixes[@]/#/-e } | tr '\n' ' ') > "$project.$hashcmd"
)