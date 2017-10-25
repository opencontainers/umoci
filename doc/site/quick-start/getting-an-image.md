+++
title = "Getting an Image"
weight = 10
+++

For most users, before you can do anything with umoci, you have to first have
an [OCI image][oci-image]. At the time of writing, there is no standard way of
getting an OCI image.  Distribution is still an open topic in the
specification, and there are very few implementations of a distribution
extension to the OCI specification. I've [personally worked on one][parcel] but
there is still a lot of work to go before you can skip this step and get OCI
images without the need to convert from other things.

In order to get an OCI image, you need to convert it from another container
image format. Luckily, as the OCI spec was based on the Docker image format,
there is no loss of information when converting between the two formats.
[skopeo][skopeo] is an incredibly useful tool that allows you to fetch and
convert a Docker image (from a registry, local daemon or even from a file saved
with `docker save`) to an OCI image (and vice-versa). Read their documentation
for more information about the various other formats they support.

{{% notice warning %}}
At the time of writing there is a <a
href="https://github.com/projectatomic/skopeo/pull/420">known issue in
skopeo</a> (and the latest release does not contain an existing hotfix), caused
by a change in <a
href="https://blog.docker.com/2017/09/docker-official-images-now-multi-platform/">the
"official" library</a>. Some images (such as the above openSUSE images) are not
multi-arch and thus still work properly, but this should be taken into
consideration.
{{% /notice %}}

After getting skopeo, you can download an image as follows. Note that you can
include multiple Docker images inside the same OCI image (under different
"tags").

```text
% skopeo copy docker://opensuse/amd64:42.2 oci:opensuse:42.2
Getting image source signatures
Copying blob sha256:f65b94255373e4dc9645fcb551756b87726a1c891fe6c89f6bbbc864ff845c15
 46.59 MB / 46.59 MB [=========================================================]
Copying config sha256:5af572844af6ae4122721ba6bfa11b4048dc4535a9f52772e809a68cac4e9244
 0 B / 805 B [-----------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```
```text
% skopeo copy docker://opensuse/amd64:42.1 oci:opensuse:old_42.1
Getting image source signatures
Copying blob sha256:d9e29ed5a74f21e153b05ecc646fe1157fcfa991c9661759986191408665521b
 36.60 MB / 36.60 MB [=========================================================]
Copying config sha256:1652ed016d569d50729738e2f4ab3564f7375a25150c4a1ac1cc6687e586a5ce
 0 B / 805 B [-----------------------------------------------------------------]
Writing manifest to image destination
Storing signatures
```

At this point, you have a directory called `opensuse` which is the downloaded
OCI image stored as a directory. This is currently the only type of layout that
umoci can interact with.

{{% notice warning %}}
At the time of writing there is a <a
href="https://github.com/projectatomic/skopeo/pull/306">known issue in
skopeo</a>, which causes the above examples to not act correctly (only the
latest image fetched will be accessible by it's tag name from the OCI image).
{{% /notice %}}

[oci-image]: https://github.com/opencontainers/image-spec
[parcel]: https://github.com/cyphar/parcel
[skopeo]: https://github.com/projectatomic/skopeo
