+++
title = "Garbage Collection"
weight = 40
+++

Every umoci operation that modifies an image will not delete any now-unused
blobs in the image (so as to ensure that any other operations that assume those
blobs are present will not error out). However, this will result in a large
number of useless blobs remaining in the image after operating on an image for
a long enough period of time. `umoci gc` will garbage collect all blobs that
are not reachable from any known image tag. Note that calling `umoci gc`
between `umoci unpack` and `umoci repack` may result in errors if you've
removed all references to the blobs used by `umoci unpack`.

```text
% umoci gc --layout opensuse
```
