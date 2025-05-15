// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
 * Copyright (C) 2018 Cisco Systems
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

// Package umoci provides the top-level Go APIs for umoci functionality. Note
// that the umoci Go API is not yet considered stable -- we will aim to
// document any breaking changes in our changelogs but users should verify that
// updates will not break their usage of umoci.
package umoci

import (
	"fmt"

	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
)

// OpenLayout opens an existing OCI image layout, and fails if it does not
// exist.
func OpenLayout(imagePath string) (casext.Engine, error) {
	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return casext.Engine{}, fmt.Errorf("open CAS: %w", err)
	}

	return casext.NewEngine(engine), nil
}

// CreateLayout creates an existing OCI image layout, and fails if it already
// exists.
func CreateLayout(imagePath string) (casext.Engine, error) {
	err := dir.Create(imagePath)
	if err != nil {
		return casext.Engine{}, err
	}

	return OpenLayout(imagePath)
}
