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

package layer

import (
	"archive/tar"
	"testing"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rootlesscontainers "github.com/rootless-containers/proto/go-proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

// We need to have individual tests for the rootlesscontainers-proto, because
// when we do integration tests for the "user.rootlesscontainers" handling we
// don't have access to the protobuf parser from within bats (we just match the
// strings which isn't as easy to debug).

// TestMapRootless ensures that mapHeader correctly handles the rootless case
// (both for user.rootlesscontainers and the more general case).
func TestMapRootless(t *testing.T) {
	// Just a basic "*archive/tar.Header" that we use for testing.
	rootUid := 1000 //nolint:revive // Uid is preferred
	rootGid := 100  //nolint:revive // Gid is preferred
	mapOptions := MapOptions{
		UIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootUid), ContainerID: 0, Size: 1}},
		GIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootGid), ContainerID: 0, Size: 1}},
		Rootless:    true,
	}
	baseHdr := tar.Header{
		Name:     "etc/passwd",
		Typeflag: tar.TypeReg,
		Xattrs:   make(map[string]string), //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
	}

	for _, test := range []struct {
		name           string
		uid, gid       int                          // (uid, gid) for hdr
		proto          *rootlesscontainers.Resource // (uid, gid) for xattr
		outUid, outGid int                          //nolint:revive // (uid, gid) after map
		errorExpected  bool
	}{
		// Noop values.
		{"NilMap", 0, 0, nil, 0, 0, false},
		{"NoopMap", 0, 0, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: rootlesscontainers.NoopID}, 0, 0, false},
		// Basic mappings.
		{"OnlyUid", 0, 0, &rootlesscontainers.Resource{Uid: 123, Gid: rootlesscontainers.NoopID}, 123, 0, false},
		{"OnlyGid", 0, 0, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: 456}, 0, 456, false},
		{"UidGid", 0, 0, &rootlesscontainers.Resource{Uid: 8001, Gid: 1337}, 8001, 1337, false},
		// XXX: We cannot enable these tests at the moment because we currently
		//      just ignore the owner and always set it to 0 for rootless
		//      containers.
		//{851, 182, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: rootlesscontainers.NoopID}, 851, 182, false},
		//{154, 992, &rootlesscontainers.Resource{Uid: 9982, Gid: rootlesscontainers.NoopID}, 9982, 992, false},
		//{291, 875, &rootlesscontainers.Resource{Uid: 42158, Gid: 31337}, 42158, 31337, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Update baseHdr.
			baseHdr.Uid = test.uid
			baseHdr.Gid = test.gid
			delete(baseHdr.Xattrs, rootlesscontainers.Keyname) //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
			if test.proto != nil {
				payload, err := proto.Marshal(test.proto)
				require.NoError(t, err, "marshal proto")
				baseHdr.Xattrs[rootlesscontainers.Keyname] = string(payload) //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
			}

			// Map.
			err := mapHeader(&baseHdr, mapOptions)
			if test.errorExpected {
				require.Error(t, err, "map header should fail")
			} else {
				require.NoError(t, err, "map header")
			}

			// Output header shouldn't contain "user.rootlesscontainers".
			assert.NotContains(t, baseHdr.Xattrs, rootlesscontainers.Keyname, "user.rootlesscontainers should not be mapped") //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
			// Make sure that the uid and gid are what we wanted.
			assert.Equal(t, test.outUid, baseHdr.Uid, "mapped uid")
			assert.Equal(t, test.outGid, baseHdr.Gid, "mapped uid")
		})
	}
}

// TestUnmapRootless ensures that unmapHeader correctly handles the rootless
// case (both for user.rootlesscontainers and the more general case).
func TestUnmapRootless(t *testing.T) {
	// Just a basic "*archive/tar.Header" that we use for testing.
	rootUid := 1000 //nolint:revive // Uid is preferred
	rootGid := 100  //nolint:revive // Gid is preferred
	mapOptions := MapOptions{
		UIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootUid), ContainerID: 0, Size: 1}},
		GIDMappings: []rspec.LinuxIDMapping{{HostID: uint32(rootGid), ContainerID: 0, Size: 1}},
		Rootless:    true,
	}
	baseHdr := tar.Header{
		Name:     "etc/passwd",
		Typeflag: tar.TypeReg,
	}

	for _, test := range []struct {
		name     string
		uid, gid int                          // (uid, gid) for hdr
		proto    *rootlesscontainers.Resource // expected "user.rootlesscontainers" payload
	}{
		// Basic mappings to check.
		{"Root", 0, 0, nil},
		{"RootGid", 521, 0, &rootlesscontainers.Resource{Uid: 521, Gid: rootlesscontainers.NoopID}},
		{"RootUid", 0, 942, &rootlesscontainers.Resource{Uid: rootlesscontainers.NoopID, Gid: 942}},
		{"NonRoot1", 333, 825, &rootlesscontainers.Resource{Uid: 333, Gid: 825}},
		{"NonRoot2", 185, 9923, &rootlesscontainers.Resource{Uid: 185, Gid: 9923}},
	} {
		t.Run(test.name, func(t *testing.T) {
			// Update baseHdr.
			baseHdr.Uid = test.uid
			baseHdr.Gid = test.gid
			delete(baseHdr.Xattrs, rootlesscontainers.Keyname) //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying

			// Unmap.
			err := unmapHeader(&baseHdr, mapOptions)
			require.NoError(t, err, "unmapHeader")

			// Check the owner.
			assert.Equal(t, rootUid, baseHdr.Uid, "unmapped uid")
			assert.Equal(t, rootGid, baseHdr.Gid, "unmapped uid")

			// Check that the xattr is what we wanted.
			if test.proto == nil {
				assert.NotContains(t, baseHdr.Xattrs, rootlesscontainers.Keyname, "mapping shouldn't create user.rootlesscontainers") //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
			} else {
				assert.Contains(t, baseHdr.Xattrs, rootlesscontainers.Keyname, "mapping should create user.rootlesscontainers") //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying
				payload := baseHdr.Xattrs[rootlesscontainers.Keyname]                                                           //nolint:staticcheck // SA1019: Xattrs is deprecated but PAXRecords is more annoying

				var parsed rootlesscontainers.Resource
				err := proto.Unmarshal([]byte(payload), &parsed)
				require.NoError(t, err, "unmarshal user.rootlesscontainers payload")

				assert.Equal(t, test.proto.Uid, parsed.Uid, "user.rootlesscontainers payload uid")
				assert.Equal(t, test.proto.Gid, parsed.Gid, "user.rootlesscontainers payload gid")
			}

			// Finally, just do a check to ensure that mapHeader returns the old values.
			err = mapHeader(&baseHdr, mapOptions)
			require.NoError(t, err, "mapHeader")
			assert.Equal(t, test.uid, baseHdr.Uid, "mapped uid")
			assert.Equal(t, test.gid, baseHdr.Gid, "mapped uid")
		})
	}
}
