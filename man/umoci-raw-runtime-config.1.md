% umoci-raw-runtime-config(1) # umoci raw runtime-config - Generate an OCI runtime configuration for an image
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci raw runtime-config - Generate an OCI runtime configuration for an image

# SYNOPSIS
**umoci raw runtime-config**
**--image**=*image*[:*tag*]
[**--rootfs**=*rootfs*]
[**--rootless**]
*config*

**umoci raw config**
**--image**=*image*[:*tag*]
[**--rootfs**=*rootfs*]
[**--rootless**]
*config*

# DESCRIPTION
Generate a new OCI runtime configuration from an image, without extracting the
rootfs of said image. The configuration is written to the path given by
*config*, overwriting it if it exists already. This is one of the operations
done by **umoci-unpack**(1) when generating the runtime bundle, but because of
the overhead of extracting a root filesystem, **umoci-unpack**(1) is not
practical to be used many times if the user doesn't actually want to use the
root filesystem. Some fields require a root filesystem as a "source of truth",
and a source root filesystem can be specified using **--rootfs**. The other
flags have the same effects as with **umoci-unpack**(1).

Note however that the output of **umoci-raw-runtime-config**(1) is not
necessarily identical to the output from **umoci-unpack**(1). This is
especially true if **--rootfs** is not specified, which results in **umoci**(1)
leaving fields in the runtime spec to their defaults if computing their values
would require using the root filesystem as a source-of-truth.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The OCI image tag which will be extracted to the *bundle*. *image* must be a
  path to a valid OCI image and *tag* must be a valid tag in the image. If
  *tag* is not provided it defaults to "latest".

**--rootfs**=*rootfs*
  Use *rootfs* as a secondary source of truth when generating the runtime
  configuration (this is especially important for *Config.User* conversion). If
  unspecified, any runtime fields that require a secondary source of truth to
  be filled with be left in their default values. This may result in
  discrepancies between the output of **umoci-unpack**(1) and
  **umoci-raw-runtime-config**(1).

**--rootless**
  Generate a rootless container configuration, similar to the configuration
  produced by **umoci-unpack**(1) when provided the **--rootless** flag.

# EXAMPLE
The following downloads an image from a **docker**(1) registry using
**skopeo**(1) and then generates the *config.json* for that image.

```
% skopeo copy docker://opensuse/amd64:42.2 oci:image:latest
% umoci raw runtime-config --image image:latest config.json
```

If a root filesystem is already present, it is possible to specify it with the
**--rootfs** flag. This will source the root filesystem for conversion
operations that necessitate it.

```
% skopeo copy docker://opensuse/amd64:42.2 oci:image:latest
# umoci unpack --image image bundle
% umoci raw runtime-generate --image image --rootfs bundle/rootfs config.json
```

# SEE ALSO
**umoci**(1), **umoci-repack**(1), **runc**(8)
