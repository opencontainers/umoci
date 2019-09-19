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

package v2

import (
	"github.com/openSUSE/umoci/oci/casext/mediatype"
	"github.com/opencontainers/image-spec/specs-go/v1"
)

// MediaTypeImageManifest is the OCIv2 version of ImageManifest.
const MediaTypeImageManifest = "application/x-umoci/ociv2.manifest.v0+json"

// Manifest provides `application/vnd.cyphar.oci.image.manifest.v0+json`
// mediatype structure when marshalled to JSON.
type Manifest struct {
	// Config references a configuration object for a container, by digest.
	// The referenced configuration object is a JSON blob that the runtime uses
	// to set up the container.
	Config v1.Descriptor `json:"config"`

	// Root is a descriptor to the root of the inode tree representing the
	// filesystem (must be a directory).
	Root v1.Descriptor `json:"root"`

	// Annotations contains arbitrary metadata for the image manifest.
	Annotations map[string]string `json:"annotations,omitempty"`
}

func init() {
	mediatype.RegisterTarget(MediaTypeImageManifest)
	mediatype.RegisterParser(MediaTypeImageManifest, mediatype.CustomJSONParser(Manifest{}))
}
