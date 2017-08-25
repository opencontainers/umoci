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

package idtools

import (
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

func TestToHost(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      1337,
			ContainerID: 0,
			Size:        1,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 1337, container: 0, failure: false},
		{host: -1, container: 1337, failure: true},
		{host: -1, container: 1, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToHost(test.container, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.host {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestToHostNil(t *testing.T) {
	for _, test := range []int{
		1337,
		8000,
		2222,
		0,
		1,
	} {
		id, err := ToHost(test, nil)
		if err != nil {
			t.Errorf("unexpected error: %+v", err)
		} else if id != test {
			t.Errorf("expected to get %d, got %d", test, id)
		}
	}
}

func TestToHostLarger(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      8000,
			ContainerID: 0,
			Size:        1000,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 8000, container: 0, failure: false},
		{host: 8232, container: 232, failure: false},
		{host: 8999, container: 999, failure: false},
		{host: -1, container: 1000, failure: true},
		{host: -1, container: 8000, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToHost(test.container, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.host {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestToHostMultiple(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      2222,
			ContainerID: 0,
			Size:        100,
		},
		{
			HostID:      7777,
			ContainerID: 100,
			Size:        300,
		},
		{
			HostID:      9001,
			ContainerID: 9001,
			Size:        1,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 9001, container: 9001, failure: false},
		{host: 2222, container: 0, failure: false},
		{host: 2272, container: 50, failure: false},
		{host: 2321, container: 99, failure: false},
		{host: 7777, container: 100, failure: false},
		{host: 8010, container: 333, failure: false},
		{host: 8076, container: 399, failure: false},
		{host: -1, container: 400, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToHost(test.container, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.host {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestToContainer(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      1337,
			ContainerID: 0,
			Size:        1,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 1337, container: 0, failure: false},
		{host: -1, container: 1337, failure: true},
		{host: -1, container: 1, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToContainer(test.host, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.container {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestToContainerNil(t *testing.T) {
	for _, test := range []int{
		1337,
		8000,
		2222,
		0,
		1,
	} {
		id, err := ToContainer(test, nil)
		if err != nil {
			t.Errorf("unexpected error: %+v", err)
		} else if id != test {
			t.Errorf("expected to get %d, got %d", test, id)
		}
	}
}

func TestToContainerLarger(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      8000,
			ContainerID: 0,
			Size:        1000,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 8000, container: 0, failure: false},
		{host: 8232, container: 232, failure: false},
		{host: 8999, container: 999, failure: false},
		{host: -1, container: 1000, failure: true},
		{host: -1, container: 8000, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToContainer(test.host, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.container {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestToContainerMultiple(t *testing.T) {
	idMap := []rspec.LinuxIDMapping{
		{
			HostID:      2222,
			ContainerID: 0,
			Size:        100,
		},
		{
			HostID:      7777,
			ContainerID: 100,
			Size:        300,
		},
		{
			HostID:      9001,
			ContainerID: 9001,
			Size:        1,
		},
	}

	for _, test := range []struct {
		host, container int
		failure         bool
	}{
		{host: 9001, container: 9001, failure: false},
		{host: 2222, container: 0, failure: false},
		{host: 2272, container: 50, failure: false},
		{host: 2321, container: 99, failure: false},
		{host: 7777, container: 100, failure: false},
		{host: 8010, container: 333, failure: false},
		{host: 8076, container: 399, failure: false},
		{host: -1, container: 400, failure: true},
		{host: -1, container: -1, failure: true},
	} {
		id, err := ToContainer(test.host, idMap)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with container=%d", test.container)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else if id != test.container {
				t.Errorf("expected to get %d, got %d", test.host, id)
			}
		}
	}
}

func TestParseIDMapping(t *testing.T) {

	for _, test := range []struct {
		spec                  string
		host, container, size uint32
		failure               bool
	}{
		{spec: "0:0:1", host: 0, container: 0, size: 1, failure: false},
		{spec: "32:100:2421", host: 100, container: 32, size: 2421, failure: false},
		{spec: "0:1337:1924", host: 1337, container: 0, size: 1924, failure: false},
		{spec: "2:1", host: 1, container: 2, size: 1, failure: false},
		{spec: "422:123", host: 123, container: 422, size: 1, failure: false},
		{spec: "", host: 0, container: 0, size: 0, failure: true},
		{spec: "::", host: 0, container: 0, size: 0, failure: true},
		{spec: "1:2:", host: 0, container: 0, size: 0, failure: true},
		{spec: "in:va:lid", host: 0, container: 0, size: 0, failure: true},
		{spec: "1:n:0", host: 0, container: 0, size: 0, failure: true},
		{spec: "i:2:0", host: 0, container: 0, size: 0, failure: true},
	} {
		idMap, err := ParseMapping(test.spec)
		if test.failure {
			if err == nil {
				t.Errorf("expected an error with spec %s -- got %+v", test.spec, idMap)
			}
		} else {
			if err != nil {
				t.Errorf("unexpected error: %+v", err)
			} else {
				if idMap.HostID != test.host {
					t.Errorf("%q: expected to get host %d, got %d", test.spec, test.host, idMap.HostID)
				}
				if idMap.ContainerID != test.container {
					t.Errorf("%q: expected to get container %d, got %d", test.spec, test.container, idMap.HostID)
				}
				if idMap.Size != test.size {
					t.Errorf("%q: expected to get size %d, got %d", test.spec, test.size, idMap.HostID)
				}
			}
		}
	}

}
