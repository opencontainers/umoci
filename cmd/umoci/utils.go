// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

// Package main contains utility functions for umoci commands.
package main

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// parseSourceDateEpoch parses the SOURCE_DATE_EPOCH environment variable
// and returns the corresponding time.Time, or zero time if not set.
func parseSourceDateEpoch() (time.Time, error) {
	sourceDateEpochStr := os.Getenv("SOURCE_DATE_EPOCH")
	if sourceDateEpochStr == "" {
		return time.Time{}, nil
	}

	timestamp, err := strconv.ParseInt(sourceDateEpochStr, 10, 64)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing SOURCE_DATE_EPOCH %q: %w", sourceDateEpochStr, err)
	}

	return time.Unix(timestamp, 0).UTC(), nil
}
