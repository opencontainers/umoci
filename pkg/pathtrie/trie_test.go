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
	})

	// TODO: If we switch to making Delete clear trash more aggressively, we
	// should test that here.
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
		err := trie.WalkFrom("/", func(path, value string) error {
			if path == "/a/b/cc" {
				return testErr
			}
			return nil
		})
		require.ErrorIs(t, err, testErr, "walk trie")
	})
}
