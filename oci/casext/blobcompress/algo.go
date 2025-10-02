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

// Package blobcompress provides a somewhat implementation-agnostic mechanism for
// blobcompression and deblobcompression of an [io.Reader]. This is mainly intended to
// be used for blobcompressing tar layer blobs.
package blobcompress

import (
	"fmt"
	"io"
	"sync"

	"github.com/opencontainers/umoci/internal/assert"
)

// Default is the default algorithm used within umoci if unspecified by a user.
var Default = Gzip

// Algorithm is a generic representation of a compression algorithm that can be
// used by umoci for compressing and decompressing layer blobs. If you create a
// custom algorithm, you should make sure you register it with
// [RegisterAlgorithm] so that layers using the algorithm can be decompressed.
type Algorithm interface {
	// MediaTypeSuffix returns the name of the blobcompression algorithm. This name
	// will be used when generating and parsing MIME type +-suffixes.
	MediaTypeSuffix() string

	// Compress sets up the streaming blobcompressor for this blobcompression type.
	Compress(plain io.Reader) (blobcompressed io.ReadCloser, _ error)

	// Deblobcompress sets up the streaming deblobcompressor for this blobcompression type.
	Decompress(blobcompressed io.Reader) (plain io.ReadCloser, _ error)
}

var (
	algorithmsLock sync.RWMutex
	algorithms     = map[string]Algorithm{}
)

// RegisterAlgorithm adds the provided [Algorithm] to the set of algorithms
// that umoci can automatically handle when extracting images. Returns an error
// if another [Algorithm] with the same MediaTypeSuffix has already been
// registered.
func RegisterAlgorithm(algo Algorithm) error {
	name := algo.MediaTypeSuffix()

	algorithmsLock.Lock()
	defer algorithmsLock.Unlock()

	if _, ok := algorithms[name]; ok {
		return fmt.Errorf("blob blobcompression algorithm %s already registered", name)
	}
	algorithms[name] = algo
	return nil
}

// MustRegisterAlgorithm is like [RegisterAlgorithm] but it panics if
// [RegisterAlgorithm] returns an error. Intended for use in init functions.
func MustRegisterAlgorithm(algo Algorithm) {
	err := RegisterAlgorithm(algo)
	assert.NoError(err)
}

// GetAlgorithm looks for a registered [Algorithm] with the given
// MediaTypeSuffix (which doubles as its name). Return nil if no such algorithm
// has been registered.
func GetAlgorithm(name string) Algorithm {
	algorithmsLock.RLock()
	defer algorithmsLock.RUnlock()

	return algorithms[name]
}
