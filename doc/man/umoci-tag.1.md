% umoci-tag(1) # umoci tag - Create tags in OCI images
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci tag - Create tags in OCI images

# SYNOPSIS
**umoci tag**
**--image**=*image*[:*tag*]
*new-tag*

# DESCRIPTION
Creates a new tag that is a copy of *tag* with the name *new-tag*. If *new-tag*
already exists, it will be replaced. The original *tag* will be unchanged.

# OPTIONS

**--image**=*image*[:*tag*]
  The source OCI image tag to create a copy of. *image* must be a path to a
  valid OCI image and *tag* must be a valid tag in the image. If *tag* is not
  provided it defaults to "latest".

# EXAMPLE
The following swaps two image tags in an OCI image.

```
% umoci tag --image image:to-change new
% umoci tag --image image:latest to-change
% umoci tag --image image:new latest
% umoci rm --image image:new
```

# SEE ALSO
**umoci**(1), **umoci-remove**(1)
