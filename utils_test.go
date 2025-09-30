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

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
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
		{"QuotedValues", "Funky Values", []string{"a b", "c\nd", "e\t\bf", "foobar-baz"}, `Funky Values: "a b", "c\nd", "e\t\bf", foobar-baz` + "\n"},
		{"WhitespaceValue", "whitespace value", []string{"a b"}, `whitespace value: "a b"` + "\n"},
		{"WhitespaceValues", "multiple whitespace values", []string{"a b", "foo\u00a0bar", "abc\u202fdef"}, `multiple whitespace values: "a b", "foo\u00a0bar", "abc\u202fdef"` + "\n"},
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
		{"Nil", "\t", "Nil Values", nil, "\tNil Values: (empty)\n"},
		{"Empty", "\t\t", "Empty Values", []string{}, "\t\tEmpty Values: (empty)\n"},
		{"Basic", "\t\t\t", "Basic Values", []string{"a", "b", "cdef"}, "\t\t\t" + `Basic Values:
				a
				b
				cdef
`},
		{"Whitespace", "\t\t\t", "Whitespace Values", []string{" a b c   ", "foo  bar\n"}, "\t\t\t" + `Whitespace Values:
				" a b c   "
				"foo  bar\n"
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
		{"Nil", "\t", "Nil Map", nil, "\tNil Map: (empty)\n"},
		{"Empty", "\t\t", "Empty Map", map[string]string{}, "\t\tEmpty Map: (empty)\n"},
		{"Basic", "\t\t\t", "Basic Map", map[string]string{"a": "b", "c.e.f": "foobar123", "01": "2345"}, "\t\t\t" + `Basic Map:
				01: 2345
				a: b
				c.e.f: foobar123
`},
		{"Whitespace", "\t\t\t", "Whitespace Values", map[string]string{" a b c  ": "cdef", "foo": "foo  bar\n", "a b": "c d"}, "\t\t\t" + `Whitespace Values:
				" a b c  ": cdef
				"a b": "c d"
				foo: "foo  bar\n"
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
		{"Nil", "\t", "Nil Set", nil, "\tNil Set: (empty)\n"},
		{"Empty", "\t\t", "Empty Set", set[string]{}, "\t\tEmpty Set: (empty)\n"},
		{"Basic", "\t\t\t", "Basic Set", mkset("a", "b", "c.e.f", "foobar123", "01", "2345"), "\t\t\tBasic Set: 01, 2345, a, b, c.e.f, foobar123\n"},
		{"Whitespace", "\t\t\t", "Whitespace Set", mkset(" a b c  ", "foo bar\n", "c\tef", "foo\u00a0bar123"), "\t\t\t" + `Whitespace Set: " a b c  ", "c\tef", "foo bar\n", "foo\u00a0bar123"` + "\n"},
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

func TestPprintPlatform(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefix   string
		platform ispec.Platform
		expected string
	}{
		{"Basic", "\t\t", ispec.Platform{
			OS:           "linux",
			Architecture: "amd64",
		}, "\t\t" + `Platform:
			OS: linux
			Architecture: amd64
`},
		{"ArchVariant", "\t\t", ispec.Platform{
			OS:           "freebsd",
			Architecture: "arm64",
			Variant:      "v7",
		}, "\t\t" + `Platform:
			OS: freebsd
			Architecture: arm64 (v7)
`},
		{"OSFeatures", "\t\t", ispec.Platform{
			OS:           "windows",
			OSVersion:    "10.0.14393.1066",
			OSFeatures:   []string{"win32k", "win-dummy"},
			Architecture: "amd64",
		}, "\t\t" + `Platform:
			OS: windows
			OS Version: 10.0.14393.1066
			OS Features: win32k, win-dummy
			Architecture: amd64
`},
		{"Whitespace", "\t\t", ispec.Platform{
			OS:           "fake os",
			OSVersion:    "a b c",
			OSFeatures:   []string{"a,b", "c d e f"},
			Architecture: "arm is cool",
			Variant:      "version 7",
		}, "\t\t" + `Platform:
			OS: "fake os"
			OS Version: "a b c"
			OS Features: "a,b", "c d e f"
			Architecture: "arm is cool" (version 7)
`},
		{"Quoted", "\t\t", ispec.Platform{
			OS:           "fake\tos\n",
			OSVersion:    "a\vbc",
			OSFeatures:   []string{"a\tb", "c\u00a0bar"},
			Architecture: "arm\nis\ncool",
			Variant:      "version\b7",
		}, "\t\t" + `Platform:
			OS: "fake\tos\n"
			OS Version: "a\vbc"
			OS Features: "a\tb", "c\u00a0bar"
			Architecture: "arm\nis\ncool" ("version\b7")
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintPlatform(&output, test.prefix, test.platform)
			require.NoErrorf(t, err, "pprint(%q, %#v)", test.prefix, test.platform)
			assert.Equal(t, test.expected, output.String())
		})
	}
}

func TestPprintDescriptor(t *testing.T) {
	for _, test := range []struct {
		name       string
		prefix     string
		descriptor ispec.Descriptor
		expected   string
	}{
		{"Basic", "\t\t", ispec.Descriptor{
			MediaType: "image/png",
			Digest:    "sha256:7d30ef97fe167eb78d5c52503096522ebba7ba95789ba735fd68da9d4838f84d",
			Size:      61235,
		}, "\t\t" + `Descriptor:
			Media Type: image/png
			Digest: sha256:7d30ef97fe167eb78d5c52503096522ebba7ba95789ba735fd68da9d4838f84d
			Size: 61.23kB
`},
		{"Annotations", "\t\t", ispec.Descriptor{
			MediaType: ispec.MediaTypeImageManifest,
			Digest:    "blake2:acbe05bb922855b8b2fea560e11ad2d135505a9f194a0fa99f1c57d3974990f6853ce22e97273d8dbf7424c09fe43e53ce4080dcc543482be46965bb24639cbe",
			Size:      59129319,
			Annotations: map[string]string{
				"com.example.foo": "abc123",
				"ci.umo.foobar":   "test",
				"com.cyphar.test": "https://www.cyphar.com/",
			},
		}, "\t\t" + `Descriptor:
			Media Type: application/vnd.oci.image.manifest.v1+json
			Digest: blake2:acbe05bb922855b8b2fea560e11ad2d135505a9f194a0fa99f1c57d3974990f6853ce22e97273d8dbf7424c09fe43e53ce4080dcc543482be46965bb24639cbe
			Size: 59.13MB
			Annotations:
				ci.umo.foobar: test
				com.cyphar.test: https://www.cyphar.com/
				com.example.foo: abc123
`},
		{"URLs", "\t\t", ispec.Descriptor{
			MediaType: ispec.MediaTypeImageLayerGzip,
			Digest:    "sha512:dd65b6a8d277c7b06003178fc7a125fa6ad021dff2beffd82df0b469f2afe735312e2932608d42d1bc8421141edb0882680e36d45f5d804783ef555a3566a9f8",
			Size:      926812731,
			URLs: []string{
				"https://www.example.com/example.txt",
				"https://www.cyphar.com/",
			},
		}, "\t\t" + `Descriptor:
			Media Type: application/vnd.oci.image.layer.v1.tar+gzip
			Digest: sha512:dd65b6a8d277c7b06003178fc7a125fa6ad021dff2beffd82df0b469f2afe735312e2932608d42d1bc8421141edb0882680e36d45f5d804783ef555a3566a9f8
			Size: 926.8MB
			URLs:
				https://www.example.com/example.txt
				https://www.cyphar.com/
`},
		{"Whitespace", "\t\t", ispec.Descriptor{
			MediaType: "application/x-dummy; foo",
			Digest:    "dummy: a b c d",
			Size:      123,
			URLs: []string{
				"https:// www. example.com/example.txt",
				"https:// www. cyphar.com/",
			},
			Annotations: map[string]string{
				"com. example.foo": "abc123",
				"ci.umo.foobar":    "test foo",
				"com cyphar test":  "https   www cyphar com ",
			},
		}, "\t\t" + `Descriptor:
			Media Type: "application/x-dummy; foo"
			Digest: "dummy: a b c d"
			Size: 123B
			URLs:
				"https:// www. example.com/example.txt"
				"https:// www. cyphar.com/"
			Annotations:
				ci.umo.foobar: "test foo"
				"com cyphar test": "https   www cyphar com "
				"com. example.foo": abc123
`},
		{"Quoted", "\t\t", ispec.Descriptor{
			MediaType: "application/x-dummy;\nfoo",
			Digest:    "dummy:a\bb\bc\td",
			Size:      123,
			URLs: []string{
				"\t\n",
			},
			Annotations: map[string]string{
				"com.\texample.foo": "abc123",
				"ci.umo.foobar":     "test\nfoo",
				"com cyphar test":   "https \t www cyphar com ",
			},
		}, "\t\t" + `Descriptor:
			Media Type: "application/x-dummy;\nfoo"
			Digest: "dummy:a\bb\bc\td"
			Size: 123B
			URLs:
				"\t\n"
			Annotations:
				ci.umo.foobar: "test\nfoo"
				"com cyphar test": "https \t www cyphar com "
				"com.\texample.foo": abc123
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintDescriptor(&output, test.prefix, test.descriptor)
			require.NoErrorf(t, err, "pprint(%q, %#v)", test.prefix, test.descriptor)
			assert.Equal(t, test.expected, output.String())
		})
	}
}
