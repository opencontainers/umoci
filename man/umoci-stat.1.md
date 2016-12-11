% umoci-stat(1) # umoci stat - Display status information about an image tag
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci stat - Display status information about an image tag

# SYNOPSIS
**umoci stat**
**--image**=*image*[:*tag*]
[**--json**]

# DESCRIPTION
Generates various pieces of status information about an image tag, including
the history of the image.

**WARNING**: Do not depend on the output of this tool unless you are using the
**--json** flag. The intention of the default formatting of this tool is to
make it human-readable, and might change in future versions. For parseable
and stable output, use **--json**.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The OCI image tag to display information about. *image* must be a path to a
  valid OCI image and *tag* must be a valid tag in the image. If *tag* is not
  provided it defaults to "latest".

**--json**
  Output the status information as a JSON encoded blob.

# FORMAT
The format of the **--json** blob is as follows. Many of these fields come from
the [OCI image specification][1].

    {
      # This is the set of history entries for the image.
      "history": [
        {
          "layer":       <descriptor>, # null if empty_layer is true
          "diff_id":     <diffid>,
          "created":     <created>,
          "created_by":  <created_by>,
          "author":      <author>,
          "empty_layer": <empty_layer>
        }...
      ]
    }

In future versions of **umoci**(1) there may be extra fields added to the above
structure. However, the currently defined fields will always be set (until a
backwards-incompatible release is made).

# SEE ALSO
**umoci**(1)

[1]: https://github.com/opencontainers/image-spec
