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
	"time"

	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
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
		{"Empty", "\t\t", ispec.Descriptor{}, "\t\t" + `Descriptor:
			Media Type: ""
			Digest: ""
			Size: 0B
`},
		// Adapted from <https://github.com/opencontainers/image-spec/blob/v1.1.1/descriptor.md#examples>.
		{"SpecExample-1", "\t\t", ispec.Descriptor{
			MediaType: ispec.MediaTypeImageManifest,
			Digest:    "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
			Size:      7682,
		}, "\t\t" + `Descriptor:
			Media Type: application/vnd.oci.image.manifest.v1+json
			Digest: sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270
			Size: 7.682kB
`},
		{"SpecExample-2", "\t\t", ispec.Descriptor{
			MediaType: ispec.MediaTypeImageManifest,
			Digest:    "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
			Size:      7682,
			URLs: []string{
				"https://example.com/example-manifest",
			},
		}, "\t\t" + `Descriptor:
			Media Type: application/vnd.oci.image.manifest.v1+json
			Digest: sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270
			Size: 7.682kB
			URLs:
				https://example.com/example-manifest
`},
		// TODO: Support pretty-printing v1.1.1 fields.
		{"SpecExample-3", "\t\t", ispec.Descriptor{
			MediaType:    "",
			ArtifactType: "application/vnd.example.sbom.v1",
			Digest:       "sha256:87923725d74f4bfb94c9e86d64170f7521aad8221a5de834851470ca142da630",
			Size:         123,
		}, "\t\t" + `Descriptor:
			Media Type: ""
			Digest: sha256:87923725d74f4bfb94c9e86d64170f7521aad8221a5de834851470ca142da630
			Size: 123B
`},
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

func TestPprintImageConfig(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefix   string
		config   ispec.ImageConfig
		expected string
	}{
		{"Empty", "\t\t", ispec.ImageConfig{}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
`},
		{"User", "\t\t", ispec.ImageConfig{
			User: "foo:bar",
		}, "\t\t" + `Image Config:
			User: foo:bar
			Command: (empty)
`},
		{"Command", "\t\t", ispec.ImageConfig{
			User: "0",
			Cmd:  []string{"/bin/bash", "-c", "while true; sleep 1s; done;"},
		}, "\t\t" + `Image Config:
			User: 0
			Command:
				/bin/bash
				-c
				"while true; sleep 1s; done;"
`},
		{"Entrypoint", "\t\t", ispec.ImageConfig{
			User:       "0",
			Entrypoint: []string{"/bin/bash", "-c"},
		}, "\t\t" + `Image Config:
			User: 0
			Entrypoint:
				/bin/bash
				-c
			Command: (empty)
`},
		{"Entrypoint+Command", "\t\t", ispec.ImageConfig{
			User:       "0",
			Entrypoint: []string{"/bin/bash", "-c"},
			Cmd:        []string{"while true; sleep 1s; done;"},
		}, "\t\t" + `Image Config:
			User: 0
			Entrypoint:
				/bin/bash
				-c
			Command:
				"while true; sleep 1s; done;"
`},
		{"Environment", "\t\t", ispec.ImageConfig{
			Env: []string{
				"PATH=/bin:/usr/bin",
				"HOME=/home/cyphar",
				"FOOBAR=The quick brown fox jumps over the lazy dog.",
				"EQUAL==",
			},
		}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
			Environment:
				PATH=/bin:/usr/bin
				HOME=/home/cyphar
				"FOOBAR=The quick brown fox jumps over the lazy dog."
				EQUAL==
`},
		{"StopSignal", "\t\t", ispec.ImageConfig{
			StopSignal: "SIGKILL",
		}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
			Stop Signal: SIGKILL
`},
		{"ExposedPorts", "\t\t", ispec.ImageConfig{
			ExposedPorts: mkset("80/tcp", "50000/udp"),
		}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
			Exposed Ports: 50000/udp, 80/tcp
`},
		{"Volumes", "\t\t", ispec.ImageConfig{
			Volumes: mkset("/foo/bar", "/baz", "/beep boop"),
		}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
			Volumes: /baz, "/beep boop", /foo/bar
`},
		{"Labels", "\t\t", ispec.ImageConfig{
			Labels: map[string]string{
				"foo bar": "baz",
			},
		}, "\t\t" + `Image Config:
			User: ""
			Command: (empty)
			Labels:
				"foo bar": baz
`},
		{"Basic", "\t\t", ispec.ImageConfig{
			Cmd:        []string{"bash"},
			WorkingDir: "/go",
			Env: []string{
				"PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"GOLANG_VERSION=1.25.1",
				"GOTOOLCHAIN=local",
				"GOPATH=/go",
			},
		}, "\t\t" + `Image Config:
			User: ""
			Command:
				bash
			Working Directory: /go
			Environment:
				PATH=/go/bin:/usr/local/go/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
				GOLANG_VERSION=1.25.1
				GOTOOLCHAIN=local
				GOPATH=/go
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintImageConfig(&output, test.prefix, test.config)
			require.NoErrorf(t, err, "pprint(%q, %#v)", test.prefix, test.config)
			assert.Equal(t, test.expected, output.String())
		})
	}
}

func mktime(sec, nsec int64) *time.Time {
	time := time.Unix(sec, nsec).UTC()
	return &time
}

func TestPprintImage(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefix   string
		image    ispec.Image
		expected string
	}{
		{"Empty", "\t\t", ispec.Image{}, "\t\t" + `Author: ""
		Platform:
			OS: ""
			Architecture: ""
		Image Config:
			User: ""
			Command: (empty)
`},
		// Adapted from <https://github.com/opencontainers/image-spec/blob/v1.1.1/config.md#example>.
		{"SpecExample", "\t\t", ispec.Image{
			Created: mktime(1446330176, 15925234),
			Author:  "Alyssa P. Hacker <alyspdev@example.com>",
			Platform: ispec.Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			Config: ispec.ImageConfig{
				User:       "alice",
				Entrypoint: []string{"/bin/my-app-binary"},
				Cmd:        []string{"--foreground", "--config", "/etc/my-app.d/default.cfg"},
				WorkingDir: "/home/alice",
				Env: []string{
					"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
					"FOO=oci_is_a",
					"BAR=well_written_spec",
				},
				ExposedPorts: mkset("8080/tcp"),
				Volumes:      mkset("/var/job-result-data", "/var/log/my-app-logs"),
				Labels: map[string]string{
					"com.example.project.git.url":    "https://example.com/project.git",
					"com.example.project.git.commit": "45a939b2999782a3f005621a8d0f29aa387e1d6b",
				},
			},
			// Not printed by pprintImage, as this is handled by the history
			// pretty-printer.
			RootFS: ispec.RootFS{
				Type: "layers",
				DiffIDs: []digest.Digest{
					"sha256:c6f988f4874bb0add23a778f753c65efe992244e148a1d2ec2a8b664fb66bbd1",
					"sha256:5f70bf18a086007016e948b04aed3b82103a36bea41755b6cddfaf10ace3c6ef",
				},
			},
			History: []ispec.History{
				{
					Created:   mktime(1446330174, 690851953),
					CreatedBy: "/bin/sh -c #(nop) ADD file:a3bc1e842b69636f9df5256c49c5374fb4eef1e281fe3f282c65fb853ee171c5 in /",
				},
				{
					Created:    mktime(1446330175, 613815829),
					CreatedBy:  "/bin/sh -c #(nop) CMD [\"sh\"]",
					EmptyLayer: true,
				},
				{
					Created:   mktime(1446330176, 329850019),
					CreatedBy: "/bin/sh -c apk add curl",
				},
			},
		}, "\t\t" + `Created: 2015-10-31T22:22:56.015925234Z
		Author: "Alyssa P. Hacker <alyspdev@example.com>"
		Platform:
			OS: linux
			Architecture: amd64
		Image Config:
			User: alice
			Entrypoint:
				/bin/my-app-binary
			Command:
				--foreground
				--config
				/etc/my-app.d/default.cfg
			Working Directory: /home/alice
			Environment:
				PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin
				FOO=oci_is_a
				BAR=well_written_spec
			Exposed Ports: 8080/tcp
			Volumes: /var/job-result-data, /var/log/my-app-logs
			Labels:
				com.example.project.git.commit: 45a939b2999782a3f005621a8d0f29aa387e1d6b
				com.example.project.git.url: https://example.com/project.git
`},
		{"Basic", "\t\t", ispec.Image{
			Created: mktime(1757077500, 0),
			Author:  "Aleksa Sarai <cyphar@cyphar.com>",
			Platform: ispec.Platform{
				OS:           "linux",
				Architecture: "amd64",
			},
			Config: ispec.ImageConfig{
				User:         "root",
				Entrypoint:   []string{"/bin/bash", "-c"},
				Cmd:          []string{"shutdown -h"},
				WorkingDir:   "/tmp",
				Env:          []string{"PATH=/bin:/usr/bin", "HOME=/"},
				StopSignal:   "SIGSTOP",
				ExposedPorts: mkset("12345/tcp"),
				Volumes:      mkset("/tmp", "/opt/foo/bar"),
				Labels: map[string]string{
					"org.opencontainers.image.url": "https://www.cyphar.com/",
					"org.label-schema.descripton":  "foo bar baz",
				},
			},
		}, "\t\t" + `Created: 2025-09-05T13:05:00Z
		Author: "Aleksa Sarai <cyphar@cyphar.com>"
		Platform:
			OS: linux
			Architecture: amd64
		Image Config:
			User: root
			Entrypoint:
				/bin/bash
				-c
			Command:
				"shutdown -h"
			Working Directory: /tmp
			Environment:
				PATH=/bin:/usr/bin
				HOME=/
			Stop Signal: SIGSTOP
			Exposed Ports: 12345/tcp
			Volumes: /opt/foo/bar, /tmp
			Labels:
				org.label-schema.descripton: "foo bar baz"
				org.opencontainers.image.url: https://www.cyphar.com/
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintImage(&output, test.prefix, test.image)
			require.NoErrorf(t, err, "pprint(%q, %#v)", test.prefix, test.image)
			assert.Equal(t, test.expected, output.String())
		})
	}
}

func TestPprintManifest(t *testing.T) {
	for _, test := range []struct {
		name     string
		prefix   string
		manifest ispec.Manifest
		expected string
	}{
		{"Empty", "\t\t", ispec.Manifest{}, "\t\t" + `Schema Version: 0
		Media Type: ""
		Config:
			Descriptor:
				Media Type: ""
				Digest: ""
				Size: 0B
`},
		// Adapted from <https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md#example-image-manifest>.
		// TODO: Support pretty-printing v1.1.1 fields.
		{"SpecExample", "\t\t\t", ispec.Manifest{
			Versioned: imeta.Versioned{SchemaVersion: 2},
			MediaType: ispec.MediaTypeImageManifest,
			Config: ispec.Descriptor{
				MediaType: ispec.MediaTypeImageConfig,
				Digest:    "sha256:b5b2b2c507a0944348e0303114d8d93aaaa081732b86451d9bce1f432a537bc7",
				Size:      7023,
			},
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:9834876dcfb05cb167a5c24953eba58c4ac89b1adf57f28f2f9d09af107ee8f0",
					Size:      32654,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b",
					Size:      16724,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736",
					Size:      73109,
				},
			},
			Subject: &ispec.Descriptor{
				MediaType: ispec.MediaTypeImageManifest,
				Digest:    "sha256:5b0bcabd1ed22e9fb1310cf6c2dec7cdef19f0ad69efa1f392e94a4333501270",
				Size:      7682,
			},
			Annotations: map[string]string{
				"com.example.key1": "value1",
				"com.example.key2": "value2",
			},
		}, "\t\t\t" + `Schema Version: 2
			Media Type: application/vnd.oci.image.manifest.v1+json
			Config:
				Descriptor:
					Media Type: application/vnd.oci.image.config.v1+json
					Digest: sha256:b5b2b2c507a0944348e0303114d8d93aaaa081732b86451d9bce1f432a537bc7
					Size: 7.023kB
			Layers:
				Descriptor:
					Media Type: application/vnd.oci.image.layer.v1.tar+gzip
					Digest: sha256:9834876dcfb05cb167a5c24953eba58c4ac89b1adf57f28f2f9d09af107ee8f0
					Size: 32.65kB
				Descriptor:
					Media Type: application/vnd.oci.image.layer.v1.tar+gzip
					Digest: sha256:3c3a4604a545cdc127456d94e421cd355bca5b528f4a9c1905b15da2eb4a4c6b
					Size: 16.72kB
				Descriptor:
					Media Type: application/vnd.oci.image.layer.v1.tar+gzip
					Digest: sha256:ec4b8955958665577945c89419d1af06b5f7636b4ac3da7f12184802ad867736
					Size: 73.11kB
			Annotations:
				com.example.key1: value1
				com.example.key2: value2
`},
		// Adapted from <https://github.com/opencontainers/image-spec/blob/v1.1.1/manifest.md#guidelines-for-artifact-usage>.
		// TODO: Support pretty-printing v1.1.1 fields.
		{"ArtifactExample-1", "\t\t\t", ispec.Manifest{
			Versioned:    imeta.Versioned{SchemaVersion: 2},
			MediaType:    ispec.MediaTypeImageManifest,
			ArtifactType: "application/vnd.example+type",
			Config:       ispec.DescriptorEmptyJSON,
			Layers:       []ispec.Descriptor{ispec.DescriptorEmptyJSON},
			Annotations: map[string]string{
				"oci.opencontainers.image.created": "2023-01-02T03:04:05Z",
				"com.example.data":                 "payload",
			},
		}, "\t\t\t" + `Schema Version: 2
			Media Type: application/vnd.oci.image.manifest.v1+json
			Config:
				Descriptor:
					Media Type: application/vnd.oci.empty.v1+json
					Digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
					Size: 2B
			Layers:
				Descriptor:
					Media Type: application/vnd.oci.empty.v1+json
					Digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
					Size: 2B
			Annotations:
				com.example.data: payload
				oci.opencontainers.image.created: 2023-01-02T03:04:05Z
`},
		{"ArtifactExample-2", "\t\t\t", ispec.Manifest{
			Versioned:    imeta.Versioned{SchemaVersion: 2},
			MediaType:    ispec.MediaTypeImageManifest,
			ArtifactType: "application/vnd.example+type",
			Config:       ispec.DescriptorEmptyJSON,
			Layers: []ispec.Descriptor{
				{
					MediaType: "application/vnd.example+type",
					Digest:    "sha256:e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317",
					Size:      1234,
				},
			},
		}, "\t\t\t" + `Schema Version: 2
			Media Type: application/vnd.oci.image.manifest.v1+json
			Config:
				Descriptor:
					Media Type: application/vnd.oci.empty.v1+json
					Digest: sha256:44136fa355b3678a1146ad16f7e8649e94fb4fc21fe77e8310c060f61caaff8a
					Size: 2B
			Layers:
				Descriptor:
					Media Type: application/vnd.example+type
					Digest: sha256:e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317
					Size: 1.234kB
`},
		{"ArtifactExample-3", "\t\t\t", ispec.Manifest{
			Versioned:    imeta.Versioned{SchemaVersion: 2},
			MediaType:    ispec.MediaTypeImageManifest,
			ArtifactType: "application/vnd.example+type",
			Config: ispec.Descriptor{
				MediaType: "application/vnd.example.config.v1+json",
				Digest:    "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03",
				Size:      123,
			},
			Layers: []ispec.Descriptor{
				{
					MediaType: "application/vnd.example+type",
					Digest:    "sha256:e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317",
					Size:      1234,
				},
			},
		}, "\t\t\t" + `Schema Version: 2
			Media Type: application/vnd.oci.image.manifest.v1+json
			Config:
				Descriptor:
					Media Type: application/vnd.example.config.v1+json
					Digest: sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
					Size: 123B
			Layers:
				Descriptor:
					Media Type: application/vnd.example+type
					Digest: sha256:e258d248fda94c63753607f7c4494ee0fcbe92f1a76bfdac795c9d84101eb317
					Size: 1.234kB
`},
		{"Basic", "\t\t", ispec.Manifest{
			Versioned: imeta.Versioned{SchemaVersion: 2},
			MediaType: ispec.MediaTypeImageManifest,
			Config: ispec.Descriptor{
				MediaType: ispec.MediaTypeImageConfig,
				Digest:    "sha256:6d79abd4c96299aa91f5a4a46551042407568a3858b00ab460f4ba430984f62c",
				Size:      2297,
			},
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:15b1d8a5ff03aeb0f14c8d39a60a73ef22f656550bfa1bb90d1850f25a0ac0fa",
					Size:      49279531,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:22718812f617084a0c5e539e02723b75bf79ea2a589430f820efcbb07f45b91b",
					Size:      25613635,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:401a98f7495bee3e8e6943da9f52f0ab1043c43eb1d107a3817fc2a4b916be97",
					Size:      67776756,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:1c315634bf9079ab45072808dc1101241352a3762e0bb1ec75369a0f65672ab0",
					Size:      102071864,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:330457f0054be4c07fbaeac90483ac4f534113ccd944fe16beb9e079a3ab3a36",
					Size:      60045609,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:771ee02be966963d69210ed8243baa8f322661858bae69d1c2d5d13fe4dc92ba",
					Size:      126,
				},
				{
					MediaType: ispec.MediaTypeImageLayerGzip,
					Digest:    "sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1",
					Size:      32,
				},
			},
			Annotations: map[string]string{
				"com.docker.official-images.bashbrew.arch": "amd64",
				"org.opencontainers.image.base.digest":     "sha256:6f9fd607b2e260d41c51207fb74010ba06931047760494f5f6d59854e7d55db4",
				"org.opencontainers.image.base.name":       "buildpack-deps:trixie-scm",
				"org.opencontainers.image.created":         "2025-09-03T18:13:04Z",
				"org.opencontainers.image.revision":        "be5c27da377afdeebf0c5747560bedd7f96ccb1c",
				"org.opencontainers.image.source":          "https://github.com/docker-library/golang.git#be5c27da377afdeebf0c5747560bedd7f96ccb1c:1.25/trixie",
				"org.opencontainers.image.url":             "https://hub.docker.com/_/golang",
				"org.opencontainers.image.version":         "1.25.1-trixie",
			},
		}, "\t\t" + `Schema Version: 2
		Media Type: application/vnd.oci.image.manifest.v1+json
		Config:
			Descriptor:
				Media Type: application/vnd.oci.image.config.v1+json
				Digest: sha256:6d79abd4c96299aa91f5a4a46551042407568a3858b00ab460f4ba430984f62c
				Size: 2.297kB
		Layers:
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:15b1d8a5ff03aeb0f14c8d39a60a73ef22f656550bfa1bb90d1850f25a0ac0fa
				Size: 49.28MB
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:22718812f617084a0c5e539e02723b75bf79ea2a589430f820efcbb07f45b91b
				Size: 25.61MB
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:401a98f7495bee3e8e6943da9f52f0ab1043c43eb1d107a3817fc2a4b916be97
				Size: 67.78MB
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:1c315634bf9079ab45072808dc1101241352a3762e0bb1ec75369a0f65672ab0
				Size: 102.1MB
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:330457f0054be4c07fbaeac90483ac4f534113ccd944fe16beb9e079a3ab3a36
				Size: 60.05MB
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:771ee02be966963d69210ed8243baa8f322661858bae69d1c2d5d13fe4dc92ba
				Size: 126B
			Descriptor:
				Media Type: application/vnd.oci.image.layer.v1.tar+gzip
				Digest: sha256:4f4fb700ef54461cfa02571ae0db9a0dc1e0cdb5577484a6d75e68dc38e8acc1
				Size: 32B
		Annotations:
			com.docker.official-images.bashbrew.arch: amd64
			org.opencontainers.image.base.digest: sha256:6f9fd607b2e260d41c51207fb74010ba06931047760494f5f6d59854e7d55db4
			org.opencontainers.image.base.name: buildpack-deps:trixie-scm
			org.opencontainers.image.created: 2025-09-03T18:13:04Z
			org.opencontainers.image.revision: be5c27da377afdeebf0c5747560bedd7f96ccb1c
			org.opencontainers.image.source: https://github.com/docker-library/golang.git#be5c27da377afdeebf0c5747560bedd7f96ccb1c:1.25/trixie
			org.opencontainers.image.url: https://hub.docker.com/_/golang
			org.opencontainers.image.version: 1.25.1-trixie
`},
	} {
		t.Run(test.name, func(t *testing.T) {
			var output bytes.Buffer
			err := pprintManifest(&output, test.prefix, test.manifest)
			require.NoErrorf(t, err, "pprint(%q, %#v)", test.prefix, test.manifest)
			assert.Equal(t, test.expected, output.String())
		})
	}
}
