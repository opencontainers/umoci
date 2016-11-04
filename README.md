## `umoci` ###

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

### In Progress ###

Currently `umoci` isn't being developed, since there are several open PRs and
issues that need to be resolved in order for this code to be worked on. In no
particular order:

* [ ] `go-mtree` needs to have better handling of comparing specifications and
  directories. I have [an open pull request which is being reviewed and will
  hopefully be merged soon][gomtree-pr].

* [ ] The proposed implementation for [`oci-create-layer`][oci-create-layer]
  needs to be finalised so that I can fork it to use `[]gomtree.InodeDelta` to
  generate the diff layer. I intend to actually contribute that fork back
  upstream, but the maintainers have expressed concerns about `go-mtree`
  because of its age.

* [ ] While I don't agree with the CLI tooling, in order to effectively access the
  CAS of an OCI image we need to have the `Engine` interface from [this
  PR][oci-cas] merged. In particular, I don't want to have to write my own
  version of the blob writing code.

There are also some other issues that I need to make a decision about (specifically)

[gomtree-pr]: https://github.com/vbatts/go-mtree/pull/48
[oci-create-layer]: https://github.com/opencontainers/image-tools/pull/8
[oci-cas]: https://github.com/opencontainers/image-tools/pull/5

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
