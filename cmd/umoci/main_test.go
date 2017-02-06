/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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
	"os"
	"strings"
	"testing"
)

// Build:
//  $ go test -c -covermode=count -o umoci \
//            -cover -coverpkg=github.com/openSUSE/umoci/... \
//            github.com/openSUSE/umoci/cmd/umoci
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
	os.Args = args

	if run {
		main()
	}
}
