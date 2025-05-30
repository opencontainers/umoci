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

// Package pathtrie provides a minimal implementation of a trie where each node
// is a path component, allowing you to efficiently store metadata about a
// path-based structure and iterate over subtrees within said structure.
package pathtrie

import (
	"path/filepath"
	"strings"
)

type trieNode[V any] struct {
	value    *V
	children map[string]*trieNode[V]

	// Used for bookkeeping to make delete much more efficient. Once
	// numChildren is 0 we can be sure it's safe to remove this node from
	// parent (which is connected via parent.children[key]).
	parent      *trieNode[V] // parent.children stores this node
	key         string       // parent.children[key] === this node
	numChildren uintptr      // how many total values set in this node and all descendants?
}

func newNode[V any](parent *trieNode[V], key string) *trieNode[V] {
	return &trieNode[V]{
		value:    nil,
		children: map[string]*trieNode[V]{},

		parent:      parent,
		key:         key,
		numChildren: 0,
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
		root.children[top] = newNode[V](root, top)
		next, exists = root.children[top]
	}
	if !exists {
		return nil
	}
	return next.lookupNode(pathKey(remaining), alloc)
}

// PathTrie represents a trie with each node being a path component.
type PathTrie[V any] struct {
	*trieNode[V]
}

// NewTrie returns a new empty [PathTrie].
func NewTrie[V any]() *PathTrie[V] {
	return &PathTrie[V]{newNode[V](nil, "")}
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
	} else {
		// If we've inserted a new value we need to update the bookkeeping of
		// how many children there are in the trie branch.
		for cur := node; cur != nil; cur = cur.parent {
			cur.numChildren++
		}
	}
	node.value = &value
	return old, found
}

func (t *PathTrie[V]) remove(path string, recursive bool) (old V, found bool) {
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

		// Update the bookkeeping for children and prune any unused parts of
		// the trie.
		var deletedChildren uintptr
		if recursive {
			// For recusive removals, just detach the child from the tree.
			delete(node.children, file)
			deletedChildren = child.numChildren
		} else if found {
			// If there was a value for non-recursive removal we've only
			// removed one child.
			deletedChildren = 1
		}
		// Update the child count up the stack and prune any branches that no
		// longer have actual children.
		for cur := child; cur != nil; cur = cur.parent {
			cur.numChildren -= deletedChildren
			if cur.numChildren == 0 && cur.parent != nil {
				delete(cur.parent.children, cur.key)
			}
		}
	}
	return old, found
}

// Delete removes the value at the given path, but any child entries are kept
// as-is. To remove the entry and all child entries use [DeleteAll]. If there
// was a value at the given path, the old value is returned and found will be
// true.
func (t *PathTrie[V]) Delete(path string) (old V, found bool) {
	return t.remove(path, false)
}

// DeleteAll removes the entry at the given path and all child entries. To only
// delete the value at the path use [Delete]. If there was a value at the given
// path, the old value is returned and found will be true.
func (t *PathTrie[V]) DeleteAll(path string) (old V, found bool) {
	return t.remove(path, true)
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
