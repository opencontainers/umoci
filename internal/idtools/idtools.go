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

// Package idtools provides helpers for dealing with Linux ID mappings.
package idtools

import (
	"fmt"
	"strconv"
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
)

// ToHost translates a remapped container ID to an unmapped host ID using the
// provided ID mapping. If no mapping is provided, then the mapping is a no-op.
// If there is no mapping for the given ID an error is returned.
func ToHost(contID int, idMap []rspec.LinuxIDMapping) (int, error) {
	if idMap == nil {
		return contID, nil
	}

	for _, m := range idMap {
		if uint32(contID) >= m.ContainerID && uint32(contID) < m.ContainerID+m.Size {
			return int(m.HostID + (uint32(contID) - m.ContainerID)), nil
		}
	}

	return -1, fmt.Errorf("container id %d cannot be mapped to a host id", contID)
}

// ToContainer takes an unmapped host ID and translates it to a remapped
// container ID using the provided ID mapping. If no mapping is provided, then
// the mapping is a no-op. If there is no mapping for the given ID an error is
// returned.
func ToContainer(hostID int, idMap []rspec.LinuxIDMapping) (int, error) {
	if idMap == nil {
		return hostID, nil
	}

	for _, m := range idMap {
		if uint32(hostID) >= m.HostID && uint32(hostID) < m.HostID+m.Size {
			return int(m.ContainerID + (uint32(hostID) - m.HostID)), nil
		}
	}

	return -1, fmt.Errorf("host id %d cannot be mapped to a container id", hostID)
}

// Helper to return a uint32 from strconv.ParseUint type-safely.
func parseUint32(str string) (uint32, error) {
	val, err := strconv.ParseUint(str, 10, 32)
	return uint32(val), err
}

// ParseMapping takes a mapping string of the form "container:host[:size]" and
// returns the corresponding rspec.LinuxIDMapping. An error is returned if not
// enough fields are provided or are otherwise invalid. The default size is 1.
func ParseMapping(spec string) (rspec.LinuxIDMapping, error) {
	parts := strings.Split(spec, ":")

	var err error
	var hostID, contID, size uint32
	switch len(parts) {
	case 3:
		size, err = parseUint32(parts[2])
		if err != nil {
			return rspec.LinuxIDMapping{}, fmt.Errorf("invalid size in mapping: %w", err)
		}
	case 2:
		size = 1
	default:
		return rspec.LinuxIDMapping{}, fmt.Errorf("invalid number of fields in mapping %q: %d", spec, len(parts))
	}

	contID, err = parseUint32(parts[0])
	if err != nil {
		return rspec.LinuxIDMapping{}, fmt.Errorf("invalid containerID in mapping: %w", err)
	}

	hostID, err = parseUint32(parts[1])
	if err != nil {
		return rspec.LinuxIDMapping{}, fmt.Errorf("invalid hostID in mapping: %w", err)
	}

	return rspec.LinuxIDMapping{
		HostID:      hostID,
		ContainerID: contID,
		Size:        size,
	}, nil
}
