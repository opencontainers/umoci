% umoci-raw(1) # umoci raw - Advanced internal image tooling
% Aleksa Sarai
% APRIL 2017
# NAME
umoci raw - Advanced internal image tooling

# SYNOPSIS
**umoci raw**
*command* [*args*]

# DESCRIPTION
**umoci-raw**(1) is a subcommand that contains further subcommands specifically
intended for "advanced" usage of **umoci**(1). Unless you are familiar with the
details of the OCI image specification, or otherwise understand what the
implications of using these features is, it is not recommended. The top-level
tools (**umoci-unpack**(1) and so on) should be sufficient for most use-cases.

# COMMANDS

**runtime-config, config**
  Generate an OCI runtime configuration for an image, without the rootfs. See
  **umoci-raw-runtime-config**(1) for more detailed usage information.

# SEE ALSO
**umoci**(1),
**umoci-raw-runtime-config**(1)
