% umoci-new(1) # umoci new - Create a blank tag in an OCI image
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci new - Create a blank tag in an OCI image

# SYNOPSIS
**umoci new**
**--image**=*image*[:*tag*]

# DESCRIPTION
Create a blank tag in an OCI image. The created image's configuration and
manifest are all set to **umoci**-defined default values, with no filesystem
layer blobs added to the image.

Once a new image is created with **umoci-new**(1) you can directly use the
image with **umoci-unpack**(1), **umoci-repack**(1), and **umoci-config**(1) to
modify the new tagged image as you see fit. This allows you to create entirely
new images from scratch, without needing a base image to start with.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The destination of the blank tag in the OCI image. *image* must be a path to
  a valid OCI image, and *tag* must be a valid tag name. If a tag already
  exists with the name *tag* it will be overwritten. If *tag* is not provided
  it defaults to "latest".

# EXAMPLE
The following creates a brand new OCI image layout and then creates a blank tag
for further manipulation with **umoci-repack**(1) and **umoci-config**(1).

```
% umoci init --layout image
% umoci new --image image:tag
```

# SEE ALSO
**umoci**(1), **umoci-unpack**(1), **umoci-repack**(1), **umoci-config**(1)

