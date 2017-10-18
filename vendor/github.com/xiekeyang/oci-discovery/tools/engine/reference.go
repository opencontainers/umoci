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

package engine

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// Reference holds a single resolved engine config.
type Reference struct {

	// Config is the engine configuration.
	Config Config

	// URI is the source, if any, from which Config was retrieved.  It
	// can be used to expand any relative reference contained within
	// Config.
	URI *url.URL
}

// UnmarshalJSON reads 'config' and 'uri' properties into Config and
// URI respectively.  The main difference from the stock
// json.Unmarshal implementation is that the 'uri' value is read from
// a string instead of from an object with Scheme, Host,
// etc. properties.
func (reference *Reference) UnmarshalJSON(b []byte) (err error) {
	var dataInterface interface{}
	if err := json.Unmarshal(b, &dataInterface); err != nil {
		return err
	}

	data, ok := dataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("reference is not a JSON object: %v", dataInterface)
	}

	configInterface, ok := data["config"]
	if !ok {
		return fmt.Errorf("reference missing required 'config' entry: %v", data)
	}

	err = reference.Config.unmarshalInterface(configInterface)
	if err != nil {
		return err
	}

	uriInterface, ok := data["uri"]
	if !ok {
		reference.URI = nil
	} else {
		uriString, ok := uriInterface.(string)
		if !ok {
			return fmt.Errorf("reference uri is not a string: %v", uriInterface)
		}
		reference.URI, err = url.Parse(uriString)
		if err != nil {
			return err
		}
	}

	return nil
}

// MarshalJSON writes 'config' and 'uri' properties to the output
// object.  The main difference from the stock json.Marshal
// implementation is that the 'uri' value is written as a string instead
// of an object with Scheme, Host, etc. properties.
func (reference Reference) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{}
	data["config"] = reference.Config
	if reference.URI != nil {
		data["uri"] = reference.URI.String()
	}
	return json.Marshal(data)
}
