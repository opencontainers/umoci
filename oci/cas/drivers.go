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

package cas

import (
	"sync"

	"github.com/pkg/errors"
)

// TODO: URIs need to be handled better, with some way of specifying what the
//       format or protocol is meant to be used. Currently Create(...) doesn't
//       work properly.

// Driver is an interface describing a CAS driver that can be used to create
// new cas.Engine instances. The intention is for this to be generic enough
// that multiple backends can be implemented for umoci and other tools without
// requiring changes to other components of such tools.
type Driver interface {
	// Supported returns whether the resource at the given URI is supported by
	// the driver (used for auto-detection). If two drivers support the same
	// URI, then the earliest registered driver takes precedence.
	//
	// Note that this is _not_ a validation of the URI -- if the URI refers to
	// an invalid or non-existent resource it is expected that the URI is
	// "supported".
	Supported(uri string) bool

	// Open "opens" a new CAS engine accessor for the given URI.
	Open(uri string) (Engine, error)

	// Create creates a new image at the provided URI.
	Create(uri string) error
}

var (
	dm      sync.RWMutex
	drivers []Driver
)

// Register registers a new Driver in the global set of drivers. This is
// intended to be called from the init function in packages that implement
// cas.Engine (similar to the crypto package).
func Register(driver Driver) {
	dm.Lock()
	drivers = append(drivers, driver)
	dm.Unlock()
}

func findSupported(uri string) Driver {
	dm.RLock()
	defer dm.RUnlock()

	for _, driver := range drivers {
		if driver.Supported(uri) {
			return driver
		}
	}
	return nil
}

// Open returns a new cas.Engine created by one of the registered drivers that
// support the provided URI (if no such driver exists, an error is returned).
// If more than one driver supports the provided URI, the first of the
// candidate drivers to have been registered is chosen.
func Open(uri string) (Engine, error) {
	driver := findSupported(uri)
	if driver == nil {
		return nil, errors.Errorf("drivers: unsupported uri: %s", uri)
	}

	return driver.Open(uri)
}

// Create creates a new image by one of the registered drivers that support the
// provided URI (if no such driver exists, an error is returned). If more than
// one driver supports the provided URI, the first of the candidate drivers to
// be registered is chosen.
func Create(uri string) error {
	driver := findSupported(uri)
	if driver == nil {
		return errors.Errorf("drivers: unsupported uri: %s", uri)
	}

	return driver.Create(uri)
}
