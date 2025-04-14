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
	"strings"
)

const (
	// GzipSuffix is the standard media-type suffix for gzip compressed blobs.
	GzipSuffix = "gzip"
	// ZstdSuffix is the standard media-type suffix for zstd compressed blobs.
	ZstdSuffix = "zstd"
)

// SplitMediaTypeSuffix takes an OCI-style MIME media-type and splits it into a
// base type and a "+" suffix. For layer blobs, this is usually the compression
// algorithm ("+gzip", "+zstd"). For JSON blobs this is usually "+json".
//
// The returned media-type suffix does not contain a "+", and media-types not
// containing a "+" are treated as having an empty suffix.
//
// While this usage is technically sanctioned by by RFC 6838, this method
// should only be used for OCI media-types, where this behaviour is
// well-defined.
func SplitMediaTypeSuffix(mediaType string) (baseType, suffix string) {
	if suffixStart := strings.Index(mediaType, "+"); suffixStart >= 0 {
		return mediaType[:suffixStart], mediaType[suffixStart+1:]
	}
	return mediaType, ""
}
