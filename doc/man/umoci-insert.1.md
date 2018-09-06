% umoci-insert(1) # umoci insert - insert a file into an OCI image
% Aleksa Sarai
% SEPTEMBER 2018
# NAME
umoci insert - insert a file into an OCI image

# SYNOPSIS
**umoci insert**
**--image**=*image*[:*tag*]
[**--rootless**]
[**--uid-map**=*value*]
[**--uid-map**=*value*]
[**--history.comment**=*comment*]
[**--history.created_by**=*created_by*]
[**--history.author**=*author*]
[**--history-created**=*date*]
*file*
*path*

# DESCRIPTION
Creates a new OCI image layout. The new OCI image does not contain any new
references or blobs, but those can be created through the use of
**umoci-new**(1), **umoci-tag**(1), **umoci-repack**(1) and other similar
commands.

Inserts *file* into the OCI image given by **--image** (overwriting it),
creating a new layer containing just the contents of *file* at *path*. *file*
can be a file or a directory to insert (in the latter case the directory is
always recursed), and *path* is the full path where *file* will be inserted.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The source and destination tag for the insertion of *file* at *path* inside
  the container image. *image* must be a path to a valid OCI image and *tag*
  must be a valid tag in the image. If *tag* is not provided it defaults to
  "latest".

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

# SEE ALSO
**umoci**(1), **umoci-repack**(1), **umoci-raw-add-layer**(1)
