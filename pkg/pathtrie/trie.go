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

package pathtrie

import (
	"path/filepath"
	"strings"
)

type trieNode[V any] struct {
	value    *V
	children map[string]*trieNode[V]
}

func newNode[V any]() *trieNode[V] {
	return &trieNode[V]{
		value:    nil,
		children: map[string]*trieNode[V]{},
	}
}

type pathKey string

func pathToKey(path string) pathKey {
	path = filepath.Clean(string(filepath.Separator) + path)
	if filepath.IsAbs(path) {
		path = strings.TrimPrefix(path, string(filepath.Separator))
	}
	path = filepath.ToSlash(path)
	return pathKey(path)
}

// lookupNode looks up the node at the given path. If alloc is set then any
// missing nodes are created. path MUST be filepath.ToSlash and
// filepath.Clean'd.
func (root *trieNode[V]) lookupNode(path pathKey, alloc bool) *trieNode[V] {
	if path == "" {
		return root
	}
	top, remaining, _ := strings.Cut(string(path), "/")

	next, exists := root.children[top]
	if !exists && alloc {
		root.children[top] = newNode[V]()
		next, exists = root.children[top]
	}
	if !exists {
		return nil
	}
	return next.lookupNode(pathKey(remaining), alloc)
}

type PathTrie[V any] struct {
	*trieNode[V]
}

// NewTrie returns a new empty [PathTrie].
func NewTrie[V any]() *PathTrie[V] {
	return &PathTrie[V]{newNode[V]()}
}

// Get looks up the given path in the trie. Completely non-existent paths and
// paths with no value (such as intermediate paths) are considered the same.
func (t *PathTrie[V]) Get(path string) (val V, found bool) {
	key := pathToKey(path)
	node := t.lookupNode(key, false)
	if node != nil && node.value != nil {
		return *node.value, true
	}
	return val, false
}

// Set adds a new value to the trie at the given path with the given value. If
// there was a value at the given path, the old value is returned and found
// will be true.
func (t *PathTrie[V]) Set(path string, value V) (old V, found bool) {
	key := pathToKey(path)
	node := t.lookupNode(key, true)
	if node.value != nil {
		old = *node.value
		found = true
	}
	node.value = &value
	return old, found
}

func (t *PathTrie[V]) delete(path string, recursive bool) (old V, found bool) {
	// Need to look up the parent node of the path.
	dir, file := filepath.Split(filepath.Clean(path))
	dirKey := pathToKey(dir)
	node := t.lookupNode(dirKey, false)
	if node == nil {
		return old, false
	}
	if child, ok := node.children[file]; ok {
		// Delete the value from the node if it existed.
		if child.value != nil {
			old, found = *child.value, true
		}
		child.value = nil
		// If the node has no children (or we are doing recursive removals),
		// remove it as well to avoid keeping around garbage.
		//
		// TODO: In the case of us deleting an intermediate node that had a
		// child several layers deeper that has been removed, we will not
		// delete the now-unneeded nodes. The most obvious optimisation is to
		// store a count of the number of children per-layer which we update up
		// the tree recursively so we can detect if it's safe to prune the
		// children -- the only downside is it will complicated our simple
		// lookupNode code.
		if recursive || len(child.children) == 0 {
			delete(node.children, file)
		}
	}
	return old, found
}

// Delete removes the value at the given path, but any child entries are kept
// as-is. To remove the entry and all child entries use [DeleteAll]. If there
// was a value at the given path, the old value is returned and found will be
// true.
func (t *PathTrie[V]) Delete(path string) (old V, found bool) {
	return t.delete(path, false)
}

// DeleteAll removes the entry at the given path and all child entries. To only
// delete the value at the path use [Delete]. If there was a value at the given
// path, the old value is returned and found will be true.
func (t *PathTrie[V]) DeleteAll(path string) (old V, found bool) {
	return t.delete(path, true)
}

// WalkFunc is the callback function called by [WalkFrom].
type WalkFunc[V any] func(path string, value V) error

func walk[V any](current *trieNode[V], path string, walkFn WalkFunc[V]) error {
	if current.value != nil {
		if err := walkFn(path, *current.value); err != nil {
			return err
		}
	}
	for file, child := range current.children {
		childPath := filepath.Join(path, file)
		if err := walk(child, childPath, walkFn); err != nil {
			return err
		}
	}
	return nil
}

// WalkFrom does a depth-first walk of the trie starting at the given path. The
// exact order of execution is not guaranteed to be the same each time (as node
// children are stored as maps).
func (t *PathTrie[V]) WalkFrom(path string, walkFn WalkFunc[V]) error {
	key := pathToKey(path)
	node := t.lookupNode(key, false)
	if node != nil {
		return walk(node, filepath.Clean(path), walkFn)
	}
	return nil
}
