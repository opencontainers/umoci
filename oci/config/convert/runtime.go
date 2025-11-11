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

package convert

import (
	"fmt"
	"maps"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/blang/semver/v4"
	"github.com/moby/sys/user"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"

	igen "github.com/opencontainers/umoci/oci/config/generate"
)

// Annotations described by the OCI image-spec document (these represent fields
// in an image configuration that do not have a native representation in the
// runtime-spec).
const (
	platformOsAnnotation      = "org.opencontainers.image.os"
	platformArchAnnotation    = "org.opencontainers.image.architecture"
	platformVariantAnnotation = "org.opencontainers.image.variant"
	authorAnnotation          = "org.opencontainers.image.author"
	createdAnnotation         = "org.opencontainers.image.created"
	stopSignalAnnotation      = "org.opencontainers.image.stopSignal"
	exposedPortsAnnotation    = "org.opencontainers.image.exposedPorts"
)

// ToRuntimeSpec converts the given OCI image configuration to a runtime
// configuration appropriate for use, which is templated on the default
// configuration specified by the OCI runtime-tools. It is equivalent to
// MutateRuntimeSpec("runtime-tools/generate".New(), image).Spec().
func ToRuntimeSpec(rootfs string, image ispec.Image) (rspec.Spec, error) {
	spec := Example()
	if err := MutateRuntimeSpec(&spec, rootfs, image); err != nil {
		return rspec.Spec{}, err
	}
	return spec, nil
}

// parseEnv splits a given environment variable (of the form name=value) into
// (name, value). An error is returned if there is no "=" in the line or if the
// name is empty.
func parseEnv(env string) (string, string, error) {
	parts := strings.SplitN(env, "=", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("environment variable must contain '=': %s", env)
	}

	name, value := parts[0], parts[1]
	if name == "" {
		return "", "", fmt.Errorf("environment variable must have non-empty name: %s", env)
	}
	return name, value, nil
}

// appendEnv takes a (name, value) pair and inserts it into the given
// environment list (overwriting an existing environment if already set).
func appendEnv(env *[]string, name, value string, clobber bool) {
	val := name + "=" + value
	for idx, oldVal := range *env {
		if strings.HasPrefix(oldVal, name+"=") {
			if clobber {
				(*env)[idx] = val
			}
			return
		}
	}
	*env = append(*env, val)
}

// allocateNilStruct recursively enumerates all pointers in the given type and
// replaces them with the zero-value of their associated type. It's a shame
// that this is necessary.
//
// TODO: Switch to doing this recursively with reflect.
func allocateNilStruct(spec *rspec.Spec) {
	if spec.Process == nil {
		spec.Process = &rspec.Process{}
	}
	if spec.Root == nil {
		spec.Root = &rspec.Root{}
	}
	if spec.Linux == nil {
		spec.Linux = &rspec.Linux{}
	}
	if spec.Annotations == nil {
		spec.Annotations = map[string]string{}
	}
}

// MutateRuntimeSpec mutates a given runtime configuration with the image
// configuration provided in accordance with the image specification's
// conversion mechanism (for more information, see
// <https://github.com/opencontainers/image-spec/blob/main/conversion.md>).
func MutateRuntimeSpec(spec *rspec.Spec, rootfs string, image ispec.Image) error {
	ig, err := igen.NewFromImage(image)
	if err != nil {
		return fmt.Errorf("creating image generator: %w", err)
	}

	if ig.PlatformOS() != "linux" {
		return fmt.Errorf("unsupported OS: %s", image.OS)
	}

	allocateNilStruct(spec)

	// Default config to our rspec version if none was specified.
	if spec.Version == "" {
		spec.Version = curSpecVersion.String()
	}

	// Make sure that the previous version of the spec is compatible with us.
	// We cannot operate on specifications that are newer than us (because we
	// might drop fields that the user finds important).
	oldVersion, err := semver.Parse(spec.Version)
	if err != nil {
		return fmt.Errorf("parsing original runtime-spec config version: %w", err)
	}
	if oldVersion.GT(curSpecVersion) {
		return fmt.Errorf("original runtime-spec config version %s is unsupported: %s > %s", oldVersion, oldVersion, curSpecVersion)
	}
	if oldVersion.Major != curSpecVersion.Major {
		return fmt.Errorf("original runtime-spec config version %s is incompatible with version %s: mismatching major number", oldVersion, curSpecVersion)
	}

	// Set verbatim fields
	spec.Process.Terminal = true
	spec.Root.Path = filepath.Base(rootfs)
	spec.Root.Readonly = false

	spec.Process.Cwd = "/"
	if ig.ConfigWorkingDir() != "" {
		spec.Process.Cwd = ig.ConfigWorkingDir()
	}

	for _, env := range ig.ConfigEnv() {
		name, value, err := parseEnv(env)
		if err != nil {
			return fmt.Errorf("parsing image.Config.Env: %w", err)
		}
		appendEnv(&spec.Process.Env, name, value, true)
	}

	args := []string{}
	args = append(args, ig.ConfigEntrypoint()...)
	args = append(args, ig.ConfigCmd()...)
	if len(args) > 0 {
		spec.Process.Args = args
	}

	// Set the "annotation fields".
	setAnnotation := func(name, value string) {
		if value != "" {
			spec.Annotations[name] = value
		} else {
			delete(spec.Annotations, name)
		}
	}
	setAnnotation(platformOsAnnotation, ig.PlatformOS())
	setAnnotation(platformArchAnnotation, ig.PlatformArchitecture())
	setAnnotation(platformVariantAnnotation, ig.PlatformVariant())
	setAnnotation(authorAnnotation, ig.Author())
	setAnnotation(createdAnnotation, ig.Created().Format(igen.ISO8601))
	setAnnotation(stopSignalAnnotation, image.Config.StopSignal)
	setAnnotation(exposedPortsAnnotation, strings.Join(ig.ConfigExposedPorts(), ","))
	// Config.Labels need to be applied after the auto-applied labels.
	maps.Copy(spec.Annotations, ig.ConfigLabels())

	// Set parsed fields
	// Get the *actual* uid and gid of the user. If the image doesn't contain
	// an /etc/passwd or /etc/group file then GetExecUserPath will just do a
	// numerical parsing.
	var passwdPath, groupPath string
	if rootfs != "" {
		passwdPath = filepath.Join(rootfs, "/etc/passwd")
		groupPath = filepath.Join(rootfs, "/etc/group")
	}
	execUser, err := user.GetExecUserPath(ig.ConfigUser(), nil, passwdPath, groupPath)
	if err != nil {
		// We only log an error if were not given a rootfs, and we set execUser
		// to the "default" (root:root).
		if rootfs != "" {
			return fmt.Errorf("cannot parse user spec: %q: %w", ig.ConfigUser(), err)
		}
		log.Warnf("could not parse user spec %q without a rootfs -- defaulting to root:root", ig.ConfigUser())
		execUser = new(user.ExecUser)
	}

	spec.Process.User.UID = uint32(execUser.Uid)
	spec.Process.User.GID = uint32(execUser.Gid)

	spec.Process.User.AdditionalGids = []uint32{}
	for _, sgid := range execUser.Sgids {
		spec.Process.User.AdditionalGids = append(spec.Process.User.AdditionalGids, uint32(sgid))
	}

	if execUser.Home != "" {
		appendEnv(&spec.Process.Env, "HOME", execUser.Home, false)
	}

	for _, vol := range ig.ConfigVolumes() {
		// XXX: This is _fine_ but might cause some issues in the future.
		spec.Mounts = append(spec.Mounts, rspec.Mount{
			Destination: vol,
			Type:        "tmpfs",
			Source:      "none",
			Options:     []string{"rw", "nosuid", "nodev", "noexec", "relatime"},
		})
	}

	// Remove all seccomp rules.
	spec.Linux.Seccomp = nil
	return nil
}
