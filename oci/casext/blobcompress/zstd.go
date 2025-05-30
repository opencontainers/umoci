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
	"fmt"
	"io"

	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/oci/casext/mediatype"

	"github.com/apex/log"
	zstd "github.com/klauspost/compress/zstd"
)

// Zstd provides zstd blobcompression and deblobcompression.
var Zstd Algorithm = zstdAlgo{}

type zstdAlgo struct{}

func (zs zstdAlgo) MediaTypeSuffix() string {
	return mediatype.ZstdSuffix
}

func (zs zstdAlgo) Compress(reader io.Reader) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()
	zw, err := zstd.NewWriter(pipeWriter)
	if err != nil {
		return nil, err
	}
	go func() {
		_, err := system.Copy(zw, reader)
		if err != nil {
			log.Warnf("zstd blobcompress: could not blobcompress layer: %v", err)
			_ = pipeWriter.CloseWithError(fmt.Errorf("blobcompressing layer: %w", err))
			return
		}
		if err := zw.Close(); err != nil {
			log.Warnf("zstd blobcompress: could not close gzip writer: %v", err)
			_ = pipeWriter.CloseWithError(fmt.Errorf("close zstd writer: %w", err))
			return
		}
		if err := pipeWriter.Close(); err != nil {
			log.Warnf("zstd blobcompress: could not close pipe: %v", err)
			// We don't CloseWithError because we cannot override the Close.
			return
		}
	}()

	return pipeReader, nil
}

func (zs zstdAlgo) Decompress(reader io.Reader) (io.ReadCloser, error) {
	plain, err := zstd.NewReader(reader)
	if err != nil {
		return nil, err
	}
	return plain.IOReadCloser(), nil
}

func init() {
	MustRegisterAlgorithm(Zstd)
}
