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

package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

// TODO: Replace all of this with "go build -cover".

// Build:
//  $ go test -c -covermode=count -o umoci \
//            -cover -coverpkg=github.com/opencontainers/umoci/... \
//            github.com/opencontainers/umoci/cmd/umoci
// Run:
//  $ ./umoci __DEVEL--i-heard-you-like-tests -test.coverprofile [file] [args]...

// TestUmoci is a hack that allows us to figure out what the coverage is during
// integration tests. I would not recommend that you use a binary built using
// this hack outside of a test suite.
func TestUmoci(t *testing.T) {
	var (
		args []string
		run  bool
	)

	for _, arg := range os.Args {
		switch {
		case arg == "__DEVEL--i-heard-you-like-tests":
			run = true
		case strings.HasPrefix(arg, "-test"):
		case strings.HasPrefix(arg, "__DEVEL"):
		default:
			args = append(args, arg)
		}
	}

	if run {
		if err := Main(args); err != nil {
			// Output to stderr rather than the test log so that the
			// integration tests can properly handle cleaning up the output.
			fmt.Fprintf(os.Stderr, "%v\n", err)
			t.Fail()
		}
	}

	// Before returning, we change stdout to /dev/null because "go test"
	// binaries will output information to stdout that interferes with our bats
	// tests (namely the PASS/FAIL line as well as the coverage information).
	//
	// Unfortunately there appears to be no way to block the "--- FAIL:" text
	// in case of an error...
	os.Stdout, _ = os.Create(os.DevNull)
}
