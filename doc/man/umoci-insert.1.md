% umoci-insert(1) # umoci insert - insert content into an OCI image
% Aleksa Sarai
% SEPTEMBER 2018
# NAME
umoci insert - insert content into an OCI image

# SYNOPSIS
**umoci insert**
**--image**=*image*[:*tag*]
[**--tag**=*new-tag*]
[**--compress**=*compression-type*]
[**--opaque**]
[**--rootless**]
[**--uid-map**=*value*]
[**--uid-map**=*value*]
[**--no-history**]
[**--history.comment**=*comment*]
[**--history.created_by**=*created_by*]
[**--history.author**=*author*]
[**--history-created**=*date*]
*source*
*target*

**umoci insert**
[options]
**--whiteout**
*target*


# DESCRIPTION
In the first form, insert the contents of *source* into the OCI image given by
**--image** -- **overwriting it unless you specify --tag**. This is done by
creating a new layer containing just the contents of *source* with a name of
*target*. *source* can be either a file or directory, and in the latter case it
will be recursed. If **--opaque** is specified then any paths below *target* in
the previous image layers (assuming *target* is a directory) will be removed.

In the second form, inserts a "deletion entry" into the OCI image for *target*
inside the image. This is done by inserting a layer containing just a whiteout
entry for the given path.

Note that this command works by creating a new layer, so this should not be
used to remove (or replace) secrets from an already-built image. See
**umoci-config**(1) and **--config.volume** for how to achieve this correctly
by not creating image layers with secrets in the first place.

If **--no-history** was not specified, a history entry is appended to the
tagged OCI image for this change (with the various **--history.** flags
controlling the values used). To view the history, see **umoci-stat**(1).

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The source and destination tag for the insertion of *file* at *path* inside
  the container image. *image* must be a path to a valid OCI image and *tag*
  must be a valid tag in the image. If *tag* is not provided it defaults to
  "latest".

**--tag**=*new-tag*
  Tag name for the modified image, if unspecified then the original tag
  provided to **--image** will be clobbered.

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

**--opaque**
  (Assuming *target* is a directory.) Add an opaque whiteout entry for *target*
  so that any child path of *target* in previous layers is masked by the new
  entry for *target*, which will just contain the contents of *source*. This
  allows for the complete replacement of a directory, as opposed to the merging
  of directory entries.

**--whiteout**
  Add a deletion entry for *target*, so that it is not present in future
  extractions of the image.

**--rootless**
  Enable rootless insertion support. This allows for **umoci-insert**(1) to be
  used as an unprivileged user. Use of this flag implies **--uid-map=0:$(id
  -u):1** and **--gid-map=0:$(id -g):1**, as well as enabling several features
  to fake parts of the recursion process in an attempt to generate an
  as-close-as-possible clone of the filesystem for insertion.

**--uid-map**=*value*
  Specifies a UID mapping to use when inserting files. This is used in a
  similar fashion to **user_namespaces**(7), and is of the form
  **container:host[:size]**.

**--gid-map**=*value*
  Specifies a GID mapping to use when inserting files. This is used in a
  similar fashion to **user_namespaces**(7), and is of the form
  **container:host[:size]**.

**--no-history**
  Causes no history entry to be added for this operation. **This is not
  recommended for use with umoci-insert(1), since it results in the history not
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

# EXAMPLE

The following inserts a file `mybinary` into the path `/usr/bin/mybinary` and a
directory `myconfigdir` into the path `/etc/myconfigdir`. It should be noted
that if `/etc/myconfigdir` already exists in the image, the contents of the two
directories are merged (with the newer layer taking precedence).

```
% umoci insert --image oci:foo mybinary /usr/bin/mybinary
% umoci insert --image oci:foo myconfigdir /etc/myconfigdir
```

And in these examples we delete `/usr/bin/mybinary` and replace the entirety of
`/etc` with `myetcdir` (such that none of the old `/etc` entries will be
present on **umoci-unpack**(1)).

```
% umoci insert --image oci:foo --whiteout /usr/bin/mybinary
% umoci insert --image oci:foo --opaque myetcdir /etc
```

# SEE ALSO
**umoci**(1), **umoci-repack**(1), **umoci-raw-add-layer**(1)
