## `umoci` Design ##

The big issue with all of the existing OCI image tooling available (which
currently is only the official tooling from the OCI) is that it doesn't provide
a good enough tradeoff with power and convenience. In particular, the OCI
tooling isn't intended to provide any high-level interfaces. `umoci` is
different.

It is also possible that this tool will be merged with the goals of
[`oboci`][oboci], simply to avoid name confusion. But I haven't decided about
that yet, and is not a feature that I intend to work on soon.

[oboci]: https://github.com/cyphar/oboci

### Usage ###

`umoci` will be mainly used as a CLI (though I intend to provide some library
functionality similar to [runtime-tools][runtime-tools] and
[image-tools][image-tools]).

```
% umoci gc         # Garbage collects the entire image.
% umoci unpack ... # Unpack an manifest rootfs and runtime config.
% umoci repack ... # "Repack" a manifest, by adding a new diff layer.
                   # Note that the runtime config will not be modified because that is
% umoci config ... # Modify the configuration of a manfiest.
```

There are also some commands which might also be implemented, but are already
essentially implemented as part of [image-tools][image-tools] (or are more
nice-to-haves rather than must-haves).

```
% umoci init ... # Creates a new image or manifest.
% umoci stat ... # Get information about the image or a particular manifest.
```

[runtime-tools]: https://github.com/opencontainers/runtime-tools
[image-tools]: https://github.com/opencontainers/image-tools

### Garbage Collection ###

The `umoci gc` will be a standard mark-and-sweep garbage collector, where any
blob that is not in the Merkle tree for a reference will be removed. This is
something that will probably be upstreamed at some point, but will probably be
implemented here first.

### Layer Generation ###

The plan is that `umoci` will combine unpacking and repacking of layers, namely
that part of the diff layer generation is integrated into unpacking of an
image. The current plan is that `umoci` will generate the following layout when
unpacking an image:

```
unpacked-image
|-- config.json
|-- sha256_<digest>.mtree
`-- rootfs
    `-- <rootfs contents>
```

Where `<digest>` is the digest of the manifest which was unpacked, and the
`.mtree` file is the `go-mtree` specification for the rootfs after unpacking.
The intention of this setup is that when creating a diff layer from this
unpacked, all of the information required to "repack" the image is already
contained within `unpacked-image`.

This means that after making whatever modifications are necessary (like running
`runc run` on the generated bundle), `umoci` can create a new layer without
needing to look at the OCI image. This is an improvement over the
`oci-create-layer` interface, and also will end up leading to an overall
workflow like so:

```
% skopeo copy docker://opensuse/amd64:latest oci:opensuse
% umoci unpack --from ref:latest --image opensuse/ --bundle bundle/
% oci-runtime-tool generate --template bundle/config.json --read-only=false --args=build.sh --output bundle/config.json
% runc run -b bundle/ build-job-1337
% runc delete -f build-job-1337
% umoci repack --from ref:latest --image opensuse/ --bundle bundle/ --tag ref:latest
% umoci gc --image opensuse/
```

Obviously the above flag layout is subject to change, but I imagine it will
look something like that. The main point is that we generate a bundle from the
given ref (which also involves generating a `go-mtree` specification), then
after making our modifications to the rootfs we then "repack" it. Note that any
changes to the `config.json` do not affect the manifest (because the
translation is lossy and cannot be done in reverse).

Also note that the `repack` requires a `--from` argument to specify what the
source manifest was. The plan is that `umoci init` will allow you to create
some form of "dummy" manifest that has an empty rootfs to allow this workflow
to work for the creation of entirely new images.

The question of whether we should append something to the history of the
manifest is something that might be discussed, but will probably fall under the
`umoci config` interface.

### Configuration ###

One of the most frustrating parts of the OCI image format is that it has to be
completely compatible with the Docker image format. Which effectively results
in the configuration format [`application/vnd.oci.image.config.v1+json`][oci-config]
being disconnected from the runtime specification. This is unfortunate, and
means that the existing tooling for the runtime specification (namely
`oci-runtime-tools generate`) are not helpful.

So, `umoci config` will be the OCI image tool equivalent to  `oci-runtime-tools
generate`. I'm hoping this will eventually be upstreamed, but the main benefit
of `umoci config` is that the image will have all of the necessary blobs
updated so that you can actually modify the image without needing to do any
work yourself. It's likely that the generation component of this (especially if
I implement it as a library) will become part of the official OCI tooling.

The UX will probably look something like this:

```
% skopeo copy docker://opensuse/amd64:latest oci:opensuse
% umoci config --from ref:latest --tag ref:latest --image opensuse/ --cmd=anotherscript.sh --history="appended to history of manifest" --working-dir=/tmp
% umoci gc --image opensuse/
```

Which will update the `latest` reference to have the relevant manifest
configuration options "changed". Since OCI images are a CAS, "changing" a
manifest means that you generate all of the necessary blobs to replace the
manifest and then change the reference to point to the new manifest.

[oci-config]: https://github.com/opencontainers/image-spec/blob/master/config.md
