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

package umoci

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPprint(t *testing.T) {
	for _, test := range []struct {
		name     string
		key      string
		values   []string
		expected string
	}{
		{"NoValue", "KeyA", nil, "KeyA:\n"},
		{"OneValue", "KeyB", []string{"foobar"}, "KeyB: foobar\n"},
		{"MultipleValues", "Multiple Values", []string{"a", "b", "c", "d", "e"}, "Multiple Values: a, b, c, d, e\n"},
		{"CommaValue", "Has a comma", []string{"a, b, c, d, e"}, `Has a comma: "a, b, c, d, e"` + "\n"},
		{"CommaValues", "Has commas", []string{"a,b", "c,d,e"}, `Has commas: "a,b", "c,d,e"` + "\n"},
		{"QuotedValue", "Funky value", []string{"foo bar\tba\\z"}, `Funky value: "foo bar\tba\\z"` + "\n"},
		{"QuotedValues", "Funky Values", []string{"a b", "c\nd", "e\t\bf", "foobar-baz"}, `Funky Values: a b, "c\nd", "e\t\bf", foobar-baz` + "\n"},
	} {
		for _, prefix := range []struct {
			name, prefix string
		}{
			{"-NoPrefix", ""},
			{"-PrefixTab", "\t"},
			{"-PrefixTabs", "\t\t"},
			{"-PrefixOther", "...\t"},
		} {
			t.Run(test.name+prefix.name, func(t *testing.T) {
				var output bytes.Buffer
				err := pprint(&output, prefix.prefix, test.key, test.values...)
				require.NoErrorf(t, err, "pprint(%q, %q, %v...)", prefix.prefix, test.key, test.values)

				expected := prefix.prefix + test.expected
				assert.Equal(t, expected, output.String())
			})
		}
	}
}

func TestPprintSlice(t *testing.T) {
	for _, test := range []struct {
		name        string
		prefix, key string
		data        []string
		expected    string
	}{
		{"Nil", "\t", "Nil Values", nil, "\tNil Values:\n"},
		{"Empty", "\t\t", "Empty Values", []string{}, "\t\tEmpty Values:\n"},
		{"Basic", "\t\t\t", "Basic Values", []string{"a", "b", "cdef"}, "\t\t\t" + `Basic Values:
				a
				b
				cdef
`},
		{"Quoted", "\t\t\t", "Quoted Values", []string{"ab", "c\tdef", "foo-bar\n"}, "\t\t\t" + `Quoted Values:
				ab
				"c\tdef"
				"foo-bar\n"
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintSlice(&output, test.prefix, test.key, test.data)
			require.NoErrorf(t, err, "pprint(%q, %q, %v)", test.prefix, test.key, test.data)
			assert.Equal(t, test.expected, output.String())
		})
	}
}

func TestPprintMap(t *testing.T) {
	for _, test := range []struct {
		name        string
		prefix, key string
		data        map[string]string
		expected    string
	}{
		{"Nil", "\t", "Nil Map", nil, "\tNil Map:\n"},
		{"Empty", "\t\t", "Empty Map", map[string]string{}, "\t\tEmpty Map:\n"},
		{"Basic", "\t\t\t", "Basic Map", map[string]string{"a": "b", "c.e.f": "foobar123", "01": "2345"}, "\t\t\t" + `Basic Map:
				01: 2345
				a: b
				c.e.f: foobar123
`},
		{"Quoted", "\t\t\t", "Quoted Map", map[string]string{"a\tb": "cdef", "ab": "c\tdef", "foo": "bar\n", "c\tef": "foo\u00a0bar123"}, "\t\t\t" + `Quoted Map:
				"a\tb": cdef
				ab: "c\tdef"
				"c\tef": "foo\u00a0bar123"
				foo: "bar\n"
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintMap(&output, test.prefix, test.key, test.data)
			require.NoErrorf(t, err, "pprint(%q, %q, %v)", test.prefix, test.key, test.data)
			assert.Equal(t, test.expected, output.String())
		})
	}
}

type set[T comparable] = map[T]struct{}

func mkset[T comparable](ks ...T) set[T] {
	set := make(set[T], len(ks))
	for _, k := range ks {
		set[k] = struct{}{}
	}
	return set
}

func TestPprintSet(t *testing.T) {
	for _, test := range []struct {
		name        string
		prefix, key string
		data        set[string]
		expected    string
	}{
		{"Nil", "\t", "Nil Set", nil, "\tNil Set:\n"},
		{"Empty", "\t\t", "Empty Set", set[string]{}, "\t\tEmpty Set:\n"},
		{"Basic", "\t\t\t", "Basic Set", mkset("a", "b", "c.e.f", "foobar123", "01", "2345"), "\t\t\tBasic Set: 01, 2345, a, b, c.e.f, foobar123\n"},
		{"Quoted", "\t\t\t", "Quoted Set", mkset("a\tb", "cdef", "foo", "bar\n", "c\tef", "foo\u00a0bar123"), "\t\t\t" + `Quoted Set: "a\tb", "bar\n", "c\tef", cdef, foo, "foo\u00a0bar123"` + "\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintSet(&output, test.prefix, test.key, test.data)
			require.NoErrorf(t, err, "pprint(%q, %q, %v)", test.prefix, test.key, test.data)
			assert.Equal(t, test.expected, output.String())
		})
	}
}
