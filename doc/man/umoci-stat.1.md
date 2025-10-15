% umoci-stat(1) # umoci stat - Display status information about an image tag
% Aleksa Sarai
% OCTOBER 2025
# NAME
umoci stat - Display status information about an image tag

# SYNOPSIS
**umoci stat**
**--image**=*image*[:*tag*]
[**--json**]

# DESCRIPTION
Generates various pieces of status information about an image tag, including
the history of the image.

**WARNING**: Do not depend on the output of this tool. Previously we
recommended the use of **--json** as the "stable" interface but this interface
will be reworked in future.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The OCI image tag to display information about. *image* must be a path to a
  valid OCI image and *tag* must be a valid tag in the image. If *tag* is not
  provided it defaults to "latest".

**--json**
  Output the status information as a JSON encoded blob.

# FORMAT
The format of the **--json** blob is as follows. Many of these fields come from
the [OCI image specification][1].

    {
      # This is the manifest blob of the image.
      "manifest": {
        "descriptor": <descriptor of manifest blob>,
        "blob": <raw manifest blob>
      },

      # This is the configuration blob of the image.
      "config": {
        "descriptor": <descriptor of config blob>,
        "blob": <raw config blob>
      },

      # This is the set of history entries for the image.
      "history": [
        {
          "layer":       <descriptor>, # null if empty_layer is true
          "diff_id":     <diffid>,
          "created":     <created>,
          "created_by":  <created_by>,
          "author":      <author>,
          "empty_layer": <empty_layer>
        }...
      ]
    }

In future versions of **umoci**(1) there may be extra fields added to the above
structure. However, the currently defined fields will always be set (until a
backwards-incompatible release is made).

# EXAMPLE

The following gets information about an image downloaded from a **docker**(1)
registry using **skopeo**(1).

```
% skopeo copy docker://registry.opensuse.org/opensuse/tumbleweed:latest oci:image:latest
% umoci stat --image image
== MANIFEST ==
Schema Version: 2
Media Type: application/vnd.oci.image.manifest.v1+json
Config:
        Descriptor:
                Media Type: application/vnd.oci.image.config.v1+json
                Digest: sha256:d06b5a49f4d4b8f5f39c1a6d798c7b2b1611747d5a1ee3bcfb5dc48d1b52f1e1
                Size: 2.175kB
Layers:
        Descriptor:
                Media Type: application/vnd.oci.image.layer.v1.tar+gzip
                Digest: sha256:34198bffb2664ac16017024a9f8e3e29e73efd98137894d033d41e308728ae56
                Size: 38.82MB
Descriptor:
        Media Type: application/vnd.oci.image.manifest.v1+json
        Digest: sha256:cc559b926ba5cccd0d95058279dd0120588576c64ba6667f2221bc554b9f0621
        Size: 407B
        Annotations:
                org.opencontainers.image.ref.name: latest

== CONFIG ==
Created: 2025-10-14T07:39:53Z
Author: "Fabian Vogt <fvogt@suse.com>"
Platform:
        OS: linux
        Architecture: amd64
Image Config:
        User: ""
        Command:
                /bin/bash
        Labels:
                org.openbuildservice.disturl: obs://build.opensuse.org/openSUSE:Factory/images/06574e385a3d2455c90f69f65ffa3e80-opensuse-tumbleweed-image:docker
                org.opencontainers.image.created: 2025-10-14T07:39:47.127067859Z
                org.opencontainers.image.description: "Image containing a minimal environment for containers based on openSUSE Tumbleweed."
                org.opencontainers.image.source: https://build.opensuse.org/package/show/openSUSE:Factory/opensuse-tumbleweed-image?rev=06574e385a3d2455c90f69f65ffa3e80
                org.opencontainers.image.title: "openSUSE Tumbleweed Base Container"
                org.opencontainers.image.url: https://www.opensuse.org
                org.opencontainers.image.vendor: "openSUSE Project"
                org.opencontainers.image.version: 20251013.34.298
                org.opensuse.base.created: 2025-10-14T07:39:47.127067859Z
                org.opensuse.base.description: "Image containing a minimal environment for containers based on openSUSE Tumbleweed."
                org.opensuse.base.disturl: obs://build.opensuse.org/openSUSE:Factory/images/06574e385a3d2455c90f69f65ffa3e80-opensuse-tumbleweed-image:docker
                org.opensuse.base.lifecycle-url: https://en.opensuse.org/Lifetime
                org.opensuse.base.reference: registry.opensuse.org/opensuse/tumbleweed:20251013.34.298
                org.opensuse.base.source: https://build.opensuse.org/package/show/openSUSE:Factory/opensuse-tumbleweed-image?rev=06574e385a3d2455c90f69f65ffa3e80
                org.opensuse.base.title: "openSUSE Tumbleweed Base Container"
                org.opensuse.base.url: https://www.opensuse.org
                org.opensuse.base.vendor: "openSUSE Project"
                org.opensuse.base.version: 20251013.34.298
                org.opensuse.lifecycle-url: https://en.opensuse.org/Lifetime
                org.opensuse.reference: registry.opensuse.org/opensuse/tumbleweed:20251013.34.298
Descriptor:
        Media Type: application/vnd.oci.image.config.v1+json
        Digest: sha256:d06b5a49f4d4b8f5f39c1a6d798c7b2b1611747d5a1ee3bcfb5dc48d1b52f1e1
        Size: 2.175kB

== HISTORY ==
LAYER                                                                   CREATED              CREATED BY   SIZE    COMMENT
sha256:34198bffb2664ac16017024a9f8e3e29e73efd98137894d033d41e308728ae56 2025-10-14T07:39:53Z KIWI 10.2.32 38.82MB openSUSE Tumbleweed 20251013 Base Container
```

# SEE ALSO
**umoci**(1)

[1]: https://github.com/opencontainers/image-spec
