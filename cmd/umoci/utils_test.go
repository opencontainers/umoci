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

package main

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSourceDateEpoch(t *testing.T) {
	t.Run("EmptyEnvironmentVariable", func(t *testing.T) {
		require.NoError(t, os.Unsetenv("SOURCE_DATE_EPOCH"))

		result, err := parseSourceDateEpoch()

		require.NoError(t, err)
		assert.Nil(t, result)
	})

	t.Run("ValidTimestamp", func(t *testing.T) {
		t.Setenv("SOURCE_DATE_EPOCH", "1234567890")

		result, err := parseSourceDateEpoch()

		require.NoError(t, err)
		assert.False(t, result.IsZero())
		assert.Equal(t, int64(1234567890), result.Unix())
		assert.Equal(t, time.UTC, result.Location())
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		t.Setenv("SOURCE_DATE_EPOCH", "not-a-number")

		result, err := parseSourceDateEpoch()

		require.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "parse SOURCE_DATE_EPOCH=")
	})
}
