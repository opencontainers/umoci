% umoci-gc(1) # umoci gc - Garbage collects all unreferenced OCI image blobs
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci gc - Garbage collects all unreferenced OCI image blobs

# SYNOPSIS
**umoci gc**
**--layout**=*image*
[**--digest-regexp**=*regexp*]

# DESCRIPTION
Conduct a mark-and-sweep garbage collection of the provided OCI image, only
retaining blobs which can be reached by a descriptor path from the root set of
tags. All other blobs will be removed.

# OPTIONS
The global options are defined in **umoci**(1).

**--layout**=*image*
  The OCI image layout to be garbage collected. *image* must be a path to a
  valid OCI image.

**--digest-regexp**=*regexp*
  A regular expression for calculating the digest from a filesystem
  path.  This is required if your oci-layout declares an
  `oci-cas-template-v1` CAS engine.  For example, if you created the
  image with:

    umoci init --blob-uri file:///path/to/my/blobs/{algorithm}/{encoded:2}/{encoded}

  Then you should set *regexp* to:

    ^.*/(?P<algorithm>[a-z0-9+._-]+)/[a-zA-Z0-9=_-]{1,2}/(?P<encoded>[a-zA-Z0-9=_-]{1,})$

# EXAMPLE

The following deletes a tag from an OCI image and clean conducts a garbage
collection in order to clean up the remaining unused blobs.

```
% umoci rm --image image:sometag
% umoci gc --layout image
```

# SEE ALSO
**umoci**(1), **umoci-remove**(1)
