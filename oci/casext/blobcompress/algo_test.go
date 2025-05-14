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

package blobcompress

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testAlgo(t *testing.T, algo Algorithm, expectedName string, expectedDiff bool) {
	const data = "meshuggah rocks!!!"

	plainBuf := bytes.NewBufferString(data)

	r, err := algo.Compress(plainBuf)
	require.NoErrorf(t, err, "compress with %T (%q)", algo, expectedName)
	assert.Equalf(t, expectedName, algo.MediaTypeSuffix(), "algo %T.MediaTypeSuffix", algo)

	compressed, err := io.ReadAll(r)
	require.NoError(t, err, "read compressed data")
	if expectedDiff {
		assert.NotEqualf(t, data, string(compressed), "compressed data with %T", algo)
	} else {
		assert.Equalf(t, data, string(compressed), "compressed data with %T", algo)
	}

	compressedBuf := bytes.NewBuffer(compressed)

	r, err = algo.Decompress(compressedBuf)
	require.NoErrorf(t, err, "decompress with %T (%q)", algo, expectedName)

	content, err := io.ReadAll(r)
	require.NoErrorf(t, err, "read decompressed data")

	assert.Equal(t, data, string(content), "%T (%q) round-tripped data", algo, expectedName)
}

type fakeAlgo struct{ name string }

var _ Algorithm = fakeAlgo{"foo"}

func (f fakeAlgo) MediaTypeSuffix() string                     { return f.name }
func (f fakeAlgo) Compress(io.Reader) (io.ReadCloser, error)   { return nil, fmt.Errorf("err") }
func (f fakeAlgo) Decompress(io.Reader) (io.ReadCloser, error) { return nil, fmt.Errorf("err") }

func TestRegister(t *testing.T) {
	var fakeAlgo Algorithm = fakeAlgo{"fake-algo1"}
	fakeAlgoName := fakeAlgo.MediaTypeSuffix()

	err := RegisterAlgorithm(fakeAlgo)
	require.NoError(t, err, "register new algorithm")

	gotAlgo := GetAlgorithm(fakeAlgoName)
	assert.NotNil(t, gotAlgo, "get registered algorithm")
	assert.Equal(t, gotAlgo, fakeAlgo, "get registered algorithm")
}

func TestRegisterFail(t *testing.T) {
	var fakeAlgo Algorithm = fakeAlgo{"fake-algo2"}
	fakeAlgoName := fakeAlgo.MediaTypeSuffix()

	err1 := RegisterAlgorithm(fakeAlgo)
	require.NoError(t, err1, "register new algorithm")

	// Registering the same algorithm again should fail.
	err2 := RegisterAlgorithm(fakeAlgo)
	assert.Error(t, err2, "re-register algorithm with same name") //nolint:testifylint // assert.*Error* makes more sense

	gotAlgo := GetAlgorithm(fakeAlgoName)
	assert.NotNil(t, gotAlgo, "get registered algorithm")
	assert.Equal(t, gotAlgo, fakeAlgo, "get registered algorithm")
}

func TestGetFail(t *testing.T) {
	algo := GetAlgorithm("doesnotexist")
	assert.Nil(t, algo, "GetAlgorithm for non-existent compression algorithm should return nil")
}
