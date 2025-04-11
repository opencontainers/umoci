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
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
)

func testAlgo(t *testing.T, algo Algorithm, expectedName string, expectedDiff bool) {
	assert := assert.New(t)

	const data = "meshuggah rocks!!!"

	plainBuf := bytes.NewBufferString(data)

	r, err := algo.Compress(plainBuf)
	assert.NoError(err)
	assert.Equal(algo.MediaTypeSuffix(), expectedName)

	compressed, err := ioutil.ReadAll(r)
	assert.NoError(err)
	if expectedDiff {
		assert.NotEqual(string(compressed), data)
	} else {
		assert.Equal(string(compressed), data)
	}

	compressedBuf := bytes.NewBuffer(compressed)

	r, err = algo.Decompress(compressedBuf)
	assert.NoError(err)

	content, err := ioutil.ReadAll(r)
	assert.NoError(err)

	assert.Equal(string(content), data)
}

type fakeAlgo struct{ name string }

var _ Algorithm = fakeAlgo{"foo"}

func (f fakeAlgo) MediaTypeSuffix() string                       { return f.name }
func (f fakeAlgo) Compress(r io.Reader) (io.ReadCloser, error)   { return nil, fmt.Errorf("err") }
func (f fakeAlgo) Decompress(r io.Reader) (io.ReadCloser, error) { return nil, fmt.Errorf("err") }

func TestRegister(t *testing.T) {
	assert := assert.New(t)

	var fakeAlgo Algorithm = fakeAlgo{"fake-algo1"}
	fakeAlgoName := fakeAlgo.MediaTypeSuffix()

	err := RegisterAlgorithm(fakeAlgo)
	assert.NoError(err)

	gotAlgo := GetAlgorithm(fakeAlgoName)
	assert.NotNil(gotAlgo)
	assert.Equal(gotAlgo, fakeAlgo)
}

func TestRegisterFail(t *testing.T) {
	assert := assert.New(t)

	var fakeAlgo Algorithm = fakeAlgo{"fake-algo2"}
	fakeAlgoName := fakeAlgo.MediaTypeSuffix()

	err1 := RegisterAlgorithm(fakeAlgo)
	assert.NoError(err1)

	// Registering the same algorithm again should fail.
	err2 := RegisterAlgorithm(fakeAlgo)
	assert.Error(err2)

	gotAlgo := GetAlgorithm(fakeAlgoName)
	assert.NotNil(gotAlgo)
	assert.Equal(gotAlgo, fakeAlgo)
}

func TestGetFail(t *testing.T) {
	assert := assert.New(t)

	algo := GetAlgorithm("doesnotexist")
	assert.Nil(algo, "GetAlgorithm for non-existent compression algorithm should return nil")
}
