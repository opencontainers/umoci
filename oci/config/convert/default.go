/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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
	"strings"

	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/pkg/errors"
)

// Example returns an example spec file, used as a "good sane default".
// XXX: Really we should just use runc's directly.
func Example() rspec.Spec {
	return rspec.Spec{
		Version: rspec.Version,
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
	var namespaces []rspec.LinuxNamespace

	// Remove additional groups.
	spec.Process.User.AdditionalGids = nil

	// Remove networkns from the spec.
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case rspec.NetworkNamespace, rspec.UserNamespace:
			// Do nothing.
		default:
			namespaces = append(namespaces, ns)
		}
	}
	// Add userns to the spec.
	namespaces = append(namespaces, rspec.LinuxNamespace{
		Type: rspec.UserNamespace,
	})
	spec.Linux.Namespaces = namespaces

	// Fix up mounts.
	var mounts []rspec.Mount
	for _, mount := range spec.Mounts {
		// Ignore all mounts that are under /sys.
		if strings.HasPrefix(mount.Destination, "/sys") {
			continue
		}

		// Remove all gid= and uid= mappings.
		var options []string
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
	// If we are using user namespaces, then we must make sure that we don't
	// drop any of the CL_UNPRIVILEGED "locked" flags of the source "mount"
	// when we bind-mount. The reason for this is that at the point when runc
	// sets up the root filesystem, it is already inside a user namespace, and
	// thus cannot change any flags that are locked.
	unprivOpts, err := getUnprivilegedMountFlags(resolvConf)
	if err != nil {
		return errors.Wrapf(err, "inspecting mount flags of %s", resolvConf)
	}
	mounts = append(mounts, rspec.Mount{
		// NOTE: "type: bind" is silly here, see opencontainers/runc#2035.
		Type:        "bind",
		Destination: resolvConf,
		Source:      resolvConf,
		Options:     append(unprivOpts, []string{"rbind", "ro"}...),
	})
	spec.Mounts = mounts

	// Remove cgroup settings.
	spec.Linux.Resources = nil
	return nil
}
