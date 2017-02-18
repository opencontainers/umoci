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

package convert

import (
	"path/filepath"
	"strings"

	"github.com/openSUSE/umoci/third_party/user"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rgen "github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
)

// ToRuntimeSpec converts the given OCI image configuration to a runtime
// configuration appropriate for use, which is templated on the default
// configuration specified by the OCI runtime-tools. It is equivalent to
// MutateRuntimeSpec("runtime-tools/generate".New(), image).Spec().
func ToRuntimeSpec(rootfs string, image ispec.Image, manifest ispec.Manifest) (rspec.Spec, error) {
	g := rgen.New()
	if err := MutateRuntimeSpec(g, rootfs, image, manifest); err != nil {
		return rspec.Spec{}, err
	}
	return *g.Spec(), nil
}

// parseEnv splits a given environment variable (of the form name=value) into
// (name, value). An error is returned if there is no "=" in the line or if the
// name is empty.
func parseEnv(env string) (string, string, error) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return "", "", errors.Errorf("environment variable must contain '=': %s", env)
	}

	name, value := parts[0], parts[1]
	if name == "" {
		return "", "", errors.Errorf("environment variable must have non-empty name: %s", env)
	}
	return name, value, nil
}

// MutateRuntimeSpec mutates a given runtime specification generator with the
// image configuration provided. It returns the original generator, and does
// not modify any fields directly (to allow for chaining).
func MutateRuntimeSpec(g rgen.Generator, rootfs string, image ispec.Image, manifest ispec.Manifest) error {
	if image.OS != "linux" {
		return errors.Errorf("unsupported OS: %s", image.OS)
	}

	// FIXME: We need to figure out if we're modifying an incompatible runtime spec.
	//g.SetVersion(rspec.Version)
	g.SetPlatformOS(image.OS)
	g.SetPlatformArch(image.Architecture)

	g.SetProcessTerminal(true)
	g.SetRootPath(filepath.Base(rootfs))
	g.SetRootReadonly(false)

	g.SetProcessCwd("/")
	if image.Config.WorkingDir != "" {
		g.SetProcessCwd(image.Config.WorkingDir)
	}

	g.ClearProcessEnv()
	for _, env := range image.Config.Env {
		name, value, err := parseEnv(env)
		if err != nil {
			return errors.Wrap(err, "parsing image.Config.Env")
		}
		g.AddProcessEnv(name, value)
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

	// Get the *actual* uid and gid of the user. If the image doesn't contain
	// an /etc/passwd or /etc/group file then GetExecUserPath will just do a
	// numerical parsing.
	var passwdPath, groupPath string
	if rootfs != "" {
		passwdPath = filepath.Join(rootfs, "/etc/passwd")
		groupPath = filepath.Join(rootfs, "/etc/group")
	}
	execUser, err := user.GetExecUserPath(image.Config.User, nil, passwdPath, groupPath)
	if err != nil {
		return errors.Wrapf(err, "cannot parse user spec: '%s'", image.Config.User)
	}

	g.SetProcessUID(uint32(execUser.Uid))
	g.SetProcessGID(uint32(execUser.Gid))
	g.ClearProcessAdditionalGids()
	for _, gid := range execUser.Sgids {
		g.AddProcessAdditionalGid(uint32(gid))
	}
	if execUser.Home != "" {
		g.AddProcessEnv("HOME", execUser.Home)
	}

	for vol := range image.Config.Volumes {
		// XXX: This is _fine_ but might cause some issues in the future.
		g.AddTmpfsMount(vol, []string{"rw"})
	}

	// XXX: This order-of-addition is actually not codified in the spec.
	//      However, this will be sorted once I write a proposal for it.
	//      opencontainers/image-spec#479

	g.ClearAnnotations()
	for key, value := range image.Config.Labels {
		g.AddAnnotation(key, value)
	}
	for key, value := range manifest.Annotations {
		g.AddAnnotation(key, value)
	}

	return nil
}
