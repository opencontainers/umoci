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
	"io"

	"github.com/opencontainers/umoci/oci/casext/blobcompress"
)

// Compressor is an interface which users can use to implement different
// compression types.
//
// Deprecated: The compression algorithm logic has been moved to [blobcompress]
// to unify the media-type compression handling for both compression and
// decompression. Please switch to [blobcompress.Algorithm].
type Compressor interface {
	// Compress sets up the streaming compressor for this compression type.
	Compress(io.Reader) (io.ReadCloser, error)

	// MediaTypeSuffix returns the suffix to be added to the layer to
	// indicate what compression type is used, e.g. "gzip", or "" for no
	// compression.
	MediaTypeSuffix() string
}

// Deprecated: The compression algorithm logic has been moved to [blobcompress]
// to unify the media-type compression handling for both compression and
// decompression. Please switch to blobcompress.
var (
	// NoopCompressor provides no compression.
	NoopCompressor Compressor = blobcompress.Noop
	// GzipCompressor provides gzip compression.
	GzipCompressor Compressor = blobcompress.Gzip
	// ZstdCompressor provides zstd compression.
	ZstdCompressor Compressor = blobcompress.Zstd
)
