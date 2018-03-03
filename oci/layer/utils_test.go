/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017, 2018 SUSE LLC.
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

package layer

import (
	"archive/tar"
	"testing"

	"github.com/golang/protobuf/proto"
	"github.com/openSUSE/umoci/pkg/rootlesscontainers-proto"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// We need to have individual tests for the rootlesscontainers-proto, because
// when we do integration tests for the "user.rootlesscontainers" handling we
// don't have access to the protobuf parser from within bats (we just match the
// strings which isn't as easy to debug).

// TestMapRootless ensures that mapHeader correctly handles the rootless case
// (both for user.rootlesscontainers and the more general case).
func TestMapRootless(t *testing.T) {
	// Just a basic "*archive/tar.Header" that we use for testing.
	rootUID := 1000
	rootGID := 100
	mapOptions := MapOptions{
		UIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootUID), ContainerID: 0, Size: 1}},
		GIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootGID), ContainerID: 0, Size: 1}},
		Rootless:    true,
	}
	baseHdr := tar.Header{
		Name:     "etc/passwd",
		Typeflag: tar.TypeReg,
		Xattrs:   make(map[string]string),
	}

	for idx, test := range []struct {
		uid, gid       int                          // (uid, gid) for hdr
		proto          *rootlesscontainers.Resource // (uid, gid) for xattr
		outUID, outGID int                          // (uid, gid) after map
		errorExpected  bool
	}{
		// Noop values.
		{0, 0, nil, 0, 0, false},
		{0, 0, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: rootlesscontainers.NoopID}, 0, 0, false},
		// Basic mappings.
		{0, 0, &rootlesscontainers.Resource{Uid: 123, Gid: rootlesscontainers.NoopID}, 123, 0, false},
		{0, 0, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: 456}, 0, 456, false},
		{0, 0, &rootlesscontainers.Resource{Uid: 8001, Gid: 1337}, 8001, 1337, false},
		// XXX: We cannot enable these tests at the moment because we currently
		//      just ignore the owner and always set it to 0 for rootless
		//      containers.
		//{851, 182, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: rootlesscontainers.NoopID}, 851, 182, false},
		//{154, 992, &rootlesscontainers.Resource{Uid: 9982, Gid: rootlesscontainers.NoopID}, 9982, 992, false},
		//{291, 875, &rootlesscontainers.Resource{Uid: 42158, Gid: 31337}, 42158, 31337, false},
	} {
		// Update baseHdr.
		baseHdr.Uid = test.uid
		baseHdr.Gid = test.gid
		delete(baseHdr.Xattrs, rootlesscontainers.Keyname)
		if test.proto != nil {
			payload, err := proto.Marshal(test.proto)
			if err != nil {
				t.Errorf("test%d: failed to marshal proto in test", idx)
				continue
			}
			baseHdr.Xattrs[rootlesscontainers.Keyname] = string(payload)
		}

		// Map.
		err := mapHeader(&baseHdr, mapOptions)
		if (err != nil) != test.errorExpected {
			t.Errorf("test%d: error value was unexpected, errorExpected=%v got err=%v", idx, test.errorExpected, err)
			continue
		}

		// Output header shouldn't contain "user.rootlesscontainers".
		if value, ok := baseHdr.Xattrs[rootlesscontainers.Keyname]; ok {
			t.Errorf("test%d: 'user.rootlesscontainers' present in output: value=%v", idx, value)
		}
		// Make sure that the uid and gid are what we wanted.
		if uid := baseHdr.Uid; uid != test.outUID {
			t.Errorf("test%d: got unexpected uid: wanted %v got %v", idx, test.outUID, uid)
		}
		if gid := baseHdr.Gid; gid != test.outGID {
			t.Errorf("test%d: got unexpected gid: wanted %v got %v", idx, test.outGID, gid)
		}
	}
}

// TestUnmapRootless ensures that unmapHeader correctly handles the rootless
// case (both for user.rootlesscontainers and the more general case).
func TestUnmapRootless(t *testing.T) {
	// Just a basic "*archive/tar.Header" that we use for testing.
	rootUID := 1000
	rootGID := 100
	mapOptions := MapOptions{
		UIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootUID), ContainerID: 0, Size: 1}},
		GIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootGID), ContainerID: 0, Size: 1}},
		Rootless:    true,
	}
	baseHdr := tar.Header{
		Name:     "etc/passwd",
		Typeflag: tar.TypeReg,
	}

	for idx, test := range []struct {
		uid, gid int                          // (uid, gid) for hdr
		proto    *rootlesscontainers.Resource // expected "user.rootlesscontainers" payload
	}{
		// Basic mappings to check.
		{0, 0, nil},
		{521, 0, &rootlesscontainers.Resource{Uid: 521, Gid: rootlesscontainers.NoopID}},
		{0, 942, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: 942}},
		{333, 825, &rootlesscontainers.Resource{Uid: 333, Gid: 825}},
		{185, 9923, &rootlesscontainers.Resource{Uid: 185, Gid: 9923}},
	} {
		// Update baseHdr.
		baseHdr.Uid = test.uid
		baseHdr.Gid = test.gid
		delete(baseHdr.Xattrs, rootlesscontainers.Keyname)

		// Unmap.
		if err := unmapHeader(&baseHdr, mapOptions); err != nil {
			t.Errorf("test%d: unexpected error in unmapHeader: %v", idx, err)
			continue
		}

		// Check the owner.
		if baseHdr.Uid != rootUID {
			t.Errorf("test%d: got hdr uid %d when expected %d", idx, baseHdr.Uid, rootUID)
		}
		if baseHdr.Gid != rootGID {
			t.Errorf("test%d: got hdr gid %d when expected %d", idx, baseHdr.Gid, rootGID)
		}

		// Check that the xattr is what we wanted.
		if payload, ok := baseHdr.Xattrs[rootlesscontainers.Keyname]; (test.proto != nil) != ok {
			// Only bad if we expected a proto...
			t.Errorf("test%d: unexpected situation: expected xattr exist to be %v", idx, test.proto != nil)
			continue
		} else if ok {
			var parsed rootlesscontainers.Resource
			if err := proto.Unmarshal([]byte(payload), &parsed); err != nil {
				t.Errorf("test%d: unexpected error parsing payload: %v", idx, err)
				continue
			}

			if parsed.Uid != test.proto.Uid {
				t.Errorf("test%d: got xattr uid %d when expected %d", idx, parsed.Uid, test.proto.Uid)
			}
			if parsed.Gid != test.proto.Gid {
				t.Errorf("test%d: got xattr gid %d when expected %d", idx, parsed.Gid, test.proto.Gid)
			}
		}

		// Finally, just do a check to ensure that mapHeader returns the old values.
		if err := mapHeader(&baseHdr, mapOptions); err != nil {
			t.Errorf("test%d: unexpected error in mapHeader: %v", idx, err)
			continue
		}
		if baseHdr.Uid != test.uid {
			t.Errorf("test%d: round-trip of uid failed: expected %d got %d", idx, test.uid, baseHdr.Uid)
		}
		if baseHdr.Gid != test.gid {
			t.Errorf("test%d: round-trip of gid failed: expected %d got %d", idx, test.gid, baseHdr.Gid)
		}
	}
}
