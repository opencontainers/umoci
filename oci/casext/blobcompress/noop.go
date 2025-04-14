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
	"io"
)

// Noop does no compression.
var Noop Algorithm = noopAlgo{}

type noopAlgo struct{}

func (n noopAlgo) MediaTypeSuffix() string {
	return ""
}

func (n noopAlgo) Compress(reader io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(reader), nil
}

func (n noopAlgo) Decompress(reader io.Reader) (io.ReadCloser, error) {
	return io.NopCloser(reader), nil
}

func init() {
	MustRegisterAlgorithm(Noop)
}
