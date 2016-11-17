## `umoci` ###

[![Build Status](https://img.shields.io/travis/cyphar/umoci/master.svg)](https://travis-ci.org/cyphar/umoci)
![License: Apache 2.0](https://img.shields.io/github/license/cyphar/umoci.svg)

**Status: Pre-Alpha**

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

Some of the [planned design][design.md] has been written down, but is subject
to change (once all of the in progress points below have been addressed).

[oci-image-spec]: https://github.com/opencontainers/image-spec
[oci-image-tools]: https://github.com/opencontainers/image-tools
[disc-1]: https://github.com/opencontainers/image-spec/pull/411
[disc-2]: https://github.com/opencontainers/image-tools/pull/5
[disc-3]: https://github.com/opencontainers/image-tools/pull/8
[design.md]: DESIGN.md

### In Progress ###

Currently `umoci` relies on several from-scratch implementations of existing
PRs against upstream projects (or aliased vendor projects that include PRs
merged that are not merged upstream). This is because currently upstream
projects are simply not mature enough to be used. It also means that this code
is definitely not safe for production.

Also note that currently, `umoci` requires root privileges to extract the
`rootfs` of an image. This is simply ludicrous, but this will be fixed before
`0.0.0` and will be tracked by [this issue][unpriv-issue].

Also, some of the vendored `opencontainers` libraries have bugs which means
that I have to manually patch them after running `hack/vendor.sh`. Which is
just bad on so many levels.

[unpriv-issue]: https://github.com/cyphar/umoci/issues/26

### License ###

`umoci` is licensed under the terms of the Apache 2.0 license.

```
umoci: Umoci Modifies Open Containers' Images
Copyright (C) 2016 SUSE LLC.

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
