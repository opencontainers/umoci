+++
title = "Workflow Optimisation"
weight = 10
+++

One of the first things that a user of umoci may notice is that certain
operations can be quite expensive. Notably unpack and repack operations require
either scanning through each layer archive of an image, or scanning through the
filesystem. Both operations require quite a bit of disk IO, and can take a
while. Fedora images are known to be quite large, and can take several seconds
to operate on.

```text
% time umoci unpack --image fedora:26 bundle
umoci unpack --image fedora:26 bundle  8.43s user 1.68s system 105% cpu 9.562 total
% time umoci repack --image fedora:26-old bundle
umoci repack --image fedora:26 bundle  3.62s user 0.43s system 115% cpu 3.520 total
% find bundle/rootfs -type f -exec touch {} \;
% time umoci repack --image fedora:26-new bundle
umoci repack --image fedora:26-new bundle  32.03s user 4.50s system 112% cpu 32.559 total
```

While it is not currently possible to optimise or parallelise the above
operations individually (due to the structure of the layer archives), it is
possible to optimise your workflows in certain situations. These workflow tips
effectively revolve around reducing the amount of extractions that are
performed.

### `--refresh-bundle` ###

A very common workflow when building a series of layers in an image is that,
since you want to place different files in different layers of the image, you
have to do something like the following:

```text
% umoci unpack --image image_build_XYZ:wip bundle_a
% ./some_build_process_1 ./bundle_a
% umoci repack --image image_build_XYZ:wip bundle_a
% umoci unpack --image image_build_XYZ:wip bundle_b
% ./some_build_process_2 ./bundle_b
% umoci repack --image image_build_XYZ:wip bundle_b
% umoci unpack --image image_build_XYZ:wip bundle_c
% ./some_build_process_3 ./bundle_c
% umoci repack --image image_build_XYZ:wip bundle_c
% umoci tag --image image_build_XYZ:wip final
```

The above usage, while correct, is not very efficient. Each layer that is
created requires us to to do an unpack of the entire `image_build_XYZ:wip`
image before we can do anything. By noting that the root filesystem contained
in `bundle_a` after we've made our changes is effectively the same as the root
filesystem that we extract into `bundle_b` (and since we already have
`bundle_a` we don't have to extract it), we can conclude that using `bundle_a`
is probably going to be more efficient. However, you cannot just do this the
"intuitive way":

```text
% umoci unpack --image image_build_XYZ:wip bundle_a
% ./some_build_process_1 ./bundle_a
% umoci repack --image image_build_XYZ:wip bundle_a
% ./some_build_process_2 ./bundle_a
% umoci repack --image image_build_XYZ:wip bundle_a
% ./some_build_process_3 ./bundle_a
% umoci repack --image image_build_XYZ:wip bundle_a
% umoci tag --image image_build_XYZ:wip final
```

Because the metadata stored in `bundle_a` includes information about what image
the bundle was based on (this is used when creating the modified image
metadata). Thus, the above usage will *not* result in multiple layers being
created, and the usage is roughly identical to the following:

```text
% umoci unpack --image image_build_XYZ:wip bundle_a
% ./some_build_process_1 ./bundle_a
% ./some_build_process_2 ./bundle_a
% ./some_build_process_3 ./bundle_a
% umoci repack --image image_build_XYZ:wip bundle_a
% umoci tag --image image_build_XYZ:wip final
```

Do not despair however, there is a flag just for you! With `--refresh-bundle`
it is possible to perform the above operations without needing to do any extra
unpack operations.

```text
% umoci unpack --image image_build_XYZ:wip bundle_a
% ./some_build_process_1 ./bundle_a
% umoci repack --refresh-bundle --image image_build_XYZ:wip bundle_a
% ./some_build_process_2 ./bundle_a
% umoci repack --refresh-bundle --image image_build_XYZ:wip bundle_a
% ./some_build_process_3 ./bundle_a
% umoci repack --refresh-bundle --image image_build_XYZ:wip bundle_a
% umoci tag --image image_build_XYZ:wip final
```

Internally, `--refresh-bundle` is modifying the few metadata files inside
`bundle_a` so that future repack invocations modify the new image created by
the previous repack operation rather than basing it on the original unpacked
image. Therefore the cost of `--refresh-bundle` is constant, and is actually
**much** smaller than the cost of doing additional unpack operations.

### `umoci insert` ###

Sometimes all you want to do is to add some files to an image (or remove some
files) and nothing else, and in those cases doing an `umoci unpack`-`umoci
repack` cycle is also quite expensive. This is especially true when you
consider that OCIv1 images are backed by `tar` archives -- and the delta layer
being generated is just going to be a `tar` archive of the files you are
adding. The most basic usage of `umoci insert` is to just specify what files
you want added, and what you want them to be called in the image (we don't have
any magical `rsync` semantics -- we just copy the root to whatever path you
tell us).

{{% notice info %}}
Note that unlike most other `umoci` commands, `umoci insert` **will overwrite
the image you give it**. As a counter-example, the `--image` flag of `umoci
repack` refers to the *target* image not the *source* image (the source image
is already known, because `umoci unpack` saves that information).

This behaviour may change in the future, but it's not clear what would be an
obvious interface for this change (older versions of `umoci` had separate
`--src` and `--dst` flags, but they were unwieldy and so were removed in
favour of the `--image` style).

Also note that each `umoci insert` creates a separate layer.
{{% /notice %}}

```text
% umoci insert --image myimg:foo mybinary /usr/bin/release-binary
% umoci insert --image myimg:foo myconfigdir /etc/binary.d
```

If the target file already exists in previous layers, the new layer will
overwrite any older versions of the files inserted (when extracted).

You can also remove a file (or directory) from an image by using the
`--whiteout` option, which creates a new layer with a "whiteout" entry for the
path you give it. If the file doesn't already exist, the behaviour depends on
the extraction tool used -- `umoci insert` will ignore whiteouts for
non-existent files when extracting.

{{% notice warning %}}
**Do not use this to remove secrets from an image.** Since `umoci insert`
operates by creating a new layer, older layers will still contain a copy of the
secret you are trying to remove. If you want to avoid things from being
included in an image in the first place, take a look at `umoci repack
--mask-path` (which causes changes to the given paths to not be included in the
new layer) or `umoci config --config.volumes` (which is automatically treated
as a masked path by `umoci repack`).
{{% /notice %}}

```text
% umoci insert --whiteout /usr/bin/old-binary
% umoci insert --whiteout /etc/old-config.d
```

Finally, there is one more important thing to know about `umoci insert` -- how
directory insertion is handled. By default, `umoci insert` just creates a new
layer with the contents of the directory. When unpacked, this results in any
existing contents in that directory (from older layers) to be merged with the
new layer's contents. You can imagine this as though you extracted your new
directory on top of the previous layers' cumulative directory state.

But what if you want to entire replace the contents of a directory? That's the
reason why we have `--opaque` -- it allows you to effectively blank out any
pre-existing contents of the directory and replace it entirely with the new
directory. If the target was not a directory in previous layers, or the source
is not a directory, then the behaviour will depend on the tool used for
extraction -- `umoci unpack` will just ignore the meaningless opaque whiteout
entry.

```text
% umoci insert --opaque myetcdir /etc
```

The same caveat about `umoci insert --whiteout` applies here, as older layers
will contain the files that were removed by the opaque whiteout.

{{% notice info %}}
It should be noted that this is the only way that umoci will currently create
an "opaque whiteout". This means that if you need to replace an entire
directory wholesale, the layer created by `umoci insert --opaque` is far more
efficient in the resulting layer than the `umoci unpack`-`umoci repack` cycle
(even if you ignore the CPU-time benefits).

Though currently `umoci insert` only allows one operation per layer, which is
mostly a UX restriction. This may change in the future, and so `umoci insert`
will be *far* more generally usable and efficient in terms of number of layers
generated.
{{% /notice %}}
