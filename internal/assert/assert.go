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

// This package originally came from
// "github.com/cyphar/filepath-securejoin/pathrs-lite/internal/assert" and was
// relicensed under Apache-2.0 by the copyright holder (Aleksa Sarai / SUSE).

// Package assert provides some basic assertion helpers for Go.
package assert

import (
	"fmt"
)

// Assert panics if the predicate is false with the provided argument.
func Assert(predicate bool, msg any) {
	if !predicate {
		panic(msg)
	}
}

// NoError panics if the error is non-nil and the message is the error itself.
// This is just shorthand for "Assert(err == nil, err)".
func NoError(err error) {
	Assert(err == nil, err)
}

// Assertf panics if the predicate is false and formats the message using the
// same formatting as [fmt.Printf].
//
// [fmt.Printf]: https://pkg.go.dev/fmt#Printf
func Assertf(predicate bool, fmtMsg string, args ...any) {
	Assert(predicate, fmt.Sprintf(fmtMsg, args...))
}
