+++
title = "Home"
+++

## umoci ##

Made with &#10084; by [openSUSE][openSUSE].

umoci is a free software tool for manipulating and interacting with container
images in the standardised [Open Container Initiative's][oci] image format. It
provides one of the most flexible image management toolsets, requiring neither
a daemon nor any particular filesystem setup. It is already used in a variety
of different projects and by several companies.

{{% notice tip %}}
umoci is currently in desperate need of a logo design. If you're interested in
creating a logo for us (and releasing it under a <a
href="https://creativecommons.org/">free artwork license</a>) then don't
hesitate to reach out!
{{% /notice %}}

[openSUSE]: https://opensuse.org/
[oci]: https://www.opencontainers.org/
[creative-commons]: https://creativecommons.org/

### Features ###

umoci's feature set is intentionally restricted, as it has well-defined goals.

* Extraction of images produces a standardised [OCI runtime
  bundle][oci-runtime], which is immediately usable by [runc][runc] or [any
  other OCI-compliant runtime][oci-runtimes]. However, these bundles are also
  usable without the need for containers (which means that builders can mutate
  the root filesystem in whatever fashion they choose).
* Generates delta layers without requiring filesystem-specific features.
  Rather, it makes use of existing [mtree][mtree(8)] manifest tooling to
  compute the deltas of paths in the root filesystem.
* Supports [rootless containers][rootless] natively, both by allowing for
  extraction of layers that would normally require privileges and by generating
  runtime configurations that `runc` can use as an unprivileged user.
* Internal libraries are entirely built around a generic content addressable
  store interface, allowing for code reuse by other projects and the
  possibility for new backends.

[mtree(8)]: https://www.freebsd.org/cgi/man.cgi?mtree(8)
[oci-runtime]: https://github.com/opencontainers/runtime-spec
[runc]: https://github.com/opencontainers/runc
[oci-runtimes]: https://github.com/opencontainers/runtime-spec/blob/v1.0.0/implementations.md
[rootless]: https://rootlesscontaine.rs/

### Install ###

Pre-built binaries can be downloaded from [umoci's releases page][releases]. As
umoci's builds are reproducible, a cryptographic checksum file is included in
the release assets. All of the assets are also signed with a [release
key][umoci-keyring].

```text
pub   rsa4096 2016-06-21 [SC] [expires: 2031-06-18]
      5F36C6C61B5460124A75F5A69E18AA267DDB8DB4
uid           [ultimate] Aleksa Sarai <asarai@suse.com>
uid           [ultimate] Aleksa Sarai <asarai@suse.de>
sub   rsa4096 2016-06-21 [E] [expires: 2031-06-18]
```

umoci is also available from several distribution's repositories:

* [openSUSE](https://software.opensuse.org/package/umoci)
* [Gentoo](https://packages.gentoo.org/packages/app-emulation/umoci)
* [Arch Linux (AUR)](https://aur.archlinux.org/packages/umoci/)

To build umoci from the [source code][source], a simple `make && make install`
should work on most machines. The [changelog][changelog] is also available.

[releases]: https://github.com/openSUSE/umoci/releases
[umoci-keyring]: /umoci.keyring
[source]: https://github.com/openSUSE/umoci
[changelog]: /changelog

### License ###

umoci is licensed under the terms of the Apache 2.0 license.

```text
umoci: Umoci Modifies Open Containers' Images
Copyright (C) 2016, 2017 SUSE LLC.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```
