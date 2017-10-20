% umoci-init(1) # umoci init - Create a new OCI image layout
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci init - Create a new OCI image layout

# SYNOPSIS
**umoci init**
**--layout**=*image*
[**--blob-uri**=*template*

# DESCRIPTION
Creates a new OCI image layout. The new OCI image does not contain any new
references or blobs, but those can be created through the use of
**umoci-new**(1), **umoci-tag**(1), **umoci-repack**(1) and other similar
commands.

# OPTIONS
The global options are defined in **umoci**(1).

**--layout**=*image*
  The path where the OCI image layout will be created. The path must not exist
  already or **umoci-init**(1) will return an error.

**--blob-uri**=*template*
  The URI Template for retrieving digests.  Relative URIs will be
  resolved with the image path as the base URI.  For more details,
  see the [OCI CAS Template Protocol][cas-template].

# EXAMPLE

The following creates a brand new OCI image layout and then creates a blank tag
for further manipulation with **umoci-repack**(1) and **umoci-config**(1).

```
% umoci init --layout image
% umoci new --image image:tag
```

# SEE ALSO
**umoci**(1), **umoci-new**(1)

[cas-template]: https://github.com/xiekeyang/oci-discovery/blob/0be7eae246ae9a975a76ca209c045043f0793572/cas-template.md
