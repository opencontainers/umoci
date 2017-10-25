+++
title = "Roadmap"
weight = 15
+++

This document describes an informal roadmap for the future of the umoci
project, in both technical and operational aspects. If you feel there is
something missing from this document, feel free to
[contribute][contributing.md].

[contributing.md]: /contributing

### Improved API Design ###

One of the main goals of umoci is to act as a "defacto" implementation of the
relevant image-spec interfaces. In particular, the `oci/cas` and `oci/casext`
packages are perfect examples of the style of interface that would be
incredibly reusable. In addition, the `oci/cas/driver` model allows for
different backend implementations to be used as long as they provide the
`oci/cas.Engine` interface.

However, at the moment several of the other packages in `oci/` need some love
in creating proper re-usable interfaces. There are many useful components in
`oci/layer` (for example), but they are all hidden behind the top-level
entrypoints that are used by `umoci unpack` and `umoci repack`. Other `oci/`
libraries have similar issues. This is arguably the highest priority item in
this roadmap, as it is blocking the adoption of our `oci/` libraries as well as
making other refactors much harder to do.

Also note that the `mutate/` package should also be reworked to provide a much
cleaner interface (as well as providing support for proper nested handling). To
be quite honest, I'm not actually sure if it handles the arbitrary nesting
feature of `1.0.0-rc5` and later (and that's an issue we need to resolve).

### Arbitrary Tree Structures Interface ###

With the upgrade to `1.0.0-rc5` of the image-spec, it is now possible to have
arbitrarily nested `Index` objects. This means that users may wish to create
arbitrary tree structures, as well as interact with such structures. Noting
that ultimately a user will just want to interact with a `Manifest` (at the end
of the day), there are several things that need to be improved in the interface
design.

* When querying an image to get a `Manifest`, we need to have a "resolution"
  interface (which must also be operated in a non-interactive way) that allows
  for increasing the specificity when describing what manifest we are
  interested in. After applying `org.opencontainers.image.ref.name` and
  platform filters, we have to provide additional filtering and referencing
  support. Unfortunately it currently is not very clear how this is meant to
  work with an arbitrary image, or if it is expected the referencing will
  always be contextual for a particular image. In addition, there is no part of
  the spec that actually describes the algorithm for such referencing.

* When replacing a `Manifest`, we just need to replace that manifest and all of
  the objects along the resolution path. Unfortunately, the last part of that
  requirement is quite complicated to deal with now that dereferencing has been
  punted to us (especially when it comes to the concept of creating a new tag
  for the modified object -- should we therefore re-create the entire tree of
  the original "reference" and work from there?). Ultimately I feel it may be
  required to store the `Manifest` descriptor as well as the highest-level
  (from the `index.json`) descriptor so that umoci knows where to re-create
  the structure from.

* When creating a new `Manifest` in an image, we need to be able to handle any
  arbitrary tree structure. Ultimately I think the only proper way of handling
  this will be to require the user to provide a specification-style
  description. I have toyed with the idea of automatically restructuring images
  as necessary (something that would be required if you wanted to avoid the
  creation of duplicate tags in the top-level image -- something that I really
  would like to make sure happens) but it's not clear if users will always want
  us to be maximally clever. Maybe we should provide that and in addition
  provide the specification-style interface for power users (then we have to
  figure out what happens if the two schemes clash or if the specification
  invalidates our own preferences).

I'm really hoping the above issues may be solvable through some sort of fancy
algorithm (especially when it comes to resolving conflicts). Further research
is required in this area.

An additional point to the above is that the current `--image` interface is
quite ugly (mainly because I copied Docker's referencing interface). We really
should dispense with this and go straight to [proper URI semantics][rfc3986]
with `#partial` tags being used to indicate `org.opencontainers.image.ref.name`.
In addition, I really would like `--layout` to no longer be necessary. Maybe if
we made `#partial` tags required this would allow us to combine the two flags,
though it would break the ability to have a "default" tag.

[rfc3986]: https://tools.ietf.org/html/rfc3986

### Direct `mtree`-from-layer Generation ###

Currently umoci generates its `mtree(8)` manifest from the real filesystem
after the entire bundle has been extracted. This works well enough in practice
(and actually guarantees us that a `repack` will contain an empty archive if we
just did an `unpack` and nothing else). However, it has several issues.

* With `--rootless` we do several tricks to make `umoci unpack` work as an
  unprivileged user. Many of the tricks don't change how things are extracted
  (such as the whole `pkg/unpriv` trick), but there are several cases where we
  will intentionally incorrectly extract objects to avoid privilege errors.
  Examples of this include `security.capabilities` xattrs (which are currently
  not user namespace safe). In order for a from-layer system to work, we would
  need to modify the generated `mtree(8)` manifest so that it matches the
  on-disk state that we expect given what tricks we know we will employ.

* In addition, binding the `mtree(8)` tricks to the layer representation will
  mean that any future improvements to the OCI image-spec (such as removing the
  need for linear archives) will result in far more complicated changes to
  umoci. However, if the generator is designed in an extensible fashion this
  should not be a huge deal.

Vincent Batts (the author of `gomtree`) has expressed interest in this sort of
functionality, but I'm not entirely convinced that it will work as well as
expected (especially given the enormous amount of special casing in `go-mtree`
that already exists in order to implement `tar` archive support).

### Bus Factor ###

At the moment this project was effectively entirely written and maintained by
one person. This is obviously an unsustainable development model, and also
raises several other issues. In addition, currently there is no public mailing
list for the development of umoci (nor is there a security mailing list)
which makes it difficult to on-board new maintainers or significant
contributors.
