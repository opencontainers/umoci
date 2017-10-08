+++
title = "Creating an Image"
weight = 30
+++

Creating a new image with umoci is fairly simple, and effectively involves
creating an image "husk" which you can then [operate on][workflow] as though it
was a normal image. New images contain no layers, and have a dummy
configuration that should be replaced by a user.

If you wish to create a new image layout (which contains nothing except the
bare minimum to specify that the image is an OCI image), you can do so with
`umoci init`.

```text
% umoci init --layout new_image
```

If you wish to create a new image inside the image layout, you can do so with
`umoci new`.

```text
% umoci new --image new_image:new_tag
```

[workflow]: /quick-start/workflow
