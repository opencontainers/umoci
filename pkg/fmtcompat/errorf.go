/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2024 SUSE LLC
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

package fmtcompat

import (
	"fmt"
	"regexp"
	"runtime"

	"github.com/apex/log"
)

var printfVerbRegex = regexp.MustCompile("%[^%]")

// Errorf is like fmt.Errorf, except if ALL %w error values are nil, a nil
// error is returned. This is intended to be a compatibility workaround for
// migrating away from "github.com/pkg/errors".Wrap (which would return nil if
// the wrapped error is nil). A debug message is output if any values are nil.
func Errorf(fmtstr string, args ...interface{}) error {
	// We need to try to stop using this wrapper as soon as possible, so give a
	// debug log whenever we hit a user that us doing Errorf(nil).
	debugCaller := "<unknown caller>"
	if _, file, line, ok := runtime.Caller(1); ok {
		debugCaller = fmt.Sprintf("%s:%d", file, line)
	}

	var errorVerbs, nilErrors int
	verbs := printfVerbRegex.FindAllString(fmtstr, -1)
	if len(verbs) != len(args) {
		log.Warnf("[internal error] wrong number of arguments (%d) for Errorf format %q", len(args), fmtstr)
	} else {
		for idx, verb := range verbs {
			if verb != "%w" {
				continue
			}
			errorVerbs++
			if args[idx] == nil {
				nilErrors++
				log.Debugf("[internal error] wrapped error argument %d to Errorf(%q) from %s is a nil error", idx, fmtstr, debugCaller)
			}
		}
	}
	if errorVerbs > 0 && nilErrors == errorVerbs {
		log.Debugf("[internal error] all wrapped errors passed to Errorf(%q) from %s are nil errors", fmtstr, debugCaller)
		return nil
	}
	return fmt.Errorf(fmtstr, args...)
}
