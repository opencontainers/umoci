% umoci-gc(1) # umoci gc - Garbage collects all unreferenced OCI image blobs
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci gc - Garbage collects all unreferenced OCI image blobs

# SYNOPSIS
**umoci gc**
**--layout**=*image*

# DESCRIPTION
Conduct a mark-and-sweep garbage collection of the provided OCI image, only
retaining blobs which can be reached by a descriptor path from the root set of
tags. All other blobs will be removed.

# OPTIONS
The global options are defined in **umoci**(1).

**--layout**=*image*
  The OCI image layout to be garbage collected. *image* must be a path to a
  valid OCI image.

# EXAMPLE

The following deletes a tag from an OCI image and clean conducts a garbage
collection in order to clean up the remaining unused blobs.

```
% umoci rm --image image:sometag
% umoci gc --layout image
```

# SEE ALSO
**umoci**(1), **umoci-remove**(1)
