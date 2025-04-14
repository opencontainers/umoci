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

ATTEMPT=0
MAX_ATTEMPTS=10
BACKOFF=0.2

until ( timeout 5s curl "$@" )
do
	[[ "$((++ATTEMPT))" -lt "$MAX_ATTEMPTS" ]] || exit 1
	sleep "$(bc <<<"$BACKOFF * 2*$ATTEMPT")"
done
