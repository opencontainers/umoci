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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestVerifyError(t *testing.T) {
	t.Run("NoError", func(t *testing.T) {
		testFn := func() (Err error) {
			defer VerifyError(&Err, func() error { return nil })
			return nil
		}
		assert.NoError(t, testFn(), "no error returned")
	})

	t.Run("SingleError", func(t *testing.T) {
		testErr := errors.New("TestVerifyError example error")
		testFn := func() (Err error) {
			defer VerifyError(&Err, func() error { return testErr })
			return nil
		}
		assert.ErrorIs(t, testFn(), testErr, "basic error should be returned")
	})

	t.Run("Multiple", func(t *testing.T) {
		wantErr := errors.New("wanted error")
		badErr := errors.New("unwanted error")

		testFn := func(finalErr error, errs ...error) (numErrCalled int, Err error) {
			for _, err := range errs {
				defer VerifyError(&Err, func() error {
					numErrCalled++
					return err
				})
			}
			return numErrCalled, finalErr
		}

		t.Run("DeferErr_OnlyLast", func(t *testing.T) {
			numErr, err := testFn(nil, nil, nil, wantErr)
			assert.Equal(t, 3, numErr, "each deferred error function should be called")
			assert.ErrorIs(t, err, wantErr, "last error should be kept")
		})

		t.Run("DeferErr_OnlyFirst", func(t *testing.T) {
			numErr, err := testFn(nil, wantErr, nil, nil)
			assert.Equal(t, 3, numErr, "each deferred error function should be called")
			assert.ErrorIs(t, err, wantErr, "first deferred error should be returned")
		})

		t.Run("MainErr_OnlyMain", func(t *testing.T) {
			numErr, err := testFn(wantErr, nil, nil, nil)
			assert.Equal(t, 3, numErr, "each deferred error function should be called")
			assert.ErrorIs(t, err, wantErr, "main error should be kept")
		})

		t.Run("DeferErr_Multiple", func(t *testing.T) {
			numErr, err := testFn(nil, badErr, badErr, wantErr, nil)
			assert.Equal(t, 4, numErr, "each deferred error function should be called")
			assert.ErrorIs(t, err, wantErr, "first deferred error should be returned")
		})

		t.Run("MainErr_Multiple", func(t *testing.T) {
			numErr, err := testFn(wantErr, badErr, badErr, badErr)
			assert.Equal(t, 3, numErr, "each deferred error function should be called")
			assert.ErrorIs(t, err, wantErr, "main error should be kept")
		})
	})
}
