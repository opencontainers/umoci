% umoci-remove(1) # umoci tag - Remove tags from OCI images
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci remove - Removes tags from OCI images

# SYNOPSIS
**umoci remove**
**--image**=*image*[:*tag*]

**umoci rm**
**--image**=*image*[:*tag*]

# DESCRIPTION
Removes the given tag from the OCI image. The relevant blobs are **not**
removed -- in order to remove all unused blobs see **umoci-gc**(1).

# OPTIONS

**--image**=*image*[:*tag*]
  The source OCI image tag to remove. *image* must be a path to a valid OCI
  image and *tag* must be a valid tag name (**umoci-remove**(1) does not return
  an error if the tag did not exist). If *tag* is not provided it defaults to
  "latest".

# EXAMPLE
The following creates a copy of a tag and then deletes the original.

```
% umoci tag --image image:tag new-tag
% umoci rm --image image:tag
```

# SEE ALSO
**umoci**(1), **umoci-tag**(1), **umoci-gc**(1)
