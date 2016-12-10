% umoci-init(1) # umoci init - Modifies the inituration of an OCI image
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci init - Modifies the inituration of an OCI image

# SYNOPSIS
**umoci init**
**--layout**=*image*

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

# SEE ALSO
**umoci**(1), **umoci-new**(1)
