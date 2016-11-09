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
	"strconv"
	"strings"

	"github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rgen "github.com/opencontainers/runtime-tools/generate"
)

// ToRuntimeSpec converts the given OCI image configuration to a runtime
// configuration appropriate for use, which is templated on the default
// configuration specified by the OCI runtime-tools. It is equivalent to
// MutateRuntimeSpec("runtime-tools/generate".New(), image).Spec().
func ToRuntimeSpec(image v1.Image) rspec.Spec {
	g := rgen.New()
	MutateRuntimeSpec(g, image)
	return *g.Spec()
}

// MutateRuntimeSpec mutates a given runtime specification generator with the
// image configuration provided. It returns the original generator, and does
// not modify any fields directly (to allow for chaining).
func MutateRuntimeSpec(g rgen.Generator, image v1.Image) rgen.Generator {
	if image.OS != "linux" {
		panic("unsupported OS")
	}

	// FIXME: We need to figure out if we're modifying an incompatible runtime spec.
	//g.SetVersion(rspec.Version)
	g.SetPlatformOS(image.OS)
	g.SetPlatformArch(image.Architecture)

	g.SetProcessTerminal(true)
	g.SetRootPath("rootfs") // XXX: Should be configurable.
	g.SetRootReadonly(false)

	g.SetProcessCwd("/")
	if image.Config.WorkingDir != "" {
		g.SetProcessCwd(image.Config.WorkingDir)
	}

	for _, env := range image.Config.Env {
		g.AddProcessEnv(env)
	}

	// We don't append to g.Spec().Process.Args because the default is non-zero.
	// FIXME: Should we make this instead only append to the pre-existing args
	//        if Entrypoint != ""? I'm not really sure (Docker doesn't).
	args := []string{}
	args = append(args, image.Config.Entrypoint...)
	args = append(args, image.Config.Cmd...)
	if len(args) == 0 {
		args = []string{"sh"}
	}
	g.SetProcessArgs(args)

	if uid, err := strconv.Atoi(image.Config.User); err == nil {
		g.SetProcessUID(uint32(uid))
	} else if ug := strings.Split(image.Config.User, ":"); len(ug) == 2 {
		uid, err := strconv.Atoi(ug[0])
		if err != nil {
			panic("config.User: unsupported uid format")
		}

		gid, err := strconv.Atoi(ug[1])
		if err != nil {
			panic("config.User: unsupported gid format")
		}

		g.SetProcessUID(uint32(uid))
		g.SetProcessGID(uint32(gid))
	} else if image.Config.User != "" {
		panic("config.User: unsupported format")
	}

	// TODO: Handle cases where these are unset properly.
	g.SetLinuxResourcesCPUShares(uint64(image.Config.CPUShares))
	g.SetLinuxResourcesMemoryLimit(uint64(image.Config.Memory))
	g.SetLinuxResourcesMemoryReservation(uint64(image.Config.Memory))
	g.SetLinuxResourcesMemorySwap(uint64(image.Config.MemorySwap))

	for vol := range image.Config.Volumes {
		// XXX: Is it fine to generate source=""?
		g.AddBindMount("", vol, []string{"rw", "rbind"})
	}

	return g
}
