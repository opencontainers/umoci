/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2024 SUSE LLC
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

package fmtcompat

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestErrorfSimpleString(t *testing.T) {
	err := Errorf("dummy error")

	require.Error(t, err)
	assert.ErrorContains(t, err, "dummy error")
}

func TestErrorfNonWrappedError(t *testing.T) {
	var dummyError error

	err := Errorf("dummy error %v", dummyError)

	require.Error(t, err)
	assert.ErrorContains(t, err, "dummy error")
	assert.ErrorContains(t, err, "<nil>")
}

func TestErrorfWrappedError(t *testing.T) {
	baseError := errors.New("inner error")

	err := Errorf("outer error %w", baseError)

	require.Error(t, err)
	assert.ErrorContains(t, err, "outer error")
	assert.ErrorContains(t, err, "inner error")
	assert.ErrorIs(t, err, baseError)
}

func TestErrorfWrappedNilError(t *testing.T) {
	var baseError error

	err := Errorf("outer error %w", baseError)

	assert.NoError(t, err)
}

func TestErrorfMultipleWrappedErrors(t *testing.T) {
	baseError1 := errors.New("inner1 error")
	baseError2 := errors.New("inner2 error")

	err := Errorf("outer error %w %w", baseError1, baseError2)

	require.Error(t, err)
	assert.ErrorContains(t, err, "outer error")
	assert.ErrorContains(t, err, "inner1 error")
	assert.ErrorIs(t, err, baseError1)
	assert.ErrorContains(t, err, "inner2 error")
	assert.ErrorIs(t, err, baseError2)
}

func TestErrorfMultipleWrappedNilErrors(t *testing.T) {
	var baseError1, baseError2 error

	err := Errorf("outer error %w %w", baseError1, baseError2)

	assert.NoError(t, err)
}

func TestErrorfMultipleWrappedMixedErrors(t *testing.T) {
	var baseError1 error
	baseError2 := errors.New("inner2 error")

	err := Errorf("outer error %w %w", baseError1, baseError2)

	require.Error(t, err)
	assert.ErrorContains(t, err, "outer error")
	assert.ErrorContains(t, err, "<nil>")
	assert.ErrorContains(t, err, "inner2 error")
	assert.ErrorIs(t, err, baseError2)
}

func TestErrorfEscapes(t *testing.T) {
	dummyErr := errors.New("wrapped error")

	err := Errorf("dummy error %%%v %w", 123, dummyErr)

	require.Error(t, err)
	assert.ErrorContains(t, err, "dummy error")
	assert.ErrorContains(t, err, "wrapped error")
	assert.ErrorIs(t, err, dummyErr)
}

func TestErrorfEscapesNilError(t *testing.T) {
	dummyErr := errors.New("unwrapped error")

	err := Errorf("dummy error %%%v %w", dummyErr, nil)

	require.NoError(t, err)
}
