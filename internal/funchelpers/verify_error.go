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

package funchelpers

import (
	"io"

	"github.com/opencontainers/umoci/internal/assert"
)

// VerifyError is a helper designed to make verifying deferred functions that
// return errors more ergonomic (most notably Close). This helper is intended
// to be used with named return values.
//
//	func foo() (Err error) {
//		f, err := os.Create("foobar")
//		if err != nil {
//			return err
//		}
//		defer funchelpers.VerifyError(&Err, foo.Close)
//		return nil
//	}
//
// which is equivalent to
//
//	func foo() (Err error) {
//		f, err := os.Create("foobar")
//		if err != nil {
//			return err
//		}
//		defer func() {
//			if err := f.Close(); err != nil && Err == nil {
//				Err = err
//			}
//		}
//		return nil
//	}
func VerifyError(Err *error, closeFn func() error) {
	assert.Assert(Err != nil,
		"VerifyError must be called with non-nil Err slot") // programmer error
	if err := closeFn(); err != nil && *Err == nil {
		*Err = err
	}
}

// VerifyClose is shorthand for `VerifyError(Err, closer.Close)`.
func VerifyClose(Err *error, closer io.Closer) {
	VerifyError(Err, closer.Close)
}
