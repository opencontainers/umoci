## `umoci` [![Release](https://img.shields.io/github/release/openSUSE/umoci.svg)](https://github.com/openSUSE/umoci/releases/latest) ###

[![Build Status](https://img.shields.io/travis/openSUSE/umoci/master.svg)](https://travis-ci.org/openSUSE/umoci)
![License: Apache 2.0](https://img.shields.io/github/license/openSUSE/umoci.svg)

**Status: Alpha**

**u**moci **m**odifies **O**pen **C**ontainer **i**mages. Not a great name, but
what are you going to do. It also is a cool way for people to "dip their toe"
into OCI images ("umoci" also means "to dip" in Serbian).

`umoci` intends to be a complete manipulation tool for [OCI images][oci-image-spec].
In particular, it should be seen as a more end-user-focused version of the
[`oci-image-tools` provided by the OCI][oci-image-tools]. The hope is that all
of this tooling will eventually be merged with the upstream repository, so that
it is always kept up-to-date by the Open Container Initiative community.

However, currently there is a [lot][disc-1] [of][disc-2] [dicussion][disc-3]
about the new tooling going into the OCI image tools, and right now I need
tooling that can abstract all of the internals of the OCI specification into a
single CLI interface. The main purpose of this tool is to serve as example of
what **I** would like to see in an `oci-image` tool.

[oci-image-spec]: https://github.com/opencontainers/image-spec
[oci-image-tools]: https://github.com/opencontainers/image-tools
[disc-1]: https://github.com/opencontainers/image-spec/pull/411
[disc-2]: https://github.com/opencontainers/image-tools/pull/5
[disc-3]: https://github.com/opencontainers/image-tools/pull/8

### Usage ###

`umoci` has a subcommand-based commandline. For more detailed information, see
the generated man pages (which you can build with `make doc`).

```
% umoci --help
NAME:
   umoci - umoci modifies Open Container images

USAGE:
   umoci [global options] command [command options] [arguments...]

VERSION:
   0.0.0~rc3

AUTHOR(S):
   Aleksa Sarai <asarai@suse.com>

COMMANDS:
     help, h  Shows a list of commands or help for one command

   image:
     config      modifies the image configuration of an OCI image
     unpack      unpacks a reference into an OCI runtime bundle
     repack      repacks an OCI runtime bundle into a reference
     new         creates a blank tagged OCI image
     tag         creates a new tag in an OCI image
     remove, rm  removes a tag from an OCI image
     stat        displays status information of an image manifest

   layout:
     gc        garbage-collects an OCI image's blobs
     init      create a new OCI layout
     list, ls  lists the set of tags in an OCI image

GLOBAL OPTIONS:
   --debug        set log level to debug
   --help, -h     show help
   --version, -v  print the version
```

### Example ###

The following is an example shell session, where a user does the following operations:

1. Pulls an image from a Docker registry using [`skopeo`][skopeo];
2. Extracts the image to an OCI runtime bundle (and then makes some
   modifications to the configuration [`oci-runtime-tools`][oci-runtime-tools]);
2. Makes some modifications to the rootfs inside a container with [runC][runc];
3. Makes further modifications outside of the container to the rootfs;
4. Creates a new image the contains the set of rootfs changes;
5. Changes some of the configuration information for the image; and
6. Finally, pushes the finalised image back to the Docker registry.

```
% skopeo copy docker://opensuse/amd64:42.2 oci:opensuse:latest
Getting image source signatures
Copying blob sha256:32f7bb9291d9339af352ed8012f0e9edd05d7397d283b6c09ce604d2ecfc5d07
 37.03 MB / 37.03 MB [=========================================================]
Copying config sha256:a6f6d93caed6e40729f2303fd950cec3973dfbcf09bdaa4aab247618f716c9cb
 0 B / 1.73 KB [---------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
% umoci unpack --image opensuse bundle
INFO[0000] parsed mappings                    map.gid=[] map.uid=[]
INFO[0000] unpack manifest: unpacking layer sha256:32f7bb9291d9339af352ed8012f0e9edd05d7397d283b6c09ce604d2ecfc5d07  diffid="sha256:bb6447f230852c3e1e07fb5c5d50ec3960bbf15786660f4519ade03dc6237ca1"
INFO[0001] unpack manifest: unpacking config  config="sha256:a6f6d93caed6e40729f2303fd950cec3973dfbcf09bdaa4aab247618f716c9cb"
% oci-runtime-tool generate --bind /etc/resolv.conf:/etc/resolv.conf:ro --linux-namespace-remove network --template bundle/config.json > bundle/config.json.tmp && mv bundle/config.json{.tmp,}
% runc run -b bundle ctr
sh-4.2# zypper ref
Retrieving repository 'NON-OSS' metadata ................................[done]
Building repository 'NON-OSS' cache .....................................[done]
Retrieving repository 'OSS' metadata ....................................[done]
Building repository 'OSS' cache .........................................[done]
Retrieving repository 'OSS Update' metadata .............................[done]
Building repository 'OSS Update' cache ..................................[done]
Retrieving repository 'Update Non-Oss' metadata .........................[done]
Building repository 'Update Non-Oss' cache ..............................[done]
All repositories have been refreshed.
sh-4.2# zypper in strace
Loading repository data...
Reading installed packages...
Resolving package dependencies...

The following 2 NEW packages are going to be installed:
  libunwind strace

2 new packages to install.
Overall download size: 217.7 KiB. Already cached: 0 B. After the operation, additional 709.6 KiB will be used.
Continue? [y/n/? shows all options] (y): y
Retrieving package libunwind-1.1-11.1.x86_64  (1/2),  47.4 KiB (137.3 KiB unpacked)
Retrieving: libunwind-1.1-11.1.x86_64.rpm ...............................[done]
Retrieving package strace-4.10-3.1.x86_64     (2/2), 170.3 KiB (572.3 KiB unpacked)
Retrieving: strace-4.10-3.1.x86_64.rpm ..................................[done]
Checking for file conflicts: ............................................[done]
(1/2) Installing: libunwind-1.1-11.1.x86_64 .............................[done]
(2/2) Installing: strace-4.10-3.1.x86_64 ................................[done]
sh-4.2# zypper rr 1 4
Removing repository 'NON-OSS' ...........................................[done]
Repository 'NON-OSS' has been removed.
Removing repository 'Update Non-Oss' ....................................[done]
Repository 'Update Non-Oss' has been removed.
sh-4.2# zypper cc -a
All repositories have been cleaned up.
sh-4.2# exit
% sed -i 's/42.2/42.3/g' bundle/rootfs/etc/os-release
% umoci repack --image opensuse:42.3 --history.author="Aleksa Sarai <asarai@suse.com>" bundle
INFO[0000] created new layout  digest="sha256:f9362f2348cbdac6ff039b3fd470900912ed06169d4c9ff420db40f015a00224" mediatype="application/vnd.oci.image.manifest.v1+json" size=566
% umoci config --image opensuse:42.3 --author="Aleksa Sarai <asarai@suse.com>" \
		--created="$(date --iso-8601=seconds)" \
		--config.entrypoint="strace" --config.entrypoint="-f" \
		--config.cmd="bash"
INFO[0000] created new image  digest="sha256:6d02fed0aeaf26f5bd774d7351d1cb06a887aabfeb9aeaa949d5c2efdc0b8cbd" mediatype="application/vnd.oci.image.manifest.v1+json" size=566
% umoci gc --layout opensuse >/dev/null
% skopeo copy opensuse:42.3 docker://opensuse/amd64:42.3
Getting image source signatures
Copying blob sha256:32f7bb9291d9339af352ed8012f0e9edd05d7397d283b6c09ce604d2ecfc5d07
 0 B / 37.03 MB [--------------------------------------------------------------]
Copying blob sha256:0c7b0d5f8397d389273d347d68df215e6b0abbcd7c7a4a2ead93030312c9310b
 2.23 MB / 2.23 MB [===========================================================]
Copying config sha256:9aa5fb05adcc49d20b662789af45e0f7cdb49206926e656d6ea11c7e7504461d
 1.25 KB / 1.25 KB [===========================================================]
Writing manifest to image destination
Storing signatures
```

Note that because we haven't modified the original `opensuse/amd64:42.2`
filesystem blob, when we upload our new image to the Docker registry with
`skopeo` we don't have to re-upload that layer. In addition, the diff layer is
only ~2MB in size.

All of the above tooling is available from various [OBS repositories][obs] on
[openSUSE][opensuse]. In particular:

* `skopeo`, `umoci`, `oci-runtime-tools`, and `oci-image-tools` are available
  from [`Virtualization:containers`][obs-vc].

[opensuse]: https://www.opensuse.org/
[oci-runtime-tools]: https://github.com/opencontainers/image-tools
[obs]: https://build.opensuse.org/
[obs-vc]: https://build.opensuse.org/project/show/Virtualization:containers

### In Progress ###

Currently `umoci` relies on several from-scratch implementations of existing
PRs against upstream projects (or aliased vendor projects that include PRs
merged that are not merged upstream). This is because currently upstream
projects are simply not mature enough to be used. However, this is something
that I'm working on fixing.

### License ###

`umoci` is licensed under the terms of the Apache 2.0 license.

```
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
