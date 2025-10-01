// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
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

package internal

import (
	"errors"
)

// ErrUnimplemented is returned as a source error for umoci features that are
// not yet implemented.
var ErrUnimplemented = errors.New("unimplemented umoci feature")

// ErrInvalidEmptyJSON is returned from the mediatype parser if a descriptor
// with the "application/vnd.oci.empty.v1+json" media-type has any value other
// than "{}".
var ErrInvalidEmptyJSON = errors.New("empty json blob is invalid")
