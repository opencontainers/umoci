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

// Package engine implements types common to both ref- and CAS-engines.
package engine

import (
	"encoding/json"
	"fmt"
)

// Config represents a ref- or CAS-engine configuration.
type Config struct {

	// Protocol is a required part of refEngines and casEngines entries.
	Protocol string `json:"protocol"`

	// Data holds the protocol-specific configuration.
	Data map[string]interface{}
}

// UnmarshalJSON reads a 'protocol' key into Protocol and any
// remaining keys into Data.
func (c *Config) UnmarshalJSON(b []byte) (err error) {
	var dataInterface interface{}
	if err := json.Unmarshal(b, &dataInterface); err != nil {
		return err
	}

	return c.unmarshalInterface(dataInterface)
}

func (c *Config) unmarshalInterface(d interface{}) (err error) {
	data, ok := d.(map[string]interface{})
	if !ok {
		return fmt.Errorf("engine config is not a JSON object: %v", d)
	}

	protocolInterface, ok := data["protocol"]
	if !ok {
		return fmt.Errorf("engine config missing required 'protocol' entry: %v", data)
	}

	c.Protocol, ok = protocolInterface.(string)
	if !ok {
		return fmt.Errorf("engine config protocol is not a string: %v", protocolInterface)
	}

	delete(data, "protocol")
	c.Data = data
	return nil
}

// MarshalJSON writes any key/value pairs from Data and ensures
// 'protocol' is set equal to Protocol (clobbering a Data["protocol"]
// entry, if any).
func (c Config) MarshalJSON() ([]byte, error) {
	var data map[string]interface{}
	data = make(map[string]interface{})
	for key, value := range c.Data {
		data[key] = value
	}
	data["protocol"] = c.Protocol
	return json.Marshal(data)
}
