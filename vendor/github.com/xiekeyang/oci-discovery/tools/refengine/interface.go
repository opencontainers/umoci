// Copyright 2017 oci-discovery contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package refengine

import (
	"net/url"

	"golang.org/x/net/context"
)

// Engine represents a reference engine.
type Engine interface {

	// Get returns an array of potential Merkle roots from the store.
	// When no results are found, roots will be an empty array and err
	// will be nil.
	Get(ctx context.Context, name string) (roots []MerkleRoot, err error)

	// Close releases resources held by the engine.  Subsequent engine
	// method calls will fail.
	Close(ctx context.Context) (err error)
}

// New creates a new ref-engine instance.
type New func(ctx context.Context, baseURI *url.URL, config interface{}) (engine Engine, err error)
