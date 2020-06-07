/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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

// makeRoot does a very simple job of converting a path to a lexical
// relative-to-root. In mtree we don't deal with any symlink components.
func makeRoot(path string) string {
	return filepath.Join(string(filepath.Separator), path)
}

func maskFilter(maskedPaths map[string]struct{}, includeSelf bool) FilterFunc {
	return func(path string) bool {
		// Convert the path to be cleaned and relative-to-root.
		path = makeRoot(path)

		// Check that no ancestor of the path is a masked path.
		for parent := path; parent != filepath.Dir(parent); parent = filepath.Dir(parent) {
			if _, ok := maskedPaths[parent]; !ok {
				continue
			}
			if parent == path && !includeSelf {
				continue
			}
			log.Debugf("maskfilter: ignoring path %q matched by mask %q", path, parent)
			return false
		}
		return true
	}
}

// MaskFilter is a factory for FilterFuncs that will mask all InodeDelta paths
// that are lexical children of any path in the mask slice. All paths are
// considered to be relative to '/'.
func MaskFilter(masks []string) FilterFunc {
	maskedPaths := map[string]struct{}{}
	for _, mask := range masks {
		maskedPaths[makeRoot(mask)] = struct{}{}
	}
	return maskFilter(maskedPaths, true)
}

// SimplifyFilter is a factory that takes a list of InodeDelta and creates a
// filter to filter out all deletion entries that have a parent which also has
// a deletion entry. This is necessary to both reduce our image sizes and
// remain compatible with Docker's now-incompatible image format (the OCI spec
// doesn't require this behaviour but it's now needed because of course Docker
// won't fix their own bugs).
func SimplifyFilter(deltas []mtree.InodeDelta) FilterFunc {
	deletedPaths := make(map[string]struct{})
	for _, delta := range deltas {
		if delta.Type() != mtree.Missing {
			continue
		}
		deletedPaths[makeRoot(delta.Path())] = struct{}{}
	}
	return maskFilter(deletedPaths, false)
}

// FilterDeltas is a helper function to easily filter []mtree.InodeDelta with a
// filter function. Only entries which have `filter(delta.Path()) == true` will
// be included in the returned slice.
func FilterDeltas(deltas []mtree.InodeDelta, filters ...FilterFunc) []mtree.InodeDelta {
	var filtered []mtree.InodeDelta
	for _, delta := range deltas {
		var blocked bool
		for _, filter := range filters {
			if !filter(delta.Path()) {
				blocked = true
				break
			}
		}
		if !blocked {
			filtered = append(filtered, delta)
		}
	}
	return filtered
}
