// Copyright 2017 casengine contributors
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

// Package counter defines a byte-counting writer.  One use case is measuring the size of content being streamed into CAS.
package counter

type Counter struct {
	count uint64
}

// Write implements io.Writer for Counter.
func (c *Counter) Write(p []byte) (n int, err error) {
	c.count += uint64(len(p))
	return len(p), nil
}

// Count returns the number of bytes which have been written to this
// Counter.
func (c *Counter) Count() (n uint64) {
	return c.count
}
