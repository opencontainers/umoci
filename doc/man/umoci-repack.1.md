% umoci-repack(1) # umoci repack - Repacks an OCI runtime bundle into an image tag
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci repack - Repacks an OCI runtime bundle into an image tag

# SYNOPSIS
**umoci repack**
**--image**=*image*[:*tag*]
[**--compress**=*compression-type*]
[**--no-history**]
[**--history.comment**=*comment*]
[**--history.created_by**=*created_by*]
[**--history.author**=*author*]
[**--history-created**=*date*]
[**--refresh-bundle**]
*bundle*

# DESCRIPTION
Given a modified OCI bundle extracted with **umoci-unpack**(1) (at the given
path *bundle*), **umoci-repack**(1) computes the filesystem delta for the OCI
bundle's *rootfs*. The delta is used to generate a delta layer, which is then
appended to the original image manifest (that was used as an argument to
**umoci-unpack**(1)) and tagged as a new image tag. Between a call to
**umoci-unpack**(1) and **umoci-repack**(1) users SHOULD NOT modify the OCI
image in any way (specifically you MUST NOT use **umoci-gc**(1)).

All **--uid-map** and **--gid-map** settings are implied from the saved values
specified in **umoci-unpack**(1), so they are not available for
**umoci-repack**(1).

If **--no-history** was not specified, a history entry is appended to the
tagged OCI image for this change (with the various **--history.** flags
controlling the values used). To view the history, see **umoci-stat**(1).

Note that the original image tag (used with **umoci-unpack**(1)) will **not**
be modified unless the target of **umoci-repack**(1) is the original image tag.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The destination tag for the repacked OCI image. *image* must be a path to a
  valid OCI image (and the same *image* used in **umoci-unpack**(1) to create
  the *bundle*) and *tag* must be a valid tag name. If another tag already has
  the same name as *tag* it will be overwritten. If *tag* is not provided it
  defaults to "latest".

**--compress**=*compression-type*
  Specify the compression type to use when creating a new layer. Supported
  compression types are *none*, *gzip*, and *zstd*. **umoci-unpack**(1)
  transparently supports all compression methods you can specify with
  **--compress**.
  <!-- paragraph break -->
  The special value *auto* will cause **umoci**(1) to auto-select the most
  appropriate compression algorithm based on what previous layers are
  compressed with (it will try to use the most recent layer's compression
  algorithm which it supports). Note that *auto* will never select *none*
  compression automatically, as not compressing **tar**(1) archives is really
  not advisable.
  <!-- paragraph break -->
  If no *compression-type* is provided, it defaults to *auto*.

**--no-history**
  Causes no history entry to be added for this operation. **This is not
  recommended for use with umoci-repack(1), since it results in the history not
  including all of the image layers -- and thus will cause confusion with tools
  that look at image history.**

**--history.comment**=*comment*
  Comment for the history entry corresponding to this modification of the image
  If unspecified, **umoci**(1) will generate an implementation-dependent value.

**--history.created_by**=*created_by*
  CreatedBy entry for the history entry corresponding to this modification of
  the image. If unspecified, **umoci**(1) will generate an
  implementation-dependent value.

**--history.author**=*author*
  Author value for the history entry corresponding to this modification of the
  image. If unspecified, this value will be the image's author value **after**
  any modifications were made by this call of **umoci-config**(1).

**--history-created**=*date*
  Creation date for the history entry corresponding to this modifications of
  the image. This must be an ISO8601 formatted timestamp (see **date**(1)). If
  unspecified, the current time is used.

**--refresh-bundle**
  Whether to update the OCI bundle's metadata (i.e. mtree and umoci
  metadata) after repacking the image. If set, then the new state of
  the bundle should be equivalent to unpacking the new image tag.

# EXAMPLE
The following downloads an image from a **docker**(1) registry using
**skopeo**(1), unpacks it with **umoci-unpack**(1), modifies it and then
repacks it.

```
% skopeo copy docker://opensuse/amd64:42.2 oci:image:latest
# umoci unpack --image image bundle
# touch bundle/rootfs/a_new_file
# umoci repack --image image:new-42.2 bundle
```

# SEE ALSO
**umoci**(1), **umoci-unpack**(1)
