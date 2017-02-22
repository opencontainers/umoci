/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2017 SUSE LLC.
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

package dir

import (
	"os"

	"github.com/openSUSE/umoci/oci/cas"
)

// Driver is an implementation of drivers.Driver for local directory-backed OCI
// image layouts.
var Driver cas.Driver = dirDriver{}

type dirDriver struct{}

// Supported returns whether the resource at the given URI is supported by the
// driver (used for auto-detection). If two drivers support the same URI, then
// the earliest registered driver takes precedence.
//
// Note that this is _not_ a validation of the URI -- if the URI refers to an
// invalid or non-existent resource it is expected that the URI is "supported".
func (d dirDriver) Supported(uri string) bool {
	fi, err := os.Stat(uri)
	if err != nil {
		// If we got an error, we only support it if the error is that the
		// target doesn't exist -- Create handles creating the necessary
		// directories.
		return os.IsNotExist(err)
	}
	// dir stands for directory
	return fi.IsDir()
}

// Open "opens" a new CAS engine accessor for the given URI.
func (d dirDriver) Open(uri string) (cas.Engine, error) {
	return Open(uri)
}

// Create creates a new image at the provided URI.
func (d dirDriver) Create(uri string) error {
	return Create(uri)
}

func init() {
	cas.Register(Driver)
}
