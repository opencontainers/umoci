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
	"strings"

	"github.com/apex/log"
	"github.com/blang/semver/v4"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"golang.org/x/sys/unix"
)

// FIXME: We currently use an unreleased version of the runtime-spec and so we
// have to modify the version string because OCI specifications use "-dev" as
// suffix for not-yet-released versions but in such a way that it produces
// incorrect behaviour. This is compounded with the fact that runtime-tools
// cannot handle any version other than the single version they were compiled
// with.
//
// For instance, 1.0.2-dev is the development version after the release of
// 1.0.2, but according to SemVer 1.0.2-dev should be considered older than
// 1.0.2 (it has a pre-release tag) -- the specs should be using 1.0.2+dev.
var curSpecVersion = semver.MustParse(strings.TrimSuffix(rspec.Version, "-dev"))

// Example returns an example spec file, used as a "good sane default".
// XXX: Really we should just use runc's directly.
func Example() rspec.Spec {
	return rspec.Spec{
		Version: curSpecVersion.String(),
		Root: &rspec.Root{
			Path:     "rootfs",
			Readonly: false,
		},
		Process: &rspec.Process{
			Terminal: true,
			User:     rspec.User{},
			Args: []string{
				"sh",
			},
			Env: []string{
				"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
				"TERM=xterm",
			},
			Cwd:             "/",
			NoNewPrivileges: true,
			Capabilities: &rspec.LinuxCapabilities{
				Bounding: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Permitted: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Inheritable: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Ambient: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
				Effective: []string{
					"CAP_AUDIT_WRITE",
					"CAP_KILL",
					"CAP_NET_BIND_SERVICE",
				},
			},
			Rlimits: []rspec.POSIXRlimit{
				{
					Type: "RLIMIT_NOFILE",
					Hard: uint64(1024),
					Soft: uint64(1024),
				},
			},
		},
		Hostname: "umoci-default",
		Mounts: []rspec.Mount{
			{
				Destination: "/proc",
				Type:        "proc",
				Source:      "proc",
				Options:     nil,
			},
			{
				Destination: "/dev",
				Type:        "tmpfs",
				Source:      "tmpfs",
				Options:     []string{"nosuid", "strictatime", "mode=755", "size=65536k"},
			},
			{
				Destination: "/dev/pts",
				Type:        "devpts",
				Source:      "devpts",
				Options:     []string{"nosuid", "noexec", "newinstance", "ptmxmode=0666", "mode=0620", "gid=5"},
			},
			{
				Destination: "/dev/shm",
				Type:        "tmpfs",
				Source:      "shm",
				Options:     []string{"nosuid", "noexec", "nodev", "mode=1777", "size=65536k"},
			},
			{
				Destination: "/dev/mqueue",
				Type:        "mqueue",
				Source:      "mqueue",
				Options:     []string{"nosuid", "noexec", "nodev"},
			},
			{
				Destination: "/sys",
				Type:        "sysfs",
				Source:      "sysfs",
				Options:     []string{"nosuid", "noexec", "nodev", "ro"},
			},
			{
				Destination: "/sys/fs/cgroup",
				Type:        "cgroup",
				Source:      "cgroup",
				Options:     []string{"nosuid", "noexec", "nodev", "relatime", "ro"},
			},
		},
		Linux: &rspec.Linux{
			MaskedPaths: []string{
				"/proc/kcore",
				"/proc/latency_stats",
				"/proc/timer_list",
				"/proc/timer_stats",
				"/proc/sched_debug",
				"/sys/firmware",
				"/proc/scsi",
			},
			ReadonlyPaths: []string{
				"/proc/asound",
				"/proc/bus",
				"/proc/fs",
				"/proc/irq",
				"/proc/sys",
				"/proc/sysrq-trigger",
			},
			Resources: &rspec.LinuxResources{
				Devices: []rspec.LinuxDeviceCgroup{
					{
						Allow:  false,
						Access: "rwm",
					},
				},
			},
			Namespaces: []rspec.LinuxNamespace{
				{
					Type: "cgroup",
				},
				{
					Type: "pid",
				},
				{
					Type: "network",
				},
				{
					Type: "ipc",
				},
				{
					Type: "uts",
				},
				{
					Type: "mount",
				},
			},
		},
	}
}

// ToRootless converts a specification to a version that works with rootless
// containers. This is done by removing options and other settings that clash
// with unprivileged user namespaces.
func ToRootless(spec *rspec.Spec) error {
	// Remove additional groups.
	spec.Process.User.AdditionalGids = nil

	// Remove networkns from the spec, as well as userns (for us to add it
	// later without duplicates).
	namespaces := make([]rspec.LinuxNamespace, 0, len(spec.Linux.Namespaces))
	for _, ns := range spec.Linux.Namespaces {
		if ns.Type == rspec.NetworkNamespace || ns.Type == rspec.UserNamespace {
			continue
		}
		namespaces = append(namespaces, ns)
	}
	// Add userns to the spec.
	namespaces = append(namespaces, rspec.LinuxNamespace{
		Type: rspec.UserNamespace,
	})
	spec.Linux.Namespaces = namespaces

	// Fix up mounts.
	mounts := make([]rspec.Mount, 0, len(spec.Mounts))
	for _, mount := range spec.Mounts {
		// Ignore all mounts that are under /sys.
		if strings.HasPrefix(mount.Destination, "/sys") {
			continue
		}

		// Remove all gid= and uid= mappings.
		options := make([]string, 0, len(mount.Options))
		for _, option := range mount.Options {
			if !strings.HasPrefix(option, "gid=") && !strings.HasPrefix(option, "uid=") {
				options = append(options, option)
			}
		}

		mount.Options = options
		mounts = append(mounts, mount)
	}
	// Add the sysfs mount as an rbind.
	mounts = append(mounts, rspec.Mount{
		// NOTE: "type: bind" is silly here, see opencontainers/runc#2035.
		Type:        "bind",
		Source:      "/sys",
		Destination: "/sys",
		Options:     []string{"rbind", "nosuid", "noexec", "nodev", "ro"},
	})
	// Add /etc/resolv.conf as an rbind.
	const resolvConf = "/etc/resolv.conf"
	if err := unix.Access(resolvConf, unix.F_OK); err != nil {
		// If /etc/resolv.conf doesn't exist (such as inside OBS), just log a
		// warning and continue on. In the worst case, you'll just end up with
		// a non-networked container.
		log.Warnf("rootless configuration: automatic bind-mount for %q cannot be added as the source doesn't exist", resolvConf)
	} else {
		// If we are using user namespaces, then we must make sure that we don't
		// drop any of the CL_UNPRIVILEGED "locked" flags of the source "mount"
		// when we bind-mount. The reason for this is that at the point when runc
		// sets up the root filesystem, it is already inside a user namespace, and
		// thus cannot change any flags that are locked.
		unprivOpts, err := getUnprivilegedMountFlags(resolvConf)
		if err != nil {
			return fmt.Errorf("inspecting mount flags of %s: %w", resolvConf, err)
		}
		mounts = append(mounts, rspec.Mount{
			// NOTE: "type: bind" is silly here, see opencontainers/runc#2035.
			Type:        "bind",
			Destination: resolvConf,
			Source:      resolvConf,
			Options:     append(unprivOpts, []string{"rbind", "ro"}...),
		})
		spec.Mounts = mounts
	}

	// Remove cgroup settings.
	spec.Linux.Resources = nil
	return nil
}
