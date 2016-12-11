% umoci-list(1) # umoci list - List tags in an OCI image
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci list - List tags in an OCI image

# SYNOPSIS
**umoci list**
**--layout**=*image*

**umoci ls**
**--layout**=*image*

# DESCRIPTION
Gets the list of tags defined in an OCI image, with one tag name per line. The
output order is not defined.

# OPTIONS

**--layout**=*image*
  The OCI image layout to get the list of tags from. *image* must be a path to
  a valid OCI image.

# EXAMPLE

The following lists the set of tags in an image copied from a **docker**(1)
registry using **skopeo**(1).

```
% skopeo copy docker://opensuse/amd64:42.1 oci:image:42.1
% skopeo copy docker://opensuse/amd64:42.2 oci:image:42.2
% skopeo copy docker://opensuse/amd64:latest oci:image:latest
% umoci ls --layout image
42.1
42.2
latest
```

# SEE ALSO
**umoci**(1), **umoci-stat**(1)
