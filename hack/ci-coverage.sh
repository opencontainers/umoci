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

set -Eeuo pipefail

function info() {
	echo "[i]" "$@" >&2
}

function bail() {
	echo "[!]" "$@" >&2
	exit 1
}

function join() {
  local delim="${1:-}" ret="${2:-}"
  if [ "$#" -ge 2 ] && shift 2
  then
    printf "%s" "$ret" "${@/#/$delim}"
  fi
  echo
}

GETOPT="$(getopt -o "m:t:F" --long "merge:,textfmt:,func" -- "$@")"
eval set -- "$GETOPT"

merge=
textfmt=
func=
while true
do
	opt="$1"
	shift
	case "$opt" in
		-m|--merge)
			merge="$1"
			shift
			;;
		-t|--textfmt)
			textfmt="$1"
			shift
			;;
		-F|--func)
			func=1
			;;
		--)
			break
			;;
		*)
			bail "unknown flag $opt"
			;;
	esac
done

# We need to comma-separate the passed GOCOVERDIRs for go tool covdata.
gocoverdir="$(join , "$@")"

if [ -n "$merge" ]
then
	info "merging coverage data into $merge gocoverdir"
	mkdir -p "$merge"
	go tool covdata merge -i="$gocoverdir" -o="$merge"
fi

if [ -n "$textfmt" ]
then
	info "converting coverage data into textfmt $textfmt"
	go tool covdata textfmt -i="$gocoverdir" -o="$textfmt"
fi

if [ -n "$func" ]
then
	coverage_data="$(go tool covdata func -i="$gocoverdir")"

	# Sort the coverage of all of the functions. This helps eye-ball what parts of
	# the project don't have enough test coverage.
	grep -Ev '^total\s' <<<"$coverage_data" | sort -k 3gr

	# Now find the coverage percentage, to check against the hardcoded lower limit.
	total="$(grep '^total\s' <<<"$coverage_data" | sed -E 's|^[^[:digit:]]*([[:digit:]]+\.[[:digit:]]+)%$|\1|')"
	CUTOFF="80.0"

	echo "=== TOTAL COVERAGE: ${total}% ==="
	if (( "$(echo "$total < $CUTOFF" | bc)" ))
	then
		echo "ERROR: Test coverage has fallen below hard cutoff of $CUTOFF."
		echo "       Failing this CI run, as test coverage is a hard limit."
		exit 64
	fi
fi
