#!/bin/bash
# Copyright (C) 2013-2016 Docker, Inc.
# Copyright (C) 2016 SUSE LLC.
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

export PROJECT="github.com/cyphar/umoci"

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

	# Generate the import graph so we can delete things outside of it.
	echo -n "collecting import graph, "
	local IFS=$'\n'
	local imports=( $(
		for platform in "${platforms[@]}"; do
			export GOOS="${platform%/*}";
			export GOARCH="${platform##*/}";
			for tags in "${buildtagcombos[@]}"; do
				# Include dependencies (for packages and package tests).
				go list -e -tags "$tags" -f '{{join .Deps "\n"}}' "${packages[@]}"
				go list -e -tags "$tags" -f '{{join .TestImports "\n"}}' "${packages[@]}"
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
	local importantfiles=( -name 'LICENSE*' -or -name 'COPYING*' )

	# Delete all top-level files which are not LICENSE or COPYING related, as
	# well as deleting the actual directory if it's empty.
	for dir in "${prune[@]}"; do
		find "$dir" -maxdepth 1 -not -type d -not '(' "${importantfiles[@]}" ')' -exec rm -v -f '{}' ';'
		rmdir "$dir" 2>/dev/null || true
	done

	# Remove any extra files that we know we don't care about.
	echo -n "pruning unused files, "
	find vendor -type f -name '*_test.go' -exec rm -v '{}' ';'
	find vendor -regextype posix-extended -type f -not '(' -regex '.*\.(c|h|go)' -or '(' "${importantfiles[@]}" ')' ')' -exec rm -v '{}' ';'

	# Remove self from vendor.
	echo -n "pruning self from vendor, "
	rm -rf vendor/${PROJECT}

	echo "done"
}

# First we do a glide-up, with the repository's mirror.yaml being used.
# Hopefully the need for mirror.yaml will go away soon (I'm just waiting on the
# go-mtree changes to be merged at the moment).
rm -rf cache/
glide --home=. up -v
rm -rf cache/

# Clean up the vendor directory.
clean
