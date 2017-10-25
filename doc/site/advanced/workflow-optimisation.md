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

{{% notice tip %}}
Note that while this functionality <a
href="https://github.com/openSUSE/umoci/pull/201">has been merged</a>, it has
not yet been released in a version of umoci.
{{% /notice %}}

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
