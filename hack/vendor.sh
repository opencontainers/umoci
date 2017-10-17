#!/bin/bash
# Copyright (C) 2013-2016 Docker, Inc.
# Copyright (C) 2016, 2017 SUSE LLC.
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

export PROJECT="github.com/openSUSE/umoci"

# Clean cleans the vendor directory of any directories which are not required
# by imports in the current project. It is adapted from hack/.vendor-helpers.sh
# in Docker (which is also Apache-2.0 licensed).
clean() {
	local packages=(
		"${PROJECT}/cmd/umoci" # main umoci package
	)

	# Important because different buildtags and platforms can have different
	# import sets. It's very important to make sure this is kept up to date.
	local platforms=( "linux/amd64" )
	local buildtagcombos=( "" )

	# Remove non-top-level vendors.
	echo -n "remove library vendors,"
	find vendor/* -type d '(' -name 'vendor' -or -name 'Godeps' ')' -print0 | xargs -r0 -- rm -rfv

	# Generate the import graph so we can delete things outside of it.
	echo -n "collecting import graph, "
	local IFS=$'\n'
	local imports=( $(
		for platform in "${platforms[@]}"; do
			export GOOS="${platform%/*}";
			export GOARCH="${platform##*/}";
			for tags in "${buildtagcombos[@]}"; do
				# Include dependencies (for packages).
				go list -e -tags "$tags" -f '{{join .Deps "\n"}}' "${packages[@]}"
				# .TestImports is not recursive, so we need to do it manually.
				for dep in $(go list -e -tags "$tags" -f '{{join .Deps "\n"}}' "${packages[@]}"); do
					go list -e -tags "$tags" -f '{{join .TestImports "\n"}}' "$dep"
				done
			done
		done | grep -vP "^${PROJECT}/(?!vendor)" | sort -u
	) )
	# Remove non-standard imports from the set of dependencies detected.
	imports=( $(go list -e -f '{{if not .Standard}}{{.ImportPath}}{{end}}' "${imports[@]}") )
	unset IFS

	# We use find to prune any directory that was not included above. So we
	# have to generate the (-path A -or -path B ...) commandline.
	echo -n "pruning unused packages, "
	local findargs=( )

	# Add vendored imports that are used to the list. Note that packages in
	# vendor/ act weirdly (they are prefixed by ${PROJECT}/vendor).
	for import in "${imports[@]}"; do
		[ "${#findargs[@]}" -eq 0 ] || findargs+=( -or )
		findargs+=( -path "vendor/$(echo "$import" | sed "s:^${PROJECT}/vendor/::")" )
	done

	# Find all of the vendored directories that are not in the set of used imports.
	local IFS=$'\n'
	local prune=( $(find vendor -depth -type d -not '(' "${findargs[@]}" ')') )
	unset IFS

	# Files we don't want to delete from *any* directory.
	local importantfiles=( -name 'LICENSE*' -or -name 'COPYING*' -or -name 'NOTICE*' )

	# Delete all top-level files which are not LICENSE or COPYING related, as
	# well as deleting the actual directory if it's empty.
	for dir in "${prune[@]}"; do
		find "$dir" -maxdepth 1 -not -type d -not '(' "${importantfiles[@]}" ')' -exec rm -vf '{}' ';'
		rmdir "$dir" 2>/dev/null || true
	done

	# Remove any extra files that we know we don't care about.
	echo -n "pruning unused files, "
	find vendor -type f -name '*_test.go' -exec rm -vf '{}' ';'
	find vendor -regextype posix-extended -type f -not '(' -regex '.*\.(s|c|h|go)' -or '(' "${importantfiles[@]}" ')' ')' -exec rm -vf '{}' ';'

	# Remove self from vendor.
	echo -n "pruning self from vendor, "
	rm -rf vendor/${PROJECT}

	echo "done"
}

clone() {
	local importPath="$1"
	local commit="$2"
	local cloneURL="${3:-https://$1.git}"
	local vendorPath="vendor/$importPath"

	if [ -z "$3" ]; then
		echo "clone $importPath @ $commit"
	else
		echo "clone $cloneURL -> $importPath @ $commit"
	fi

	set -e
	(
		rm -rf "$vendorPath" && mkdir -p "$vendorPath"
		git clone "$cloneURL" "$vendorPath"
		cd "$vendorPath"
		git checkout --detach "$commit"
	) &>/dev/null
}

# Update everything.
# TODO: Put this in a vendor.conf file or something like that (to be compatible
#       with LK4D4/vndr). This setup is a bit unwieldy.
clone github.com/opencontainers/go-digest v1.0.0-rc1
clone github.com/opencontainers/image-spec v1.0.0
clone github.com/opencontainers/runtime-spec v1.0.0
clone github.com/opencontainers/runtime-tools 2d270b8764c02228eeb13e36f076f5ce6f2e3591
clone github.com/syndtr/gocapability 2c00daeb6c3b45114c80ac44119e7b8801fdd852
clone golang.org/x/crypto b8a2a83acfe6e6770b75de42d5ff4c67596675c0 https://github.com/golang/crypto
clone golang.org/x/sys 686000749eaec0b8855b8eef5336cf63899fe51d https://github.com/golang/sys
clone github.com/docker/go-units v0.3.1
clone github.com/pkg/errors v0.8.0
clone github.com/apex/log afb2e76037a5f36542c77e88ef8aef9f469b09f8
clone github.com/urfave/cli v1.20.0
clone github.com/cyphar/filepath-securejoin v0.2.1
clone github.com/vbatts/go-mtree v0.4.1
clone github.com/Sirupsen/logrus v1.0.3
clone golang.org/x/net 45e771701b814666a7eb299e6c7a57d0b1799e91 https://github.com/golang/net
# Used purely for testing.
clone github.com/mohae/deepcopy 491d3605edfb866af34a48075bd4355ac1bf46ca

# Apply patches.
self="$(readlink -f "$(dirname "${BASH_SOURCE}")")"
$self/patch.sh

# Clean up the vendor directory.
clean
