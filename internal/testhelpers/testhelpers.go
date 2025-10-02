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

// Package testhelpers provides helpers for umoci's test suite. None of the
// code in this package is enabled when building a non-test umoci binary.
package testhelpers

import "github.com/opencontainers/umoci/internal/assert"

// binaryType is set during umoci.cover building.
var binaryType = releaseBinary

// IsTestBinary returns whether the current binary is a test binary. This is
// only ever meant to be used so that test-specific initialisations can be done
// inside packages. Don't use it for anything else.
func IsTestBinary() bool {
	return binaryType == testBinary
}

const (
	testBinary    = "test"
	releaseBinary = "release"
)

// Sanity check.
func init() {
	assert.Assert(
		binaryType == releaseBinary || binaryType == testBinary,
		"BinaryType is not release or test.")
}
