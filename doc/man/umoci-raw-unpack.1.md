% umoci-raw-unpack(1) # umoci raw unpack - Unpacks an OCI image tag into a root filesystem
% Aleksa Sarai
% SEPTEMBER 2018
# NAME
umoci raw unpack - Unpacks an OCI image tag into a root filesystem

# SYNOPSIS
**umoci raw unpack**
[*umoci-unpack(1) flags*]
*rootfs*

# DESCRIPTION
Extracts all of the layers (deterministically) to a root filesystem at path
*rootfs*. This path *must not* already exist.

# OPTIONS
The global options are defined in **umoci**(1), while the options for this
particular subcommand are identical to **umoci-unpack**(1) with the exception
that the *rootfs* path is provided rather than a *bundle* path.

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
