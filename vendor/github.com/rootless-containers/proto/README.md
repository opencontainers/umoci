## `user.rootlesscontainers` ##

This project contains the [protobuf][protobuf] definition of the
`user.rootlesscontainers` extended attribute. The main purpose of this
attribute is to allow for a interoperable and standardised way of emulating
persistent syscalls in a [rootless container][rootlesscontaine.rs] (syscalls
such as `chown(2)` which would ordinarily fail).

The issues that are encountered with rootless containers and things like
`chown(2)` are discussed in [the following talk from 2017][rootless-talk].

[protobuf]: https://developers.google.com/protocol-buffers/
[rootlesscontaine.rs]: https://rootlesscontaine.rs/
[rootless-talk]: https://youtu.be/r6EcUyamu94?t=1143

### Support ###

The following is a list of known projects that support
`user.rootlesscontainers` and can interoperate between one another.

* [`umoci`][umoci] (a tool for manipulating OCI images in a very flexible
  manner). Version `0.4.0` and later have full `user.rootlesscontainers`
  support, both with `umoci unpack` and `umoci repack`.

* [Our fork of PRoot][proot-fork] with a few patches applied. `PRoot` allows
  for full emulation (through `ptrace` with optional `seccomp` acceleration) of
  all privilege operations that would produce "strange" results inside a
  rootless container. This is a perfect fit for rootless containers.

[umoci]: https://github.com/openSUSE/umoci
[proot-fork]: https://github.com/rootless-containers/PRoot

### License ###

This project (in particular the `rootlesscontainers.proto` file and generated
sources) are all licensed under the Apache License 2.0.

```
rootlesscontainers-proto: persistent rootless filesystem emulation
Copyright (C) 2018 Rootless Containers Authors

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
