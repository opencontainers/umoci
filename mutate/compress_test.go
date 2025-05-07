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
	"testing"

	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	fact = "meshuggah rocks!!!"
)

func TestNoopCompressor(t *testing.T) {
	buf := bytes.NewBufferString(fact)

	r, err := NoopCompressor.Compress(buf)
	require.NoError(t, err, "noop compress buffer")
	assert.Empty(t, NoopCompressor.MediaTypeSuffix(), "noop compressor MediaTypeSuffix")

	content, err := io.ReadAll(r)
	require.NoError(t, err, "read from noop compressor")

	assert.Equal(t, fact, string(content), "noop compressed data")
}

func TestGzipCompressor(t *testing.T) {
	buf := bytes.NewBufferString(fact)
	c := GzipCompressor

	r, err := c.Compress(buf)
	require.NoError(t, err, "gzip compress buffer")
	assert.Equal(t, "gzip", c.MediaTypeSuffix(), "gzip compressor MediaTypeSuffix")

	r, err = gzip.NewReader(r)
	require.NoError(t, err, "read gzip data")

	content, err := io.ReadAll(r)
	require.NoError(t, err, "read from round-tripped gzip")

	assert.Equal(t, fact, string(content), "gzip round-trip data")
}

func TestZstdCompressor(t *testing.T) {
	buf := bytes.NewBufferString(fact)
	c := ZstdCompressor

	r, err := c.Compress(buf)
	require.NoError(t, err, "zstd compress buffer")
	assert.Equal(t, "zstd", c.MediaTypeSuffix(), "zstd compressor MediaTypeSuffix")

	dec, err := zstd.NewReader(r)
	require.NoError(t, err, "read zstd data")

	content, err := io.ReadAll(dec)
	require.NoError(t, err, "read from round-tripped zstd")

	assert.Equal(t, fact, string(content), "zstd round-trip data")
}
