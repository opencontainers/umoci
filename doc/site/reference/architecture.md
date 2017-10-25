+++
title = "Architecture"
weight = 20
+++

umoci is a fairly simple tool, that takes advantage of a couple of tricks in
order to allow for modification of OCI images. This architecture document is
fairly high-level and will likely change once some more of the
[roadmap][roadmap.md] is implemented. If you feel something is missing from
this document, feel free to [contribute][contributing.md].

When umoci was first released, I also published [a blog post][blog] outlying
the original architecture (which has remained fairly similar over time).

[roadmap.md]: /doc/roadmap.md
[contributing.md]: /contributing
[blog]: https://www.cyphar.com/blog/post/umoci-new-oci-image-tool

### Manifests ###

The key feature of umoci is that it allows for generation of delta layers
without the need for copies of the original root filesystem or fancy filesystem
features (such as snapshots or overlays). As a result, the design is applicable
to any modern Unix-like operating system.

This feature is implemented through the use of manifest files. In particular,
[`mtree(8)` manifests][mtree(5)]. After extraction of the root filesystem has
been completed, a full manifest is generated from the root filesystem. When
wanting to generate a delta layer, a new manifest is generated and the two
manifests are compared. Any inconsistencies are then added to the delta layer.

By using this very simple (and quite old) technique, we can create delta layers
without the need for copies of the original filesystem or fancy filesystems.

[mtree(8)]: https://www.freebsd.org/cgi/man.cgi?mtree(5)

### Mutation ###

The main purpose of umoci is the ability to modify an image in various ways
(by changing the configuration or adding delta layers).

At the core of an OCI image is a content-addressable blob store, with different
types of blobs signifying important data or metadata about an image contained
in the store. As all blobs (both the metadata and data) are
content-addressable, it does not make sense to talk about "modifying" a blob.

Instead, what umoci does is that it creates a new version of a blob that is
being replaced. Then, umoci walks up the referencing path that it took to
reach the replaced blob and replaces all of the blobs that reference the old
blob with a new blob referencing the new blob. This replacement will result in
further changes to parents until the change bubbles up to the root
`index.json`. Then, umoci will create or replace a top-level `index.json`
entry to point to the newly created tree. Note that umoci will not replace
any blobs that were not in the ancestor path of the modification, which means
that all of the unchanged blobs are necessarily de-duplicated (and any other
references to the old blob remain intact).
