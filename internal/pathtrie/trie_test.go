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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testSetNew[V any](t *testing.T, trie *PathTrie[V], path string, value V) {
	old, found := trie.Set(path, value)
	assert.Falsef(t, found, "Set(%q) of non-existent path should not return an old value", path)
	assert.Emptyf(t, old, "Set(%q) of non-existent path should not return an old value", path)
}

func testSetReplace[V any](t *testing.T, trie *PathTrie[V], path string, value, expectedOld V) {
	old, found := trie.Set(path, value)
	assert.Truef(t, found, "Set(%q) of existing path should return an old value", path)
	assert.Equalf(t, expectedOld, old, "Set(%q) of existing path should return an old value", path)
}

func testGetFail[V any](t *testing.T, trie *PathTrie[V], path string) {
	got, found := trie.Get(path)
	assert.Falsef(t, found, "Get(%q) of non-existent path should not return a value", path)
	assert.Emptyf(t, got, "Get(%q) of non-existent path should not return a value", path)
}

func testGetSucceed[V any](t *testing.T, trie *PathTrie[V], path string, expectedValue V) {
	got, found := trie.Get(path)
	assert.Truef(t, found, "Get(%q) of existing path should return correct value", path)
	assert.Equalf(t, expectedValue, got, "Get(%q) of existing path should return correct value", path)
}

func testDeleteNonExist[V any](t *testing.T, trie *PathTrie[V], path string) {
	got, found := trie.Delete(path)
	assert.Falsef(t, found, "Delete(%q) of non-existent path should not return an old value", path)
	assert.Emptyf(t, got, "Delete(%q) of non-existent path should not return an old value", path)
}

func testDeleteInternal[V any](t *testing.T, trie *PathTrie[V], path string) {
	got, found := trie.Delete(path)
	assert.Falsef(t, found, "Delete(%q) of internal non-value path should not return an old value", path)
	assert.Emptyf(t, got, "Delete(%q) of internal non-value path should not return an old value", path)
}

func testDeleteExist[V any](t *testing.T, trie *PathTrie[V], path string, expectedValue V) {
	got, found := trie.Delete(path)
	assert.Truef(t, found, "Delete(%q) of existing path should return correct value", path)
	assert.Equalf(t, expectedValue, got, "Delete(%q) of existing path should return correct value", path)
}

func testDeleteAllNonExist[V any](t *testing.T, trie *PathTrie[V], path string) {
	got, found := trie.DeleteAll(path)
	assert.Falsef(t, found, "DeleteAll(%q) of non-existent path should not return an old value", path)
	assert.Emptyf(t, got, "DeleteAll(%q) of non-existent path should not return an old value", path)
}

func testDeleteAllInternal[V any](t *testing.T, trie *PathTrie[V], path string) {
	got, found := trie.DeleteAll(path)
	assert.Falsef(t, found, "DeleteAll(%q) of internal non-value path should not return an old value", path)
	assert.Emptyf(t, got, "DeleteAll(%q) of internal non-value path should not return an old value", path)
}

func testDeleteAllExist[V any](t *testing.T, trie *PathTrie[V], path string, expectedValue V) {
	got, found := trie.DeleteAll(path)
	assert.Truef(t, found, "DeleteAll(%q) of existing path should return correct value", path)
	assert.Equalf(t, expectedValue, got, "DeleteAll(%q) of existing path should return correct value", path)
}

func TestGetNonExistent(t *testing.T) {
	trie := NewTrie[string]()

	testGetFail(t, trie, "/a/b/c/d")
	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testGetFail(t, trie, "/a/b/c")
	testGetFail(t, trie, "/a/b/c/dd")
}

func TestSetGetBasic(t *testing.T) {
	trie := NewTrie[string]()

	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testGetSucceed(t, trie, "/a/b/c/d", "abcd")
	// Make sure that leading slashes and other stuff cleaned by filepath.Clean
	// don't impact the results.
	testGetSucceed(t, trie, "a/b/c/d", "abcd")
	testGetSucceed(t, trie, "/../a/b/c/d", "abcd")
	testGetSucceed(t, trie, "a///b/./c/./d/", "abcd")
}

func TestSetMultiple(t *testing.T) {
	trie := NewTrie[string]()

	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testSetReplace(t, trie, "/a/b/c/d", "ABCD", "abcd")
	testSetReplace(t, trie, "/a/b/c/d", "aBcD", "ABCD")
	testGetSucceed(t, trie, "/a/b/c/d", "aBcD")
}

func TestDelete(t *testing.T) {
	trie := NewTrie[string]()

	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testSetNew(t, trie, "/a/b", "ab")
	testSetNew(t, trie, "/a", "a")
	testSetNew(t, trie, "/b/c", "bc")

	testGetSucceed(t, trie, "/a/b/c/d", "abcd")
	testGetSucceed(t, trie, "/a/b", "ab")
	testGetSucceed(t, trie, "/a", "a")
	testGetSucceed(t, trie, "/b/c", "bc")

	t.Run("NonExistent", func(t *testing.T) {
		testDeleteNonExist(t, trie, "/non/exist")

		// Verify that the nodes weren't created.
		node1 := trie.lookupNode(pathToKey("/non"), false)
		assert.Nil(t, node1, "node /non should not have been created")
		node2 := trie.lookupNode(pathToKey("/non/exist"), false)
		assert.Nil(t, node2, "node /non/exist should not have been created")
	})

	t.Run("NoValue", func(t *testing.T) {
		testDeleteInternal(t, trie, "/a/b/c")

		testGetSucceed(t, trie, "/a/b/c/d", "abcd")
		testGetSucceed(t, trie, "/a/b", "ab")
		testGetSucceed(t, trie, "/a", "a")
		testGetSucceed(t, trie, "/b/c", "bc")
	})

	t.Run("Parent", func(t *testing.T) {
		testDeleteExist(t, trie, "/a/b", "ab")

		testGetSucceed(t, trie, "/a/b/c/d", "abcd")
		testGetFail(t, trie, "/a/b")
		testGetSucceed(t, trie, "/a", "a")
		testGetSucceed(t, trie, "/b/c", "bc")
	})

	t.Run("Final", func(t *testing.T) {
		testDeleteExist(t, trie, "/a/b/c/d", "abcd")

		testGetFail(t, trie, "/a/b/c/d")
		testGetFail(t, trie, "/a/b")
		testGetSucceed(t, trie, "/a", "a")
		testGetSucceed(t, trie, "/b/c", "bc")

		// Verify that the final node is actually gone.
		node1 := trie.lookupNode(pathToKey("/a/b/c/d"), false)
		assert.Nilf(t, node1, "node /a/b/c/d should've been pruned -- got %#v", node1)
		// And verify that the no-longer-needed nodes are also gone.
		node2 := trie.lookupNode(pathToKey("/a/b/c"), false)
		assert.Nilf(t, node2, "node /a/b/c should've been pruned -- got %#v", node2)
		node3 := trie.lookupNode(pathToKey("/a/b"), false)
		assert.Nilf(t, node3, "node /a/b should've been pruned -- got %#v", node3)
	})
}

func TestDeletePruning(t *testing.T) {
	trie := NewTrie[string]()
	assert.Zero(t, trie.numChildren, "the trie should be empty")

	testSetNew(t, trie, "/a/b/foo/bar/baz", "baz")
	testSetNew(t, trie, "/a/b/abc/def/xyz", "xyz")
	for n := range 100 {
		testSetNew(t, trie, fmt.Sprintf("/a/dummy/foo/c/%.2d", n), "foobar")
		testSetNew(t, trie, fmt.Sprintf("/a/dummy/bar/d/%.2d", n), "foobar")
	}

	// Make sure the child count is correct.
	root := trie.lookupNode(pathToKey("/"), false)
	require.NotNil(t, root, "node / should exist")
	assert.EqualValues(t, 202, root.numChildren, "unexpected number of children accounted in root")

	t.Run("DeleteAllPrune", func(t *testing.T) {
		testDeleteAllInternal(t, trie, "/a/dummy/foo/c")
		// Check that the root's accounting is updated correctly.
		assert.EqualValues(t, 102, root.numChildren, "unexpected number of children accounted in root after removing 100 children")

		// Make sure that parent directories that are now empty were pruned.
		node1 := trie.lookupNode(pathToKey("/a/dummy/foo/c"), false)
		assert.Nilf(t, node1, "node /a/dummy/foo/c should have been pruned -- got %#v", node1)
		node2 := trie.lookupNode(pathToKey("/a/dummy/foo"), false)
		assert.Nilf(t, node2, "node /a/dummy/foo should have been pruned -- got %#v", node2)
		node3 := trie.lookupNode(pathToKey("/a/dummy"), false)
		require.NotNil(t, node3, "node /a/dummy should not have been pruned")
		assert.EqualValues(t, 100, node3.numChildren, "unpruned /a/dummy should have the right child accounting")

		testDeleteAllInternal(t, trie, "/a/dummy/bar")
		// Check that the root's accounting is updated correctly.
		assert.EqualValues(t, 2, root.numChildren, "unexpected number of children accounted in root after removing 100 children")

		// And now the old shared parent should have been pruned.
		node4 := trie.lookupNode(pathToKey("/a/dummy"), false)
		assert.Nilf(t, node4, "node /a/dummy should have been pruned -- got %#v", node4)
		node5 := trie.lookupNode(pathToKey("/a"), false)
		require.NotNil(t, node5, "node /a should not have been pruned")
		assert.EqualValues(t, 2, node5.numChildren, "unpruned /a should have the right child accounting")
	})

	t.Run("PlainDeletePrune", func(t *testing.T) {
		testDeleteExist(t, trie, "/a/b/foo/bar/baz", "baz")
		// Check that the root's accounting is updated correctly.
		assert.EqualValues(t, 1, root.numChildren, "unexpected number of children accounted in root after removing 1 child")

		// Even clearing a single path should clear all unused parents.
		node1 := trie.lookupNode(pathToKey("/a/b/foo/bar"), false)
		assert.Nilf(t, node1, "node /a/b/foo/bar should have been pruned -- got %#v", node1)
		node2 := trie.lookupNode(pathToKey("/a/b/foo"), false)
		assert.Nilf(t, node2, "node /a/b/foo should have been pruned -- got %#v", node2)
		node3 := trie.lookupNode(pathToKey("/a/b"), false)
		require.NotNil(t, node3, "node /a/b should not have been pruned")
		assert.EqualValues(t, 1, node3.numChildren, "unpruned /a/b should have the right child accounting")
		node4 := trie.lookupNode(pathToKey("/a"), false)
		require.NotNil(t, node4, "node /a/b should not have been pruned")
		assert.EqualValues(t, 1, node4.numChildren, "unpruned /a should have the right child accounting")
	})

	testDeleteAllInternal(t, trie, "/a")
	assert.Zero(t, root.numChildren, "the trie should be empty")

	testSetNew(t, trie, "/z/x/y/w/v/u/t", "t")
	testSetNew(t, trie, "/z/x/y/w/v/u", "u")
	testSetNew(t, trie, "/z/x/y/w/v", "v")
	testSetNew(t, trie, "/z/x/y/w", "w")
	testSetNew(t, trie, "/z/x/y", "y")

	assert.EqualValues(t, 5, root.numChildren, "unexpected number of children accounted after setup")

	t.Run("FilledInternalNodes", func(t *testing.T) {
		// First delete the internal nodes and verify we don't accidentally
		// delete anything more than expected.
		testDeleteExist(t, trie, "/z/x/y/w", "w")
		testGetSucceed(t, trie, "/z/x/y/w/v/u/t", "t")
		testGetSucceed(t, trie, "/z/x/y/w/v/u", "u")
		testGetSucceed(t, trie, "/z/x/y/w/v", "v")
		testGetFail(t, trie, "/z/x/y/w")
		testGetSucceed(t, trie, "/z/x/y", "y")
		assert.EqualValues(t, 4, root.numChildren, "unexpected number of children accounted after removing 1 internal child")

		testDeleteExist(t, trie, "/z/x/y/w/v", "v")
		testGetSucceed(t, trie, "/z/x/y/w/v/u/t", "t")
		testGetSucceed(t, trie, "/z/x/y/w/v/u", "u")
		testGetFail(t, trie, "/z/x/y/w/v")
		testGetFail(t, trie, "/z/x/y/w")
		testGetSucceed(t, trie, "/z/x/y", "y")
		assert.EqualValues(t, 3, root.numChildren, "unexpected number of children accounted after removing 1 internal child")

		testDeleteExist(t, trie, "/z/x/y/w/v/u", "u")
		testGetSucceed(t, trie, "/z/x/y/w/v/u/t", "t")
		testGetFail(t, trie, "/z/x/y/w/v/u")
		testGetFail(t, trie, "/z/x/y/w/v")
		testGetFail(t, trie, "/z/x/y/w")
		testGetSucceed(t, trie, "/z/x/y", "y")
		assert.EqualValues(t, 2, root.numChildren, "unexpected number of children accounted after removing 1 internal child")

		node1 := trie.lookupNode(pathToKey("/z/x/y/w/v/u"), false)
		require.NotNil(t, node1, "node /z/x/y/w/v/u should not have been pruned")
		assert.EqualValues(t, 1, node1.numChildren, "unexpected number of children in internal node with only 1 child")

		// Now delete the terminal node, which should clean up the now-unused
		// parent nodes.
		testDeleteExist(t, trie, "/z/x/y/w/v/u/t", "t")
		testGetFail(t, trie, "/z/x/y/w/v/u/t")
		testGetFail(t, trie, "/z/x/y/w/v/u")
		testGetFail(t, trie, "/z/x/y/w/v")
		testGetFail(t, trie, "/z/x/y/w")
		testGetSucceed(t, trie, "/z/x/y", "y")
		assert.EqualValues(t, 1, root.numChildren, "unexpected number of children accounted after removing 1 terminal child")

		node2 := trie.lookupNode(pathToKey("/z/x/y/w/v/u"), false)
		assert.Nilf(t, node2, "node /z/x/y/w/v/u should have been pruned -- got %#v", node2)
		node3 := trie.lookupNode(pathToKey("/z/x/y/w/v"), false)
		assert.Nilf(t, node3, "node /z/x/y/w/v should have been pruned -- got %#v", node3)
		node4 := trie.lookupNode(pathToKey("/z/x/y/w"), false)
		assert.Nilf(t, node4, "node /z/x/y/w should have been pruned -- got %#v", node4)
		node5 := trie.lookupNode(pathToKey("/z/x/y"), false)
		require.NotNil(t, node5, "node /z/x/y/w/v/u should not have been pruned")
		assert.EqualValues(t, 1, node5.numChildren, "unexpected number of children in internal node with only 1 child")
	})
}

func TestDeleteAll(t *testing.T) {
	trie := NewTrie[string]()

	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testSetNew(t, trie, "/a/b/c/d/e/f", "abcdef")
	testSetNew(t, trie, "/a/b/c/d/e", "abcde")
	testSetNew(t, trie, "/a/b", "ab")
	testSetNew(t, trie, "/a/foo", "afoo")
	testSetNew(t, trie, "/b/c", "bc")

	testGetSucceed(t, trie, "/a/b/c/d/e/f", "abcdef")
	testGetSucceed(t, trie, "/a/b/c/d/e", "abcde")
	testGetSucceed(t, trie, "/a/b/c/d", "abcd")
	testGetSucceed(t, trie, "/a/b", "ab")
	testGetSucceed(t, trie, "/a/foo", "afoo")
	testGetSucceed(t, trie, "/b/c", "bc")

	t.Run("NonExistent", func(t *testing.T) {
		testDeleteAllNonExist(t, trie, "/non/exist")

		// Verify that the nodes weren't created.
		node1 := trie.lookupNode(pathToKey("/non"), false)
		assert.Nilf(t, node1, "node /non should not have been created -- got %#v", node1)
		node2 := trie.lookupNode(pathToKey("/non/exist"), false)
		assert.Nilf(t, node2, "node /non/exist should not have been created -- got %#v", node2)
	})

	t.Run("Final", func(t *testing.T) {
		testDeleteAllExist(t, trie, "/a/b/c/d/e/f", "abcdef")

		testGetFail(t, trie, "/a/b/c/d/e/f")
		testGetSucceed(t, trie, "/a/b/c/d/e", "abcde")
		testGetSucceed(t, trie, "/a/b/c/d", "abcd")
		testGetSucceed(t, trie, "/a/b", "ab")
		testGetSucceed(t, trie, "/a/foo", "afoo")
		testGetSucceed(t, trie, "/b/c", "bc")

		// Verify that the final node is actually gone.
		node := trie.lookupNode(pathToKey("/a/b/c/d/e/f"), false)
		assert.Nilf(t, node, "node /a/b/c/d/e/f should've been pruned -- got %#v", node)
	})

	t.Run("Parent", func(t *testing.T) {
		testDeleteAllExist(t, trie, "/a/b/c/d", "abcd")

		testGetFail(t, trie, "/a/b/c/d/e/f")
		testGetFail(t, trie, "/a/b/c/d/e")
		testGetFail(t, trie, "/a/b/c/d")
		testGetSucceed(t, trie, "/a/b", "ab")
		testGetSucceed(t, trie, "/a/foo", "afoo")
		testGetSucceed(t, trie, "/b/c", "bc")

		// Verify that the intermediate nodes are actually gone.
		node1 := trie.lookupNode(pathToKey("/a/b/c/d/e"), false)
		assert.Nilf(t, node1, "node /a/b/c/d/e should've been pruned -- got %#v", node1)
		node2 := trie.lookupNode(pathToKey("/a/b/c/d"), false)
		assert.Nilf(t, node2, "node /a/b/c/d should've been pruned -- got %#v", node2)
	})

	t.Run("NoValue", func(t *testing.T) {
		testDeleteAllInternal(t, trie, "/a")

		testGetFail(t, trie, "/a/b/c/d/e/f")
		testGetFail(t, trie, "/a/b/c/d/e")
		testGetFail(t, trie, "/a/b/c/d")
		testGetFail(t, trie, "/a/b/c")
		testGetFail(t, trie, "/a/b")
		testGetFail(t, trie, "/a/foo")
		testGetFail(t, trie, "/a")
		testGetSucceed(t, trie, "/b/c", "bc")

		// Verify that the intermediate nodes are actually gone.
		node1 := trie.lookupNode(pathToKey("/a/b/c"), false)
		assert.Nilf(t, node1, "node /a/b/c should've been pruned -- got %#v", node1)
		node2 := trie.lookupNode(pathToKey("/a/b"), false)
		assert.Nilf(t, node2, "node /a/b should've been pruned -- got %#v", node2)
		node3 := trie.lookupNode(pathToKey("/a/foo"), false)
		assert.Nilf(t, node3, "node /a/foo should've been pruned -- got %#v", node3)
		node4 := trie.lookupNode(pathToKey("/a"), false)
		assert.Nilf(t, node4, "node /a should've been pruned -- got %#v", node4)
	})

	testGetSucceed(t, trie, "/b/c", "bc")
}

func TestWalk(t *testing.T) {
	trie := NewTrie[string]()

	testSetNew(t, trie, "/a/b/c/d", "abcd")
	testSetNew(t, trie, "/a/b/c/d/e/f", "abcdef")
	testSetNew(t, trie, "/a/b/c/d/e/ff", "abcdeff")
	testSetNew(t, trie, "/a/b/c/d/e", "abcde")
	testSetNew(t, trie, "/a/b/cc", "abcc")
	testSetNew(t, trie, "/a/b/cc/dd/ee/ff", "abccddeeff")
	testSetNew(t, trie, "/a/b", "ab")
	testSetNew(t, trie, "/a/foo", "afoo")
	testSetNew(t, trie, "/a/foo/bar/baz", "afoobarbaz")
	testSetNew(t, trie, "/a/boop", "aboop")
	testSetNew(t, trie, "/b/c", "bc")
	testSetNew(t, trie, "/e/f", "ef")
	testSetNew(t, trie, "/baz", "baz")

	type foundEntry struct {
		path, value string
	}

	t.Run("EmptyRoot", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{"/a/b/c/d", "abcd"},
			{"/a/b/c/d/e/f", "abcdef"},
			{"/a/b/c/d/e/ff", "abcdeff"},
			{"/a/b/c/d/e", "abcde"},
			{"/a/b/cc", "abcc"},
			{"/a/b/cc/dd/ee/ff", "abccddeeff"},
			{"/a/b", "ab"},
			{"/a/foo", "afoo"},
			{"/a/foo/bar/baz", "afoobarbaz"},
			{"/a/boop", "aboop"},
			{"/b/c", "bc"},
			{"/e/f", "ef"},
			{"/baz", "baz"},
		})
	})

	t.Run("DeepSubPath", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/a/b", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{"/a/b/c/d", "abcd"},
			{"/a/b/c/d/e/f", "abcdef"},
			{"/a/b/c/d/e/ff", "abcdeff"},
			{"/a/b/c/d/e", "abcde"},
			{"/a/b/cc", "abcc"},
			{"/a/b/cc/dd/ee/ff", "abccddeeff"},
			{"/a/b", "ab"},
		})
	})

	t.Run("ShallowSubPath", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/b", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{"/b/c", "bc"},
		})
	})

	t.Run("SingleEntry", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/a/b/c/d/e/ff", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{"/a/b/c/d/e/ff", "abcdeff"},
		})
	})

	t.Run("NonExistent", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/non/exist", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.Empty(t, sawEntries, "walk trie in non-existent path should not see anything")
	})

	testSetNew(t, trie, "/", "i am root")

	t.Run("Root-Abs", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("/", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{"/", "i am root"},
			{"/a/b/c/d", "abcd"},
			{"/a/b/c/d/e/f", "abcdef"},
			{"/a/b/c/d/e/ff", "abcdeff"},
			{"/a/b/c/d/e", "abcde"},
			{"/a/b/cc", "abcc"},
			{"/a/b/cc/dd/ee/ff", "abccddeeff"},
			{"/a/b", "ab"},
			{"/a/foo", "afoo"},
			{"/a/foo/bar/baz", "afoobarbaz"},
			{"/a/boop", "aboop"},
			{"/b/c", "bc"},
			{"/e/f", "ef"},
			{"/baz", "baz"},
		})
	})

	t.Run("Root-Dot", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom(".", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{".", "i am root"},
			{"a/b/c/d", "abcd"},
			{"a/b/c/d/e/f", "abcdef"},
			{"a/b/c/d/e/ff", "abcdeff"},
			{"a/b/c/d/e", "abcde"},
			{"a/b/cc", "abcc"},
			{"a/b/cc/dd/ee/ff", "abccddeeff"},
			{"a/b", "ab"},
			{"a/foo", "afoo"},
			{"a/foo/bar/baz", "afoobarbaz"},
			{"a/boop", "aboop"},
			{"b/c", "bc"},
			{"e/f", "ef"},
			{"baz", "baz"},
		})
	})

	t.Run("Root-EmptyString", func(t *testing.T) {
		var sawEntries []foundEntry
		err := trie.WalkFrom("", func(path, value string) error {
			sawEntries = append(sawEntries, foundEntry{
				path:  path,
				value: value,
			})
			return nil
		})
		require.NoError(t, err, "walk trie")
		assert.ElementsMatch(t, sawEntries, []foundEntry{
			{".", "i am root"},
			{"a/b/c/d", "abcd"},
			{"a/b/c/d/e/f", "abcdef"},
			{"a/b/c/d/e/ff", "abcdeff"},
			{"a/b/c/d/e", "abcde"},
			{"a/b/cc", "abcc"},
			{"a/b/cc/dd/ee/ff", "abccddeeff"},
			{"a/b", "ab"},
			{"a/foo", "afoo"},
			{"a/foo/bar/baz", "afoobarbaz"},
			{"a/boop", "aboop"},
			{"b/c", "bc"},
			{"e/f", "ef"},
			{"baz", "baz"},
		})
	})

	t.Run("Error", func(t *testing.T) {
		testErr := fmt.Errorf("test error")
		err := trie.WalkFrom("/", func(path, value string) error { //nolint:revive // unused-parameter doesn't make sense for this test
			if path == "/a/b/cc" {
				return testErr
			}
			return nil
		})
		require.ErrorIs(t, err, testErr, "walk trie")
	})
}
