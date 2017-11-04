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

package refenginediscovery

import (
	"net/url"

	"github.com/xiekeyang/oci-discovery/tools/engine"
	"golang.org/x/net/context"
)

// Engines holds application/vnd.oci.ref-engines.v1+json data.
type Engines struct {

	// RefEngines is an array of ref-engine configurations.
	RefEngines []engine.Config `json:"refEngines,omitempty"`

	// CASEngines is an array of CAS-engine configurations.
	CASEngines []engine.Config `json:"casEngines,omitempty"`
}

// RefEnginesReference holds resolved Engines data.
type RefEnginesReference struct {

	// Engines holds the resolved Engines declaration.
	Engines Engines

	// URI is the source, if any, from which Engines was retrieved.  It
	// can be used to expand any relative reference contained within
	// Engines.
	URI *url.URL
}

// RefEngineReference holds a single resolved ref-engine object.
type RefEngineReference struct {
	// Config holds a single resolved ref-engine config.
	Config engine.Reference

	// CASEngines holds the ref-engines object's CAS-engine suggestions,
	// if any.
	CASEngines []engine.Reference
}

// RefEngineReferenceCallback templates a callback for use in RefEngineReferences.
type RefEngineReferenceCallback func(ctx context.Context, reference RefEngineReference) (err error)

// Engine represents a ref-engine discovery engine.
type Engine interface {

	// RefEngines calculates ref engines using Ref-Engine Discovery and
	// calls refEngineCallback on each one.  Discover returns any errors
	// returned by refEngineCallback and aborts further iteration.
	RefEngines(ctx context.Context, name string, callback RefEngineReferenceCallback) (err error)

	// Close releases resources held by the engine.  Subsequent engine
	// method calls will fail.
	Close(ctx context.Context) (err error)
}
