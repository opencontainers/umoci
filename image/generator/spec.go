/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016 SUSE LLC.
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

package generator

import (
	"encoding/json"
	"io"
	"os"
	"time"

	"github.com/opencontainers/image-spec/specs-go/v1"
)

// FIXME: Because we are not a part of upstream, we have to add some tests that
//        ensure that this set of getters and setters is complete. This should
//        be possible through some reflection.

// FIXME: Implement initConfig which makes sure everything has a valid zero value.

// Generator allows you to generate a mutable OCI image-spec configuration
// which can be written to a file (and its digest computed). It is the
// recommended way of handling modification and generation of image-spec
// configuration blobs.
type Generator struct {
	image v1.Image
}

// init makes sure everything has a "proper" zero value.
func (g *Generator) init() {
	if g.image.Config.ExposedPorts == nil {
		g.ClearConfigExposedPorts()
	}
	if g.image.Config.Env == nil {
		g.ClearConfigEnv()
	}
	if g.image.Config.Entrypoint == nil {
		g.SetConfigEntrypoint([]string{})
	}
	if g.image.Config.Cmd == nil {
		g.SetConfigCmd([]string{})
	}
	if g.image.Config.Volumes == nil {
		g.ClearConfigVolumes()
	}
	if g.image.RootFS.DiffIDs == nil {
		g.ClearRootfsDiffIDs()
	}
	if g.image.History == nil {
		g.ClearHistory()
	}
}

// New creates a new Generator with the inital template set to a default. It is
// not recommended to leave any of the options as their default values (they
// may change in the future without warning and may be invalid images).
func New() *Generator {
	// FIXME: Come up with some sane default.
	g := &Generator{
		image: v1.Image{},
	}
	g.init()
	return g
}

// NewFromTemplate creates a new Generator with the initial template being
// unmarshaled from JSON read from the provided reader (which must unmarshal
// into a valid v1.Image).
func NewFromTemplate(r io.Reader) (*Generator, error) {
	var image v1.Image
	if err := json.NewDecoder(r).Decode(&image); err != nil {
		return nil, err
	}

	// TODO: Should we validate the image here?

	g := &Generator{
		image: image,
	}

	g.init()
	return g, nil
}

// NewFromFile creates a new Generator with the initial template being
// unmarshaled from JSON read from the provided file (which must unmarshal
// into a valid v1.Image).
func NewFromFile(path string) (*Generator, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()

	return NewFromTemplate(fh)
}

// NewFromImage generates a new generator with the initial template being the
// given v1.Image.
func NewFromImage(image v1.Image) (*Generator, error) {
	g := &Generator{
		image: image,
	}

	g.init()
	return g, nil
}

// Image returns a copy of the current state of the generated image.
func (g *Generator) Image() v1.Image {
	return g.image
}

// SetConfigUser sets the username or UID which the process in the container should run as.
func (g *Generator) SetConfigUser(user string) {
	g.image.Config.User = user
}

// ConfigUser returns the username or UID which the process in the container should run as.
func (g *Generator) ConfigUser() string {
	return g.image.Config.User
}

// SetConfigMemory sets the memory limit.
func (g *Generator) SetConfigMemory(memory int64) {
	g.image.Config.Memory = memory
}

// ConfigMemory returns the memory limit.
func (g *Generator) ConfigMemory() int64 {
	return g.image.Config.Memory
}

// SetConfigMemorySwap sets the total memory usage limit (memory + swap).
func (g *Generator) SetConfigMemorySwap(memorySwap int64) {
	g.image.Config.MemorySwap = memorySwap
}

// ConfigMemorySwap returns the total memory usage limit (memory + swap).
func (g *Generator) ConfigMemorySwap() int64 {
	return g.image.Config.MemorySwap
}

// SetConfigCPUShares sets the CPU shares (relative weight vs. other containers).
func (g *Generator) SetConfigCPUShares(shares int64) {
	g.image.Config.CPUShares = shares
}

// ConfigCPUShares sets the CPU shares (relative weight vs. other containers).
func (g *Generator) ConfigCPUShares() int64 {
	return g.image.Config.CPUShares
}

// ClearConfigExposedPorts clears the set of ports to expose from a container running this image.
func (g *Generator) ClearConfigExposedPorts() {
	g.image.Config.ExposedPorts = map[string]struct{}{}
}

// AddConfigExposedPort adds a port the set of ports to expose from a container running this image.
func (g *Generator) AddConfigExposedPort(port string) {
	g.image.Config.ExposedPorts[port] = struct{}{}
}

// RemoveConfigExposedPort removes a port the set of ports to expose from a container running this image.
func (g *Generator) RemoveConfigExposedPort(port string) {
	delete(g.image.Config.ExposedPorts, port)
}

// ConfigExposedPorts returns the set of ports to expose from a container running this image.
func (g *Generator) ConfigExposedPorts() map[string]struct{} {
	// We have to make a copy to preserve the privacy of g.image.Config.
	copy := map[string]struct{}{}
	for k, v := range g.image.Config.ExposedPorts {
		copy[k] = v
	}
	return copy
}

// ClearConfigEnv clears the list of environment variables to be used in a container.
func (g *Generator) ClearConfigEnv() {
	g.image.Config.Env = []string{}
}

// AddConfigEnv appends to the list of environment variables to be used in a container.
func (g *Generator) AddConfigEnv(env string) {
	g.image.Config.Env = append(g.image.Config.Env, env)
}

// ConfigEnv returns the list of environment variables to be used in a container.
func (g *Generator) ConfigEnv() []string {
	copy := []string{}
	for _, v := range g.image.Config.Env {
		copy = append(copy, v)
	}
	return copy
}

// SetConfigEntrypoint sets the list of arguments to use as the command to execute when the container starts.
func (g *Generator) SetConfigEntrypoint(entrypoint []string) {
	g.image.Config.Entrypoint = entrypoint
}

// ConfigEntrypoint returns the list of arguments to use as the command to execute when the container starts.
func (g *Generator) ConfigEntrypoint() []string {
	// We have to make a copy to preserve the privacy of g.image.Config.
	copy := []string{}
	for _, v := range g.image.Config.Entrypoint {
		copy = append(copy, v)
	}
	return copy
}

// SetConfigCmd sets the list of default arguments to the entrypoint of the container.
func (g *Generator) SetConfigCmd(entrypoint []string) {
	g.image.Config.Cmd = entrypoint
}

// ConfigCmd returns the list of default arguments to the entrypoint of the container.
func (g *Generator) ConfigCmd() []string {
	// We have to make a copy to preserve the privacy of g.image.Config.
	copy := []string{}
	for _, v := range g.image.Config.Cmd {
		copy = append(copy, v)
	}
	return copy
}

// ClearConfigVolumes clears the set of directories which should be created as data volumes in a container running this image.
func (g *Generator) ClearConfigVolumes() {
	g.image.Config.Volumes = map[string]struct{}{}
}

// AddConfigVolume adds a volume to the set of directories which should be created as data volumes in a container running this image.
func (g *Generator) AddConfigVolume(volume string) {
	g.image.Config.Volumes[volume] = struct{}{}
}

// RemoveConfigVolume removes a volume from the set of directories which should be created as data volumes in a container running this image.
func (g *Generator) RemoveConfigVolume(volume string) {
	delete(g.image.Config.Volumes, volume)
}

// ConfigVolumes returns the set of directories which should be created as data volumes in a container running this image.
func (g *Generator) ConfigVolumes() map[string]struct{} {
	// We have to make a copy to preserve the privacy of g.image.Config.
	copy := map[string]struct{}{}
	for k, v := range g.image.Config.Volumes {
		copy[k] = v
	}
	return copy
}

// SetConfigWorkingDir sets the current working directory of the entrypoint process in the container.
func (g *Generator) SetConfigWorkingDir(workingDir string) {
	g.image.Config.WorkingDir = workingDir
}

// ConfigWorkingDir returns the current working directory of the entrypoint process in the container.
func (g *Generator) ConfigWorkingDir() string {
	return g.image.Config.WorkingDir
}

// SetRootfsType sets the type of the rootfs.
func (g *Generator) SetRootfsType(rootfsType string) {
	g.image.RootFS.Type = rootfsType
}

// RootfsType returns the type of the rootfs.
func (g *Generator) RootfsType() string {
	return g.image.RootFS.Type
}

// ClearRootfsDiffIDs clears the array of layer content hashes (DiffIDs), in order from bottom-most to top-most.
func (g *Generator) ClearRootfsDiffIDs() {
	g.image.RootFS.DiffIDs = []string{}
}

// AddRootfsDiffID appends to the array of layer content hashes (DiffIDs), in order from bottom-most to top-most.
func (g *Generator) AddRootfsDiffID(diffid string) {
	g.image.RootFS.DiffIDs = append(g.image.RootFS.DiffIDs, diffid)
}

// RootfsDiffIDs returns the the array of layer content hashes (DiffIDs), in order from bottom-most to top-most.
func (g *Generator) RootfsDiffIDs() []string {
	copy := []string{}
	for _, v := range g.image.RootFS.DiffIDs {
		copy = append(copy, v)
	}
	return copy
}

// ClearHistory clears the history of each layer.
func (g *Generator) ClearHistory() {
	g.image.History = []v1.History{}
}

// AddHistory appends to the history of the layers.
func (g *Generator) AddHistory(history v1.History) {
	g.image.History = append(g.image.History, history)
}

// History returns the history of each layer.
func (g *Generator) History() []v1.History {
	copy := []v1.History{}
	for _, v := range g.image.History {
		copy = append(copy, v)
	}
	return copy
}

// ISO8601 represents the format of an ISO-8601 time string, which is identical
// to Go's RFC3339 specification.
const ISO8601 = time.RFC3339Nano

// SetCreated sets the combined date and time at which the image was created.
func (g *Generator) SetCreated(created time.Time) {
	g.image.Created = created.Format(ISO8601)
}

// Created gets the combined date and time at which the image was created.
func (g *Generator) Created() time.Time {
	created, err := time.Parse(ISO8601, g.image.Created)
	if err != nil {
		// FIXME
		panic(err)
	}
	return created
}

// SetAuthor sets the name and/or email address of the person or entity which created and is responsible for maintaining the image.
func (g *Generator) SetAuthor(author string) {
	g.image.Author = author
}

// Author returns the name and/or email address of the person or entity which created and is responsible for maintaining the image.
func (g *Generator) Author() string {
	return g.image.Author
}

// SetArchitecture is the CPU architecture which the binaries in this image are built to run on.
func (g *Generator) SetArchitecture(arch string) {
	g.image.Architecture = arch
}

// Architecture returns the CPU architecture which the binaries in this image are built to run on.
func (g *Generator) Architecture() string {
	return g.image.Architecture
}

// SetOS sets the name of the operating system which the image is built to run on.
func (g *Generator) SetOS(os string) {
	g.image.OS = os
}

// OS returns the name of the operating system which the image is built to run on.
func (g *Generator) OS() string {
	return g.image.OS
}
