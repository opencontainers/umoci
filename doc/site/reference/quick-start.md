+++
title = "Quick Start Guide"
weight = 10
+++

This document gives you a very quick insight in how to use `umoci`, as well as
some idea about where you can get an OCI image from and how you can use it. If
you feel there is something missing from this document, feel free to
[contribute][contributing.md].

The following is loosely based on my [original announcement blog post][blog],
but more focused on usage of `umoci` as well as remaining more up-to-date than
the blog post.

If you're on [openSUSE][opensuse], then all of the tools linked in this
document should be available in the official repositories.

[contributing.md]: /CONTRIBUTING.md
[blog]: https://www.cyphar.com/blog/post/umoci-new-oci-image-tool
[opensuse]: https://www.opensuse.org/

### Getting an Image ###

At the time of writing, there is no standard way of getting an OCI image.
Distribution is still an open topic in the specification, and there are very
few implementations of a distribution extension to the OCI specification. I've
[personally worked on one][parcel] but there is still a lot of work to go
before you can skip this step and get OCI images without the need to convert
from other things.

In order to get an OCI image, you need to convert it from another container
image format. Luckily, as the OCI spec was based on the Docker image format,
there is no loss of information when converting between the two formats.
[`skopeo`][skopeo] is an incredibly useful tool that allows you to fetch and
convert a Docker image (from a registry, local daemon or even from a file saved
with `docker save`) to an OCI image (and vice-versa). Read their documentation
for more information about the various other formats they support.

After getting `skopeo`, you can download an image as follows. Note that you can
include multiple Docker images inside the same OCI image (under different
"tags").

```
% skopeo copy docker://opensuse/amd64:42.2 oci:opensuse:42.2
Getting image source signatures
Copying blob sha256:f65b94255373e4dc9645fcb551756b87726a1c891fe6c89f6bbbc864ff845c15
 46.59 MB / 46.59 MB [=========================================================]
Copying config sha256:5af572844af6ae4122721ba6bfa11b4048dc4535a9f52772e809a68cac4e9244
 0 B / 805 B [-----------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```
```
% skopeo copy docker://opensuse/amd64:42.1 oci:opensuse:old_42.1
Getting image source signatures
Copying blob sha256:d9e29ed5a74f21e153b05ecc646fe1157fcfa991c9661759986191408665521b
 36.60 MB / 36.60 MB [=========================================================]
Copying config sha256:1652ed016d569d50729738e2f4ab3564f7375a25150c4a1ac1cc6687e586a5ce
 0 B / 805 B [-----------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```

At this point, you have a directory called `opensuse` which is the downloaded
OCI image stored as a directory. This is currently the only layout that `umoci`
can interact with.

[parcel]: https://github.com/cyphar/parcel
[skopeo]: https://github.com/projectatomic/skopeo

### Unpack ###

Each image consists of a set of layers and a configuration that describes how
the image should be used. `umoci unpack` allows you to take an image and
extract its root filesystem and configuration into an [OCI runtime
bundle][oci-runtime]. This bundle can be used by an OCI compliant container
runtime to spawn a container, but also can be used directly by non-containers
(as it is just a directory).

[oci-runtime]: https://github.com/opencontainers/runtime-spec

```
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

You can spawn new containers with [`runc`][runc]. If you make any changes to
the root filesystem, you can create a new delta layer and add it to the image
by using [`umoci repack`](#repack).

[runc]: https://github.com/opencontainers/runc

```
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

```
% echo "some change" > bundle/rootfs/my_change
% sudo umoci repack --image opensuse:new bundle
% sudo umoci unpack --image opensuse:new bundle2
% cat bundle2/rootfs/my_change
some change
```

Note that any changes to `bundle/config.json` **will not** change the image's
configuration. You can change an image's configuration using [`umoci
config`](#configuration).

### Configuration ###

In order to change the configuration of an image, `umoci config` can be used. A
full description of what each configuration option means is beyond the scope of
this document, but you can [read the spec for more
information][image-spec-config].

By default, `umoci config` will override the tag given with `--image` but you
can force the change to create a new tag (leaving the original unchanged) with
`--tag`.

```
% umoci config --author="Aleksa Sarai <asarai@suse.de>" --image opensuse
```

Note that both `umoci config` and `umoci repack` include entries in the history
of the image. You can change what the history entry contains for a particular
operation by using the `--history.` set of flags (for both `umoci repack` and
`umoci config`).

[image-spec-config]: https://github.com/opencontainers/image-spec/blob/v1.0.0-rc4/config.md

### Creating New Images ###

Creating a new image with `umoci` is fairly simple, and effectively involves
creating an image "husk" which you can then operate on as though it was a
normal image.

If you wish to create a new image layout (which contains nothing except the
bare minimum to specify that the image is an OCI image), you can do so `umoci
init`.

```
% umoci init --layout new_image
```

If you wish to create a new image inside the image layout, you can do so with
`umoci new`. Note that the resulting image is effectively empty. It has an
empty root filesystem, an empty configuration and so on.

```
% umoci new --image new_image:new_tag
```

After you have an empty image, you can unpack it with `umoci unpack` and
operate on it as usual.

```
% sudo umoci unpack --image new_image:new_tag new_bundle
% ls -la new_bundle/rootfs
total 0
drwxr-xr-x 1 root root   0 Jan  1  1970 .
drwxr-xr-x 1 root root 208 Jul  3 18:25 ..
```

### Garbage Collection ###

Every `umoci` operation that modifes an image will not delete any now-unused
blobs in the image (so as to ensure that any other operations that assume those
blobs are present will not error out). However, this will result in a large
number of useless blobs remaining in the image after operating on an image for
a long enough period of time. `umoci gc` will garbage collect all blobs that
are not reachable from any known image tag. Note that calling `umoci gc`
between `umoci unpack` and `umoci repack` may result in errors if you've
removed all references to the blobs used by `umoci unpack`.

```
% umoci gc --layout opensuse
```

### Rootless Containers ###

`umoci` has first class support for [rootless containers][rootlesscontaine.rs],
and in particular it supports rootless unpacking. This means that an
unprivileged user can unpack and repack and image (which is not traditionally
possible for most images), as well as generate a runtime configuration that can
be used by `runc` to start a rootless container.

[rootlesscontaine.rs]: https://rootlesscontaine.rs/
