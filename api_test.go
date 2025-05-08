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

package umoci

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateExistingFails(t *testing.T) {
	dir := t.TempDir()

	// opening a bad layout should fail
	_, err := OpenLayout(dir)
	assert.Error(t, err, "open invalid layout") //nolint:testifylint // assert.*Error* makes more sense

	// remove directory so that create can create it
	require.NoError(t, os.RemoveAll(dir))

	// create should work
	_, err = CreateLayout(dir)
	assert.NoError(t, err, "create new layout") //nolint:testifylint // assert.*Error* makes more sense

	// but not twice
	_, err = CreateLayout(dir)
	assert.Error(t, err, "create should not work on existing layout") //nolint:testifylint // assert.*Error* makes more sense

	// but open should work now
	_, err = OpenLayout(dir)
	assert.NoError(t, err, "open layout") //nolint:testifylint // assert.*Error* makes more sense
}
