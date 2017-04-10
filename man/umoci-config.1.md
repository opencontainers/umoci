% umoci-config(1) # umoci config - Modifies the configuration of an OCI image
% Aleksa Sarai
% DECEMBER 2016
# NAME
umoci config - Modifies the configuration of an OCI image

# SYNOPSIS
**umoci config**
**--image**=*image*[:*tag*]
[**--tag**=*new-tag*]
[**--history.comment**=*comment*]
[**--history.created_by**=*created_by*]
[**--history.author**=*author*]
[**--history-created**=*date*]
[**--clear**=*value*]
[**--config.user**=*value*]
[**--config.exposedports**=*value*]
[**--config.env**=*value*]
[**--config.entrypoint**=*value*]
[**--config.cmd**=*value*]
[**--config.volume**=*value*]
[**--config.label**=*value*]
[**--config.workingdir**=*value*]
[**--created**=*value*]
[**--author**=*value*]
[**--architecture**=*value*]
[**--os**=*value*]
[**--manifest.annotation**=*value*]

# DESCRIPTION
Modify the configuration and manifest data for a particular tagged OCI image.
In addition, a history entry is appended to the tagged OCI image for this
change (with the various **--history.** flags controlling the values used). To
view the history, see **umoci-stat**(1).

Note that the original image tag (the argument to **--image**) will **not** be
modified unless the target of **umoci-config**(1) is the original image tag.

# OPTIONS
The global options are defined in **umoci**(1).

**--image**=*image*[:*tag*]
  The source tagged OCI image whose config will be modified. *image* must be
  a path to a valid OCI image and *tag* must be a valid tag in the image. If
  *tag* is not provided it defaults to "latest".

**--tag**=*new-tag*
  Tag name for the repacked image, if unspecified then the original tag
  provided to **--image** will be clobbered.

**--history.comment**=*comment*
  Comment for the history entry corresponding to this modification of the image
  configuration. If unspecified, **umoci**(1) will generate an
  implementation-dependent value.

**--history.created_by**=*created_by*
  CreatedBy entry for the history entry corresponding to this modification of
  the image configuration. If unspecified, **umoci**(1) will generate an
  implementation-dependent value.

**--history.author**=*author*
  Author value for the history entry corresponding to this modification of the
  image configuration. If unspecified, this value will be the image's author
  value **after** any modifications were made by this call of
  **umoci-config**(1).

**--history-created**=*date*
  Creation date for the history entry corresponding to this modifications of
  the image configuration. This must be an ISO8601 formatted timestamp (see
  **date**(1)). If unspecified, the current time is used.

**--clear**=*value*
  Removes all pre-existing entries for a given set or list configuration option
  (it will not undo any modification made by this call of **umoci-config**(1)).
  The valid values of *value* are:

    * config.labels
    * manifest.annotations
    * config.exposedports
    * config.env
    * config.entrypoint
    * config.cmd
    * config.volume

The following commands all set their corresponding values in the configuration
or image manifest. For more information see [the OCI image specification][1].

* **--config.user**=*value*
* **--config.exposedports**=*value*
* **--config.env**=*value*
* **--config.entrypoint**=*value*
* **--config.cmd**=*value*
* **--config.volume**=*value*
* **--config.label**=*value*
* **--config.workingdir**=*value*
* **--created**=*value*
* **--author**=*value*
* **--architecture**=*value*
* **--os**=*value*
* **--manifest.annotation**=*value*

# EXAMPLE

The following modifies an OCI image configuration in various ways, and
overwrites the original tag with the new image.

```
% umoci config --image image:tag --clear=config.env --config.env="VARIABLE=true" \
	--config.user="user:group" --config.entrypoint=cat --config.cmd=/proc/self/stat \
	--config.label="com.cyphar.umoci=true" --author="Aleksa Sarai <asarai@suse.de>" \
	--os="gnu/hurd" --architecture="lisp" --created="$(date --iso-8601=seconds)"
```

# SEE ALSO
**umoci**(1)

[1]: https://github.com/opencontainers/image-spec
