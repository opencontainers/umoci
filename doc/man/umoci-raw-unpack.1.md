% umoci-raw-unpack(1) # umoci raw unpack - Unpacks an OCI image tag into a root filesystem
% Aleksa Sarai
% APRIL 2018
# NAME
umoci raw unpack - Unpacks an OCI image tag into a root filesystem

# SYNOPSIS
**umoci raw unpack**
**--image**=*image*[:*tag*]
**--keep-dirlinks**
*rootfs*

# DESCRIPTION
Extracts all of the layers (deterministically) to a root filesystem at path
*rootfs*. This path must not already exist.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The OCI image tag which will be extracted to the *rootfs*. *image* must be a
  path to a valid OCI image and *tag* must be a valid tag in the image. If
  *tag* is not provided it defaults to "latest".

**--uid-map**=[*value*]
  Specifies a UID mapping to use while unpacking layers. This is used in a
  similar fashion to **user_namespaces**(7), and is of the form
  **container:host[:size]**.

**--gid-map**=[*value*]
  Specifies a GID mapping to use while unpacking layers. This is used in a
  similar fashion to **user_namespaces**(7), and is of the form
  **container:host[:size]**.

**--rootless**
  Enable rootless unpacking support. This allows for **umoci-raw-unpack**(1) to
  be used as an unprivileged user. Use of this flag implies **--uid-map=0:$(id
  -u):1** and **--gid-map=0:$(id -g):1**, as well as enabling several features
  to fake parts of the unpacking in the attempt to generate an
  as-close-as-possible extraction of the filesystem. Note that it is almost
  always not possible to perfectly extract an OCI image with **--rootless**,
  but it will be as close as possible.

**--keep-dirlinks**
  Instead of overwriting directories which are links to other directories when
  higher layers have an explicit directory, just write through the symlink.
  This option is inspired by rsync's option of the same name.

# EXAMPLE
The following downloads an image from a **docker**(1) registry using
**skopeo**(1), unpacks said image to a root filesystem, generates an OCI
runtime configuration file with **umoci-raw-runtime-config**(1) and then
creates a new container with **runc**(8).

```
% skopeo copy docker://opensuse/amd64:42.2 oci:image:latest
# umoci raw unpack --image image rootfs
# umoci raw runtime-config --image image --rootfs rootfs config.json
# runc run ctr
[ container session ]
```

With **--rootless** it is also possible to do the above example without root
privileges. **umoci** will generate a configuration that works with rootless
containers in **runc**(8).

```
% skopeo copy docker://opensuse/amd64:42.2 oci:image:latest
% umoci raw unpack --image image --rootless rootfs
% umoci raw runtime-config --image image --rootfs rootfs --rootless config.json
% runc --root $HOME/runc run ctr
[ rootless container session ]
```

# SEE ALSO
**umoci**(1), **umoci-raw-runtime-config**(1), **runc**(8)
