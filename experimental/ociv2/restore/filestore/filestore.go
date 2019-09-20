/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2019 SUSE LLC.
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

// TODO: All of this needs to be reworked to be lookup-safe.

package filestore

import (
	"io"
	"os"
	// Needed for digest.SHA256.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

const StoreAlgorithm = digest.SHA256

type Store interface {
	// Put inserts the given data as a new file-inode into the store.
	Put(ctx context.Context, data io.Reader) (digest.Digest, int64, error)

	// Get a read-only handle to a file-inode inside the store. If there is no
	// file-inode in the store matching the contents, (nil, nil) is returned.
	Get(ctx context.Context, digest digest.Digest) (*os.File, error)

	// Close and clean up the handle.
	Close() error
}

// Open retrieves a handle to a file-store (creating a new one if necessary).
func Open(root string) (Store, error) {
	if err := os.MkdirAll(root, 0700); err != nil {
		return nil, errors.Wrap(err, "create root")
	}
	return &store{root: root}, nil
}
