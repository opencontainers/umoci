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


# Change to site root.
site_root="$(readlink -f "$(dirname "${BASH_SOURCE}")/../.site")"
cd "$site_root"

# Copy key files from the source directory to the right place.
# These are ignored by git.
cp ../CHANGELOG.md content/changelog.md
cp ../CONTRIBUTING.md content/contributing.md

# Check out the 'gh-pages' worktree.
rm -rf public/ && git worktree prune
git fetch -f https://github.com/openSUSE/umoci.git gh-pages:gh-pages
git worktree add -B gh-pages 'public' gh-pages

# Build the source.
hugo

# Commit the changes.
(
	cd public/ ;
	git add --all ;
	git commit -sm "update gh-pages $(date --utc --iso-8601=s)" ;
)
