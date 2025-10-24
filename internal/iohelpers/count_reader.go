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

package iohelpers

import (
	"io"
)

// CountingReader is an [io.Reader] wrapper that counts how many bytes were read
// from the underlying [io.Reader].
type CountingReader struct {
	R io.Reader // underlying reader
	N int64     // number of bytes read
}

// CountReader returns a new *CountingReader that wraps the given [io.Reader].
func CountReader(rdr io.Reader) *CountingReader {
	return &CountingReader{R: rdr, N: 0}
}

func (c *CountingReader) Read(p []byte) (int, error) {
	n, err := c.R.Read(p)
	c.N += int64(n)
	return n, err
}

// BytesRead returns the number of bytes read so far from the reader. This is
// just shorthand for c.N.
func (c CountingReader) BytesRead() int64 {
	return c.N
}

// TODO: What about WriteTo?
