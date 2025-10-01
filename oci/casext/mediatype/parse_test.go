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

package mediatype

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/opencontainers/umoci/internal"
)

// TODO: Add more parsing tests.

func TestParseEmptyJSON(t *testing.T) {
	for _, test := range []struct {
		name         string
		data         []byte
		expectedBlob any
		expectedErr  error
	}{
		{
			name:         "Good",
			data:         []byte("{}"),
			expectedBlob: struct{}{},
		},
		{
			name:        "Bad-Empty",
			data:        []byte{},
			expectedErr: internal.ErrInvalidEmptyJSON,
		},
		{
			name:        "Bad-Short",
			data:        []byte(`0`),
			expectedErr: internal.ErrInvalidEmptyJSON,
		},
		{
			name:        "Bad-Suffix",
			data:        []byte("{}\n"),
			expectedErr: internal.ErrInvalidEmptyJSON,
		},
		{
			name:        "Bad",
			data:        []byte("The quick brown fox jumps over the lazy dog.\n"),
			expectedErr: internal.ErrInvalidEmptyJSON,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			got, err := emptyJSONParser(bytes.NewBuffer(test.data))
			require.ErrorIsf(t, err, test.expectedErr, "emptyJSONParser(%q)", string(test.data))
			assert.Equalf(t, test.expectedBlob, got, "emptyJSONParser(%q)", string(test.data))
		})
	}
}
