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

package mutate

import (
	"bytes"
	"io"
	"io/ioutil"
	"testing"

	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/stretchr/testify/assert"
)

const (
	fact = "meshuggah rocks!!!"
)

func TestNoopCompressor(t *testing.T) {
	assert := assert.New(t)
	buf := bytes.NewBufferString(fact)

	r, err := NoopCompressor.Compress(buf)
	assert.NoError(err)
	assert.Equal(NoopCompressor.MediaTypeSuffix(), "")

	content, err := ioutil.ReadAll(r)
	assert.NoError(err)

	assert.Equal(string(content), fact)
}

func TestGzipCompressor(t *testing.T) {
	assert := assert.New(t)

	buf := bytes.NewBufferString(fact)
	c := GzipCompressor

	r, err := c.Compress(buf)
	assert.NoError(err)
	assert.Equal(c.MediaTypeSuffix(), "gzip")

	r, err = gzip.NewReader(r)
	assert.NoError(err)

	content, err := ioutil.ReadAll(r)
	assert.NoError(err)

	assert.Equal(string(content), fact)
}

func TestZstdCompressor(t *testing.T) {
	assert := assert.New(t)

	buf := bytes.NewBufferString(fact)
	c := ZstdCompressor

	r, err := c.Compress(buf)
	assert.NoError(err)
	assert.Equal(c.MediaTypeSuffix(), "zstd")

	dec, err := zstd.NewReader(r)
	assert.NoError(err)

	var content bytes.Buffer
	_, err = io.Copy(&content, dec)
	assert.NoError(err)
	assert.Equal(content.String(), fact)
}
