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
	"encoding/json"
	"fmt"
	"net/url"
)

// MerkleRoot holds a single resolved Merkle root.
type MerkleRoot struct {
	// MediaType is the media type of Root.
	MediaType string

	// Root is the Merkle root object.  While this may be of any type.
	// OCI tools will generally use image-spec Descriptors.
	Root interface{}

	// URI is the source, if any, from which Root was retrieved.  It can
	// be used to expand any relative reference contained within Root.
	URI *url.URL
}

// UnmarshalJSON reads 'mediaType', 'root', and 'uri' properties into
// MediaType, Root, and URI respectively.  The main difference from
// the stock json.Unmarshal implementation is that the 'uri' value is
// read from a string instead of from an object with Scheme, Host,
// etc. properties.
func (root *MerkleRoot) UnmarshalJSON(b []byte) (err error) {
	var dataInterface interface{}
	if err := json.Unmarshal(b, &dataInterface); err != nil {
		return err
	}

	data, ok := dataInterface.(map[string]interface{})
	if !ok {
		return fmt.Errorf("merkle root is not a JSON object: %v", dataInterface)
	}

	mediaTypeInterface, ok := data["mediaType"]
	if !ok {
		root.MediaType = ""
	} else {
		mediaTypeString, ok := mediaTypeInterface.(string)
		if !ok {
			return fmt.Errorf("merkle root mediaType is not a string: %v", mediaTypeInterface)
		}
		root.MediaType = mediaTypeString
	}

	root.Root = data["root"]

	uriInterface, ok := data["uri"]
	if !ok {
		root.URI = nil
	} else {
		uriString, ok := uriInterface.(string)
		if !ok {
			return fmt.Errorf("merkle root uri is not a string: %v", uriInterface)
		}
		root.URI, err = url.Parse(uriString)
		if err != nil {
			return err
		}
	}

	return nil
}

// MarshalJSON writes 'mediaType', 'root', and 'uri' properties to the
// output object.  The main difference from the stock json.Marshal
// implementation is that the 'uri' value is written as a string
// instead of an object with Scheme, Host, etc. properties.
func (root MerkleRoot) MarshalJSON() ([]byte, error) {
	data := map[string]interface{}{}
	if root.MediaType != "" {
		data["mediaType"] = root.MediaType
	}
	if root.Root != nil {
		data["root"] = root.Root
	}
	if root.URI != nil {
		data["uri"] = root.URI.String()
	}
	return json.Marshal(data)
}
