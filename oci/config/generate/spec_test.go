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

package generate

import (
	"io"
	"os"
	"slices"
	"testing"
	"time"

	// Import is necessary for go-digest.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteTo(t *testing.T) {
	g := New()

	fh, err := os.CreateTemp(t.TempDir(), "umoci-TestWriteTo")
	require.NoError(t, err)
	defer fh.Close() //nolint:errcheck

	size, err := g.WriteTo(fh)
	require.NoError(t, err, "generator WriteTo")

	fi, err := fh.Stat()
	require.NoError(t, err, "stat target")
	assert.Equal(t, size, fi.Size(), "returned WriteTo size should be the final file size")
}

func TestConfigUser(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetConfigUser(expected)
	got := g.ConfigUser()

	assert.Equal(t, expected, got, "ConfigUser get/set should match")
}

func TestConfigWorkingDir(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetConfigWorkingDir(expected)
	got := g.ConfigWorkingDir()

	assert.Equal(t, expected, got, "ConfigWorkingDir get/set should match")
}

func TestPlatformOS(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetPlatformOS(expected)
	got := g.PlatformOS()

	assert.Equal(t, expected, got, "PlatformOS get/set should match")
}

func TestPlatformArchitecture(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetPlatformArchitecture(expected)
	got := g.PlatformArchitecture()

	assert.Equal(t, expected, got, "PlatformArchitecture get/set should match")
}

func TestAuthor(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetAuthor(expected)
	got := g.Author()

	assert.Equal(t, expected, got, "Author get/set should match")
}

func TestRootfsType(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetRootfsType(expected)
	got := g.RootfsType()

	assert.Equal(t, expected, got, "RootfsType get/set should match")
}

func TestRootfsDiffIDs(t *testing.T) {
	g := New()

	values := []string{"a", "b", "c"}
	diffids := []digest.Digest{}
	for _, value := range values {
		digester := digest.SHA256.Digester()
		_, _ = io.WriteString(digester.Hash(), value)
		diffids = append(diffids, digester.Digest())
	}

	g.ClearRootfsDiffIDs()
	for _, diffid := range diffids {
		g.AddRootfsDiffID(diffid)
	}
	got := g.RootfsDiffIDs()

	assert.Equal(t, diffids, got, "RootfsDiffIDs get/set should match")
}

func TestConfigEntrypoint(t *testing.T) {
	g := New()
	expected := []string{"a", "b", "c"}

	g.SetConfigEntrypoint(expected)
	got := g.ConfigEntrypoint()

	assert.Equal(t, expected, got, "ConfigEntrypoint get/set should match")
}

func TestConfigCmd(t *testing.T) {
	g := New()
	expected := []string{"a", "b", "c"}

	g.SetConfigCmd(expected)
	got := g.ConfigCmd()

	assert.Equal(t, expected, got, "ConfigCmd get/set should match")
}

func TestConfigExposedPorts(t *testing.T) {
	g := New()
	exposedPorts := []string{"100/udp", "200/tcp", "300/foo"}

	g.ClearConfigExposedPorts()
	for _, port := range exposedPorts {
		g.AddConfigExposedPort(port)
	}
	got := g.ConfigExposedPorts()

	slices.Sort(exposedPorts)
	assert.Equal(t, exposedPorts, got, "ConfigExposedPorts get/set should match")

	exposedPorts = []string{"100/udp", "300/foo"}
	g.RemoveConfigExposedPort("200/tcp")
	got[0] = "dummy" // make sure it's a copy
	got = g.ConfigExposedPorts()

	slices.Sort(exposedPorts)
	assert.Equal(t, exposedPorts, got, "ConfigExposedPorts get/set should match")
}

func TestConfigVolumes(t *testing.T) {
	g := New()
	volumes := []string{"a", "b", "c"}

	g.ClearConfigVolumes()
	for _, volume := range volumes {
		g.AddConfigVolume(volume)
	}
	got := g.ConfigVolumes()

	slices.Sort(volumes)
	assert.Equal(t, volumes, got, "ConfigVolumes get/set should match")

	volumes = []string{"b", "c"}
	g.RemoveConfigVolume("a")
	got[0] = "dummy" // make sure it's a copy
	got = g.ConfigVolumes()

	slices.Sort(volumes)
	assert.Equal(t, volumes, got, "ConfigVolumes get/set should match")
}

func TestConfigEnv(t *testing.T) {
	g := New()
	env := []string{
		"HOME=a,b,c",
		"TEST=a=b=c",
		"ANOTHER=",
	}

	g.ClearConfigEnv()
	g.AddConfigEnv("HOME", "a,b,c")
	g.AddConfigEnv("TEST", "a=b=c")
	g.AddConfigEnv("ANOTHER", "")
	got := g.ConfigEnv()

	assert.Equal(t, env, got, "ConfigEnv get/set should match")

	env[1] = "TEST=different"
	g.AddConfigEnv("TEST", "different")
	got[0] = "badvalue=" // make sure it's a copy
	got = g.ConfigEnv()

	assert.Equal(t, env, got, "ConfigEnv get/set should match")
}

func TestConfigLabels(t *testing.T) {
	g := New()
	labels := map[string]string{
		"some":  "key",
		"value": "mappings",
		"go":    "here",
	}

	g.ClearConfigLabels()
	for k, v := range labels {
		g.AddConfigLabel(k, v)
	}
	got := g.ConfigLabels()

	assert.Equal(t, labels, got, "ConfigLabels get/set should match")

	delete(labels, "some")
	g.RemoveConfigLabel("some")
	delete(got, "value") // make sure it's a copy
	got = g.ConfigLabels()

	assert.Equal(t, labels, got, "ConfigLabels get/set should match")

	delete(labels, "nonexist")
	g.RemoveConfigLabel("nonexist")
	delete(got, "value") // make sure it's a copy
	got = g.ConfigLabels()

	assert.Equal(t, labels, got, "ConfigLabels get/set should match")
}

func TestConfigStopSignal(t *testing.T) {
	g := New()
	signals := []string{
		"SIGSTOP",
		"SIGKILL",
		"SIGUSR1",
		"SIGINFO",
		"SIGPWR",
		"SIGRT13",
	}

	for _, signal := range signals {
		g.SetConfigStopSignal(signal)
		got := g.ConfigStopSignal()
		assert.Equal(t, signal, got, "ConfigStopSignal get/set should match")
	}
}

func TestCreated(t *testing.T) {
	g := New()
	timeA := time.Now()
	g.SetCreated(timeA)
	timeB := g.Created()

	assert.Equal(t, timeA, timeB, "Created get/set should match")
}
