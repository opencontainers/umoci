+++
title = "Workflow"
weight = 20
+++

umoci's workflow is based around an unpack-repack cycle, with some separate
configuration steps. Most users are going to be primarily using the unpack and
repack subcommands for most uses of umoci.

### Unpack ###

Each image consists of a set of layers and a configuration that describes how
the image should be used. `umoci unpack` allows you to take an image and
extract its root filesystem and configuration into an [runtime
bundle][oci-runtime]. This bundle can be used by an OCI compliant container
runtime to spawn a container, but also can be used directly by non-containers
(as it is just a directory).

[oci-runtime]: https://github.com/opencontainers/runtime-spec

```text
% sudo umoci unpack --image opensuse:42.2 bundle
% ls -l bundle
total 720
-rw-r--r-- 1 root root   3247 Jul  3 17:58 config.json
drwxr-xr-x 1 root root    128 Jan  1  1970 rootfs
-rw-r--r-- 1 root root 725320 Jul  3 17:58 sha256_8eac95fae2d9d0144607ffde0248b2eb46556318dcce7a9e4cc92edcd2100b67.mtree
-rw-r--r-- 1 root root    270 Jul  3 17:58 umoci.json
% cat bundle/rootfs/etc/os-release
NAME="openSUSE Leap"
VERSION="42.2"
ID=opensuse
ID_LIKE="suse"
VERSION_ID="42.2"
PRETTY_NAME="openSUSE Leap 42.2"
ANSI_COLOR="0;32"
CPE_NAME="cpe:/o:opensuse:leap:42.2"
BUG_REPORT_URL="https://bugs.opensuse.org"
HOME_URL="https://www.opensuse.org/"
```

You can spawn new containers with [runc][runc]. If you make any changes to
the root filesystem, you can create a new delta layer and add it to the image
by [repacking it](#repack).

[runc]: https://github.com/opencontainers/runc

```text
% sudo runc run -b bundle ctr-name
sh-4.3# grep NAME /etc/os-release
NAME="openSUSE Leap"
sh-4.3# uname -n
mrsdalloway
```

### Repack ###

After making some changes to the root filesystem of your extracted image, you
may want to create a new delta layer from the changes. Note that the way you
modified the image does not matter, you can create a container using the
extracted root filesystem or just modify it directly.

`umoci repack` will create a derived image based on the image originally
extracted to create the runtime bundle. Even if the original image
has been "modified", `umoci repack` will still use the original. Note that in
this invocation, `--image` refers to the new tag that will be used to reference
the modified image (which may be the same tag used to extract the original
image). `umoci repack` does not work across different images -- both the source
and destination must be in the same image (and the original blobs must not have
been garbage collected).

```text
% echo "some change" > bundle/rootfs/my_change
% sudo umoci repack --image opensuse:new bundle
% sudo umoci unpack --image opensuse:new bundle2
% cat bundle2/rootfs/my_change
some change
```

Note that any changes to `bundle/config.json` **will not** change the image's
configuration. You can change an image's configuration using [the dedicated
subcommand](#configuration).

### Configuration ###

In order to change the configuration of an image, `umoci config` can be used. A
full description of what each configuration option means is beyond the scope of
this document, but you can [read the spec for more
information][image-spec-config].

By default, `umoci config` will override the tag given with `--image` but you
can force the change to create a new tag (leaving the original unchanged) with
`--tag`.

```text
% umoci config --author="Aleksa Sarai <asarai@suse.de>" --image opensuse
```

Note that both `umoci config` and `umoci repack` include entries in the history
of the image. You can change what the history entry contains for a particular
operation by using the `--history.` set of flags (for both `umoci repack` and
`umoci config`).

[image-spec-config]: https://github.com/opencontainers/image-spec/blob/v1.0.0-rc4/config.md
