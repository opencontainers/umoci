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

package assert_test

import (
	"errors"
	"testing"

	testassert "github.com/stretchr/testify/assert"

	"github.com/opencontainers/umoci/internal/assert"
)

func TestAssertTrue(t *testing.T) {
	for _, test := range []struct {
		name string
		val  any
	}{
		{"StringVal", "foobar"},
		{"IntVal", 123},
		{"ErrorVal", errors.New("error")},
		{"StructVal", struct{ a int }{1}},
		{"NilVal", nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			testassert.NotPanicsf(t, func() {
				assert.Assert(true, test.val)
			}, "assert(true) with value %v (%T)", test.val, test.val)
		})
	}

	t.Run("Assertf", func(t *testing.T) {
		fmtMsg := "foo %s %d"
		args := []any{"bar %x", 123}
		expected := "foo bar %x 123"

		testassert.NotPanicsf(t, func() {
			assert.Assertf(true, fmtMsg, args...)
		}, "assertf(true) with (%q, %v...) == %q", fmtMsg, args, expected)
	})

	t.Run("NoError", func(t *testing.T) {
		testassert.NotPanicsf(t, func() {
			assert.NoError(nil)
		}, "NoError(nil)")
	})
}

func TestAssertFalse(t *testing.T) {
	for _, test := range []struct {
		name string
		val  any
	}{
		{"StringVal", "foobar"},
		{"IntVal", 123},
		{"ErrorVal", errors.New("error")},
		{"StructVal", struct{ a int }{1}},
	} {
		t.Run(test.name, func(t *testing.T) {
			testassert.PanicsWithValuef(t, test.val, func() {
				assert.Assert(false, test.val)
			}, "assert(false) with value %v (%T)", test.val, test.val)
		})
	}

	t.Run("NilVal", func(t *testing.T) {
		// testify can detect nil-value panics, but the behaviour of nil panics
		// changed in Go 1.21 (and can be modified by GODEBUG=panicnil=1) so we
		// can't be sure what value we will get.
		testassert.Panics(t, func() {
			assert.Assert(false, nil)
		}, "assert(false) with nil")
	})

	t.Run("Assertf", func(t *testing.T) {
		fmtMsg := "foo %s %d"
		args := []any{"bar %x", 123}
		expected := "foo bar %x 123"

		testassert.PanicsWithValuef(t, expected, func() {
			assert.Assertf(false, fmtMsg, args...)
		}, "assertf(true) with (%q, %v...) == %q", fmtMsg, args, expected)
	})

	t.Run("NoError", func(t *testing.T) {
		err := errors.New("dummy error")
		testassert.PanicsWithValuef(t, err, func() {
			assert.NoError(err)
		}, "NoError(err=%q)", err)
	})
}
