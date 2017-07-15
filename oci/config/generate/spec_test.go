/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"
	// Import is necessary for go-digest.
	_ "crypto/sha256"

	"github.com/opencontainers/go-digest"
)

func TestWriteTo(t *testing.T) {
	g := New()

	fh, err := ioutil.TempFile("", "umoci-TestWriteTo")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(fh.Name())
	defer fh.Close()

	size, err := g.WriteTo(fh)
	if err != nil {
		t.Errorf("failed to write: %+v", err)
	}

	fi, err := fh.Stat()
	if err != nil {
		t.Errorf("failed to stat: %+v", err)
	}

	if fi.Size() != size {
		t.Errorf("wrong size: expected %d, got %d", size, fi.Size())
	}
}

func TestConfigUser(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetConfigUser(expected)
	got := g.ConfigUser()

	if expected != got {
		t.Errorf("ConfigUser get/set doesn't match: expected %v, got %v", expected, got)
	}
}

func TestConfigWorkingDir(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetConfigWorkingDir(expected)
	got := g.ConfigWorkingDir()

	if expected != got {
		t.Errorf("ConfigWorkingDir get/set doesn't match: expected %v, got %v", expected, got)
	}
}

func TestArchitecture(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetArchitecture(expected)
	got := g.Architecture()

	if expected != got {
		t.Errorf("Architecture get/set doesn't match: expected %v, got %v", expected, got)
	}
}

func TestOS(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetOS(expected)
	got := g.OS()

	if expected != got {
		t.Errorf("OS get/set doesn't match: expected %v, got %v", expected, got)
	}
}

func TestAuthor(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetAuthor(expected)
	got := g.Author()

	if expected != got {
		t.Errorf("Author get/set doesn't match: expected %v, got %v", expected, got)
	}
}

func TestRootfsType(t *testing.T) {
	g := New()
	expected := "some_value"

	g.SetRootfsType(expected)
	got := g.RootfsType()

	if expected != got {
		t.Errorf("RootfsType get/set doesn't match: expected %v, got %v", expected, got)
	}
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

	if !reflect.DeepEqual(diffids, got) {
		t.Errorf("RootfsDiffIDs doesn't match: expected %v, got %v", diffids, got)
	}
}

func TestConfigEntrypoint(t *testing.T) {
	g := New()
	entrypoint := []string{"a", "b", "c"}

	g.SetConfigEntrypoint(entrypoint)
	got := g.ConfigEntrypoint()

	if !reflect.DeepEqual(entrypoint, got) {
		t.Errorf("ConfigEntrypoint doesn't match: expected %v, got %v", entrypoint, got)
	}
}

func TestConfigCmd(t *testing.T) {
	g := New()
	entrypoint := []string{"a", "b", "c"}

	g.SetConfigCmd(entrypoint)
	got := g.ConfigCmd()

	if !reflect.DeepEqual(entrypoint, got) {
		t.Errorf("ConfigCmd doesn't match: expected %v, got %v", entrypoint, got)
	}
}

func TestConfigExposedPorts(t *testing.T) {
	g := New()
	exposedports := map[string]struct{}{
		"a": {},
		"b": {},
		"c": {},
	}

	g.ClearConfigExposedPorts()
	for exposedport := range exposedports {
		g.AddConfigExposedPort(exposedport)
	}

	got := g.ConfigExposedPorts()
	if !reflect.DeepEqual(exposedports, got) {
		t.Errorf("ConfigExposedPorts doesn't match: expected %v, got %v", exposedports, got)
	}

	delete(exposedports, "a")
	g.RemoveConfigExposedPort("a")
	delete(got, "b")

	got = g.ConfigExposedPorts()
	if !reflect.DeepEqual(exposedports, got) {
		t.Errorf("ConfigExposedPorts doesn't match: expected %v, got %v", exposedports, got)
	}
}

func TestConfigVolumes(t *testing.T) {
	g := New()
	volumes := map[string]struct{}{
		"a": {},
		"b": {},
		"c": {},
	}

	g.ClearConfigVolumes()
	for volume := range volumes {
		g.AddConfigVolume(volume)
	}

	got := g.ConfigVolumes()
	if !reflect.DeepEqual(volumes, got) {
		t.Errorf("ConfigVolumes doesn't match: expected %v, got %v", volumes, got)
	}

	delete(volumes, "a")
	g.RemoveConfigVolume("a")
	delete(got, "b")

	got = g.ConfigVolumes()
	if !reflect.DeepEqual(volumes, got) {
		t.Errorf("ConfigVolumes doesn't match: expected %v, got %v", volumes, got)
	}
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
	if !reflect.DeepEqual(env, got) {
		t.Errorf("ConfigEnv doesn't match: expected %v, got %v", env, got)
	}

	env[1] = "TEST=different"
	g.AddConfigEnv("TEST", "different")

	got = g.ConfigEnv()
	if !reflect.DeepEqual(env, got) {
		t.Errorf("ConfigEnv doesn't match: expected %v, got %v", env, got)
	}
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
	if !reflect.DeepEqual(labels, got) {
		t.Errorf("ConfigLabels doesn't match: expected %v, got %v", labels, got)
	}

	delete(labels, "some")
	g.RemoveConfigLabel("some")

	got = g.ConfigLabels()
	if !reflect.DeepEqual(labels, got) {
		t.Errorf("ConfigLabels doesn't match: expected %v, got %v", labels, got)
	}

	delete(labels, "nonexist")
	g.RemoveConfigLabel("nonexist")
	delete(got, "value")

	got = g.ConfigLabels()
	if !reflect.DeepEqual(labels, got) {
		t.Errorf("ConfigLabels doesn't match: expected %v, got %v", labels, got)
	}
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
		if signal != got {
			t.Errorf("ConfigStopSignal doesn't match: expected %q, got %q", signal, got)
		}
	}
}

func TestCreated(t *testing.T) {
	g := New()
	timeA := time.Now()
	g.SetCreated(timeA)
	timeB := g.Created()

	if !timeA.Equal(timeB) {
		t.Errorf("created get/set doesn't match: expected %v, got %v", timeA, timeB)
	}
}
