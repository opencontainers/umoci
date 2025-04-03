% umoci-raw-add-layer(1) # umoci raw add-layer - add a layer archive verbatim to an image
% Aleksa Sarai
% SEPTEMBER 2018
# NAME
umoci raw add-layer - add a layer archive verbatim to an image

# SYNOPSIS
**umoci raw add-layer**
**--image**=*image*
[**--tag**=*tag*]
[**--compress**=*compression-type*]
[**--no-history**]
[**--history.comment**=*comment*]
[**--history.created_by**=*created_by*]
[**--history.author**=*author*]
[**--history-created**=*date*]
*new-layer.tar*

# DESCRIPTION
Adds the uncompressed layer archive referenced by *new-layer.tar* verbatim to
the image. Note that since this is done verbatim, no changes are made to the
layer and thus any OCI-specific `tar` extensions (such as `.wh.` whiteout
files) will be included unmodified. Use of this command is therefore only
recommended for expert users, and more novice users should look at
**umoci-repack**(1) to create their layers.

At the moment, **umoci raw add-layer** only supports appending layers to the
end of the image layer list.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The source tag to use as the base of the image containing the new layer.
  *image* must be a path to a valid OCI image and *tag* must be a valid tag in
  the image. If *tag* is not provided it defaults to "latest".

**--tag**=*tag*
  The destination tag to use for the newly created image. *tag* must be a valid
  tag in the image. If *tag* is not provided it defaults to the *tag* specified
  in **--image** (overwriting it).

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
  recommended for use with umoci-raw-add-layer(1), since it results in the
  history not including all of the image layers -- and thus will cause
  confusion with tools that look at image history.**

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

# EXAMPLE

The following takes an existing diff directory, creates a new archive from it
and then inserts it into an existing image. Note that the new archive is *not*
compressed (**umoci** will compress the archive for you).

```
% tar cfC diff-layer.tar diff/ .
% umoci raw add-layer --image oci:foo diff-layer.tar
```

# SEE ALSO
**umoci**(1), **umoci-repack**(1)
