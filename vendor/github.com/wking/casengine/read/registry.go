// Copyright 2017 casengine contributors
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

// Package read implements the CAS-engine protocol registry.
package read

import (
	"net/url"

	"github.com/wking/casengine"
	"golang.org/x/net/context"
)

// New creates a new CAS-engine ReadCloser.
type New func(ctx context.Context, baseURI *url.URL, config interface{}) (engine casengine.ReadCloser, err error)

// Constructors holds CAS-engine generators associated with registered
// protocol identifiers.
var Constructors = map[string]New{}
