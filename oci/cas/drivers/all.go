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

// Package drivers is an empty package which has subpackages that implement
// cas.Drivers (and register said drivers with cas). Importing this package
// will register all official OCI cas drivers.
package drivers

// Import all official OCI drivers.
import (
	// Implements directory-backed OCI layouts.
	_ "github.com/openSUSE/umoci/oci/cas/drivers/dir"
)
