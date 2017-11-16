% umoci-list(1) # umoci list - List tags in an OCI layout
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci list - List tags in an OCI layout

# SYNOPSIS
**umoci list**
**--layout**=*layout*

**umoci ls**
**--layout**=*layout*

# DESCRIPTION
Gets the list of tags defined in an OCI layout, with one tag name per line. The
output order is not defined.

# OPTIONS

**--layout**=*layout*
  The OCI image layout to get the list of tags from. *layout* must be a path to
  a valid OCI layout.

# EXAMPLE

The following lists the set of tags in a layout copied from a **docker**(1)
registry using **skopeo**(1).

```
% skopeo copy docker://opensuse/amd64:42.1 oci:ocidir:42.1
% skopeo copy docker://opensuse/amd64:42.2 oci:ocidir:42.2
% skopeo copy docker://opensuse/amd64:latest oci:ocidir:latest
% umoci ls --layout ocidir
42.1
42.2
latest
```

# SEE ALSO
**umoci**(1), **umoci-stat**(1)
