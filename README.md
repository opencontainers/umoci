## `umoci` ###

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

Here is a laundry list of features that are being worked on [from the design
document][design.md]. Checked items have been "completed" (noting that the UX
is likely to change and that there are certain issues documented in the code
that probably result in technically invalid OCI images -- though testing showed
that the OCI tooling didn't pick up on the transgressions).

* [x] `umoci unpack`.
* [x] `umoci repack`.
* [x] `umoci/image/generate`.
* [ ] `umoci config`.
* [ ] `umoci gc`.
* [ ] `umoci init`.
* [ ] `umoci ref`.
* [ ] `umoci info`. (*optional*)
* [ ] `umoci verify`. (*optional*)
* [ ] `umoci sign`. (*optional*)

Currently `umoci` relies on several from-scratch implementations of existing
PRs against upstream projects (or aliased vendor projects that include PRs
merged that are not merged upstream). This is because currently upstream
projects are simply not mature enough to be used. It also means that this code
is definitely not safe for production.

* `go-mtree` needs to have better handling of comparing specifications and
  directories. I have [an open pull request which is being reviewed and will
  hopefully be merged soon][gomtree-pr].

* Currently `image/layerdiff` is a complete reimplementation of the proposed
  implementation for [`oci-create-layer`][oci-create-layer]. This is because I
  needed to use `[]gomtree.InodeDelta` but the hope is that my implementation
  will eventually be pushed upstream (once `go-mtree` merges my PR and gets
  more stable).

* In addition, `image/cas` is also a complete reimplementation of [this
  PR][oci-cas]. The reason for rewriting that code was because the proposed
  code doesn't implement some of the nice interfaces I would like (and has some
  other issues that I find frustrating). I hope that I can push my
  implementation upstream.

* Currently, `umoci` requires root privileges to extract the `rootfs` of an
  image. This is simply ludicrous, and I'm hoping that [this upstream PR will
  be merged][oci-ownership] so that I can use an `[]idtools.IDMap` to allow for
  translation of the owners of files when generating the `tar` archives. There
  are almost certainly hacky ways around this but I would prefer to avoid them.

Also, some of the vendored `opencontainers` libraries have bugs which means
that I have to manually patch them after running `hack/vendor.sh`. Which is
just bad on so many levels.

[gomtree-pr]: https://github.com/vbatts/go-mtree/pull/48
[oci-create-layer]: https://github.com/opencontainers/image-tools/pull/8
[oci-cas]: https://github.com/opencontainers/image-tools/pull/5
[oci-ownership]: https://github.com/opencontainers/image-tools/pull/3

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
