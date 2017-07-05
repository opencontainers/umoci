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

package mtreefilter

import (
	"path/filepath"

	"github.com/apex/log"
	"github.com/vbatts/go-mtree"
)

// FilterFunc is a function used when filtering deltas with FilterDeltas.
type FilterFunc func(path string) bool

// isParent returns whether the path a is lexically an ancestor of the path b.
func isParent(a, b string) bool {
	a = filepath.Clean(a)
	b = filepath.Clean(b)

	for a != b && b != filepath.Dir(b) {
		b = filepath.Dir(b)
	}
	return a == b
}

// MaskFilter is a factory for FilterFuncs that will mask all InodeDelta paths
// that are lexical children of any path in the mask slice. All paths are
// considered to be relative to '/'.
func MaskFilter(masks []string) FilterFunc {
	return func(path string) bool {
		// Convert the path to be cleaned and relative-to-root.
		path = filepath.Join("/", path)

		// Check that no masks are matched.
		for _, mask := range masks {
			// Mask also needs to be cleaned and relative-to-root.
			mask = filepath.Join("/", mask)

			// Is it a parent?
			if isParent(mask, path) {
				log.Debugf("maskfilter: ignoring path %q matched by mask %q", path, mask)
				return false
			}
		}

		return true
	}
}

// FilterDeltas is a helper function to easily filter []mtree.InodeDelta with a
// filter function. Only entries which have `filter(delta.Path()) == true` will
// be included in the returned slice.
func FilterDeltas(deltas []mtree.InodeDelta, filter FilterFunc) []mtree.InodeDelta {
	var filtered []mtree.InodeDelta
	for _, delta := range deltas {
		if filter(delta.Path()) {
			filtered = append(filtered, delta)
		}
	}
	return filtered
}
