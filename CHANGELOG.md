<!--
+++
# Hugo Front-matter
title = "Changelog"
aliases = ["/CHANGELOG.md"]
+++
-->

# Changelog #
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased] ##

## [0.4.7] - 2021-04-05 ##

### Security ###
- A security flaw was found in umoci, and has been fixed in this release. If
  umoci was used to unpack a malicious image (using either `umoci unpack` or
  `umoci raw unpack`) that contained a symlink entry for `/.`, umoci would
  apply subsequent layers to the target of the symlink (resolved on the host
  filesystem). This means that if you ran umoci as root, a malicious image
  could overwrite any file on the system (assuming you didn't have any other
  access control restrictions). CVE-2021-29136

### Added ###
- umoci now compiles on FreeBSD and appears to work, with the notable
  limitation that it currently refuses to extract non-Linux images on any
  platform (this will be fixed in a future release -- see #364). #357
- Initial fuzzer implementations for oss-fuzz. #365

### Changed ###
- umoci will now read all trailing data from image layers, to combat the
  existence of some image generators that appear to append NUL bytes to the end
  of the gzip stream (which would previously cause checksum failures because we
  didn't read nor checksum the trailing junk bytes). However, umoci will still
  not read past the descriptor length. #360
- umoci now ignores all overlayfs xattrs during unpack and repack operations,
  to avoid causing issues when packing a raw overlayfs directory. #354
- Changes to the (still-internal) APIs to allow for users to use umoci more
  effectively as a library.
  - The garbage collection API now supports custom GC policies. #338
  - The mutate API now returns information about what layers were added by the
    operation. #344
  - The mutate API now supports custom compression, and has in-tree support for
    zstd. #348 #350
  - Support overlayfs-style whiteouts during unpack and repack. #342

## [0.4.6] - 2020-06-24 ##
umoci has been adopted by the Open Container Initative as a reference
implementation of the OCI Image Specification. This will have little impact on
the roadmap or scope of umoci, but it does further solidify umoci as a useful
piece of "boring container infrastructure" that can be used to build larger
systems.

### Changed ###
- As part of the adoption procedure, the import path and module name of umoci
  has changed from `github.com/openSUSE/umoci` to
  `github.com/opencontainers/umoci`. This means that users of our (still
  unstable) Go API will have to change their import paths in order to update to
  newer versions of umoci.

  The old GitHub project will contain a snapshot of `v0.4.5` with a few minor
  changes to the readme that explain the situation. Go projects which import
  the archived project will receive build warnings that explain the need to
  update their import paths.

### Added ###
- umoci now builds on MacOS, and we currently run the unit tests on MacOS to
  hopefully catch core regressions (in the future we will get the integration
  tests running to catch more possible regressions). opencontainers/umoci#318

### Fixed ###
- Suppress repeated xattr warnings on destination filesystems that do not
  support xattrs. opencontainers/umoci#311
- Work around a long-standing issue in our command-line parsing library (see
  urfave/cli#1152) by disabling argument re-ordering for `umoci config`, which
  often takes `-`-prefixed flag arguments. opencontainers/umoci#328

## [0.4.5] - 2019-12-04 ##
### Added ###
- Expose umoci subcommands as part of the API, so they can be used by other Go
  projects. opencontainers/umoci#289
- Add extensible hooking to the core libraries in umoci, to allow for
  third-party media-types to be treated just like first-party ones (the key
  difference is the introspection and parsing logic). opencontainers/umoci#299
  opencontainers/umoci#307

### Fixed ###
- Use `type: bind` for generated `config.json` bind-mounts. While this doesn't
  make too much sense (see opencontainers/runc#2035), it does mean that
  rootless containers work properly with newer `runc` releases (which appear to
  have regressed when handling file-based bind-mounts with a "bad" `type`).
  opencontainers/umoci#294 opencontainers/umoci#295
- Don't insert a new layer if there is no diff. opencontainers/umoci#293
- Only output a warning if forbidden extended attributes are present inside the
  tar archive -- otherwise we fail on certain (completely broken) Docker
  images. opencontainers/umoci#304

## [0.4.4] - 2019-01-30 ##
### Added ###
- Full-stack verification of blob hashes and descriptor sizes is now done on
  all operations, improving our hardening against bad blobs (we already did
  some verification of layer DiffIDs but this is far more thorough).
  opencontainers/umoci#278 opencontainers/umoci#280 opencontainers/umoci#282

## [0.4.3] - 2018-11-11 ##
### Added ###
- All umoci commands that had `--history.*` options can now decide to omit a
  history entry with `--no-history`. Note that while this is supported for
  commands that create layers (`umoci repack`, `umoci insert`, and `umoci raw
  add-layer`) it is not recommended to use it for those commands since it can
  cause other tools to become confused when inspecting the image history. The
  primary usecase is to allow `umoci config --no-history` to leave no traces in
  the history. See SUSE/kiwi#871. opencontainers/umoci#270
- `umoci insert` now has a `--tag` option that allows you to non-destructively
  insert files into an image. The semantics match `umoci config --tag`.
  opencontainers/umoci#273

## [0.4.2] - 2018-09-11 ##
### Added ###
- umoci now has an exposed Go API. At the moment it's unclear whether it will
  be changed significantly, but at the least now users can use
  umoci-as-a-library in a fairly sane way. opencontainers/umoci#245
- Added `umoci unpack --keep-dirlinks` (in the same vein as rsync's flag with
  the same name) which allows layers that contain entries which have a symlink
  as a path component. opencontainers/umoci#246
- `umoci insert` now supports whiteouts in two significant ways. You can use
  `--whiteout` to "insert" a deletion of a given path, while you can use
  `--opaque` to replace a directory by adding an opaque whiteout (the default
  behaviour causes the old and new directories to be merged).
  opencontainers/umoci#257

### Fixed ###
- Docker has changed how they handle whiteouts for non-existent files. The
  specification is loose on this (and in umoci we've always been liberal with
  whiteout generation -- to avoid cases where someone was confused we didn't
  have a whiteout for every entry). But now that they have deviated from the
  spec, in the interest of playing nice, we can just follow their new
  restriction (even though it is not supported by the spec). This also makes
  our layers *slightly* smaller. opencontainers/umoci#254
- `umoci unpack` now no longer erases `system.nfs4_acl` and also has some more
  sophisticated handling of forbidden xattrs. opencontainers/umoci#252
  opencontainers/umoci#248
- `umoci unpack` now appears to work correctly on SELinux-enabled systems
  (previously we had various issues where `umoci` wouldn't like it when it was
  trying to ensure the filesystem was reproducibly generated and SELinux xattrs
  would act strangely). To fix this, now `umoci unpack` will only cause errors
  if it has been asked to change a forbidden xattr to a value different than
  it's current on-disk value. opencontainers/umoci#235 opencontainers/umoci#259

## [0.4.1] - 2018-08-16 ##
### Added ###
- The number of possible tags that are now valid with `umoci` subcommands has
  increased significantly due to an expansion in the specification of the
  format of the `ref.name` annotation. To quote the specification, the
  following is the EBNF of valid `refname` values. opencontainers/umoci#234
  ```
  refname   ::= component ("/" component)*
  component ::= alphanum (separator alphanum)*
  alphanum  ::= [A-Za-z0-9]+
  separator ::= [-._:@+] | "--"
  ```
- A new `umoci insert` subcommand which adds a given file to a path inside the
  container. opencontainers/umoci#237
- A new `umoci raw unpack` subcommand in order to allow users to unpack images
  without needing a configuration or any of the manifest generation.
  opencontainers/umoci#239
- `umoci` how has a logo. Thanks to [Max Bailey][maxbailey] for contributing
  this to the project. opencontainers/umoci#165 opencontainers/umoci#249

### Fixed ###
- `umoci unpack` now handles out-of-order regular whiteouts correctly (though
  this ordering is not recommended by the spec -- nor is it required). This is
  an extension of opencontainers/umoci#229 that was missed during review.
  opencontainers/umoci#232
- `umoci unpack` and `umoci repack` now make use of a far more optimised `gzip`
  compression library. In some benchmarks this has resulted in `umoci repack`
  speedups of up to 3x (though of course, you should do your own benchmarks).
  `umoci unpack` unfortunately doesn't have as significant of a performance
  improvement, due to the nature of `gzip` decompression (in future we may
  switch to `zlib` wrappers). opencontainers/umoci#225 opencontainers/umoci#233

[maxbailey]: http://www.maxbailey.me/

## [0.4.0] - 2018-03-10 ##
### Added ###
- `umoci repack` now supports `--refresh-bundle` which will update the
  OCI bundle's metadata (mtree and umoci-specific manifests) after packing the
  image tag. This means that the bundle can be used as a base layer for
  future diffs without needing to unpack the image again. opencontainers/umoci#196
- Added a website, and reworked the documentation to be better structured. You
  can visit the website at [`umo.ci`][umo.ci]. opencontainers/umoci#188
- Added support for the `user.rootlesscontainers` specification, which allows
  for persistent on-disk emulation of `chown(2)` inside rootless containers.
  This implementation is interoperable with [@AkihiroSuda's `PRoot`
  fork][as-proot-fork] (though we do not test its interoperability at the
  moment) as both tools use [the same protobuf
  specification][rootlesscontainers-proto]. opencontainers/umoci#227
- `umoci unpack` now has support for opaque whiteouts (whiteouts which remove
  all children of a directory in the lower layer), though `umoci repack` does
  not currently have support for generating them. While this is technically a
  spec requirement, through testing we've never encountered an actual user of
  these whiteouts. opencontainers/umoci#224 opencontainers/umoci#229
- `umoci unpack` will now use some rootless tricks inside user namespaces for
  operations that are known to fail (such as `mknod(2)`) while other operations
  will be carried out as normal (such as `lchown(2)`). It should be noted that
  the `/proc/self/uid_map` checking we do can be tricked into not detecting
  user namespaces, but you would need to be trying to break it on purpose.
  opencontainers/umoci#171 opencontainers/umoci#230

### Fixed ###
- Fix a bug in our "parent directory restore" code, which is responsible for
  ensuring that the mtime and other similar properties of a directory are not
  modified by extraction inside said directory. The bug would manifest as
  xattrs not being restored properly in certain edge-cases (which we
  incidentally hit in a test-case). opencontainers/umoci#161 opencontainers/umoci#162
- `umoci unpack` will now "clean up" the bundle generated if an error occurs
  during unpacking. Previously this didn't happen, which made cleaning up the
  responsibility of the caller (which was quite difficult if you were
  unprivileged). This is a breaking change, but is in the error path so it's
  not critical. opencontainers/umoci#174 opencontainers/umoci#187
- `umoci gc` now will no longer remove unknown files and directories that
  aren't `flock(2)`ed, thus ensuring that any possible OCI image-spec
  extensions or other users of an image being operated on will no longer
  break.  opencontainers/umoci#198
- `umoci unpack --rootless` will now correctly handle regular file unpacking
  when overwriting a file that `umoci` doesn't have write access to. In
  addition, the semantics of pre-existing hardlinks to a clobbered file are
  clarified (the hard-links will not refer to the new layer's inode).
  opencontainers/umoci#222 opencontainers/umoci#223

[as-proot-fork]: https://github.com/AkihiroSuda/runrootless
[rootlesscontainers-proto]: https://rootlesscontaine.rs/proto/rootlesscontainers.proto
[umo.ci]: https://umo.ci/

## [0.3.1] - 2017-10-04 ##
### Fixed ###
- Fix several minor bugs in `hack/release.sh` that caused the release artefacts
  to not match the intended style, as well as making it more generic so other
  projects can use it. opencontainers/umoci#155 opencontainers/umoci#163
- A recent configuration issue caused `go vet` and `go lint` to not run as part
  of our CI jobs. This means that some of the information submitted as part of
  [CII best practices badging][cii] was not accurate. This has been corrected,
  and after review we concluded that only stylistic issues were discovered by
  static analysis. opencontainers/umoci#158
- 32-bit unit test builds were broken in a refactor in [0.3.0]. This has been
  fixed, and we've added tests to our CI to ensure that something like this
  won't go unnoticed in the future. opencontainers/umoci#157
- `umoci unpack` would not correctly preserve set{uid,gid} bits. While this
  would not cause issues when building an image (as we only create a manifest
  of the final extracted rootfs), it would cause issues for other users of
  `umoci`. opencontainers/umoci#166 opencontainers/umoci#169
- Updated to [v0.4.1 of `go-mtree`][gomtree-v0.4.1], which fixes several minor
  bugs with manifest generation. opencontainers/umoci#176
- `umoci unpack` would not handle "weird" tar archive layers previously (it
  would error out with DiffID errors). While this wouldn't cause issues for
  layers generated using Go's `archive/tar` implementation, it would cause
  issues for GNU gzip and other such tools. opencontainers/umoci#178
  opencontainers/umoci#179

### Changed ###
- `umoci unpack`'s mapping options (`--uid-map` and `--gid-map`) have had an
  interface change, to better match the [`user_namespaces(7)`][user_namespaces]
  interfaces. Note that this is a **breaking change**, but the workaround is to
  switch to the trivially different (but now more consistent) format.
  opencontainers/umoci#167

### Security ###
- `umoci unpack` used to create the bundle and rootfs with world
  read-and-execute permissions by default. This could potentially result in an
  unsafe rootfs (containing dangerous setuid binaries for instance) being
  accessible by an unprivileged user. This has been fixed by always setting the
  mode of the bundle to `0700`, which requires a user to explicitly work around
  this basic protection. This scenario was documented in our security
  documentation previously, but has now been fixed. opencontainers/umoci#181
  opencontainers/umoci#182

[cii]: https://bestpractices.coreinfrastructure.org/projects/1084
[gomtree-v0.4.1]: https://github.com/vbatts/go-mtree/releases/tag/v0.4.1
[user_namespaces]: http://man7.org/linux/man-pages/man7/user_namespaces.7.html

## [0.3.0] - 2017-07-20 ##
### Added ###
- `umoci` now passes all of the requirements for the [CII best practices bading
  program][cii]. opencontainers/umoci#134
- `umoci` also now has more extensive architecture, quick-start and roadmap
  documentation. opencontainers/umoci#134
- `umoci` now supports [`1.0.0` of the OCI image
  specification][ispec-v1.0.0] and [`1.0.0` of the OCI runtime
  specification][rspec-v1.0.0], which are the first milestone release. Note
  that there are still some remaining UX issues with `--image` and other parts
  of `umoci` which may be subject to change in future versions. In particular,
  this update of the specification now means that images may have ambiguous
  tags. `umoci` will warn you if an operation may have an ambiguous result, but
  we plan to improve this functionality far more in the future.
  opencontainers/umoci#133 opencontainers/umoci#142
- `umoci` also now supports more complicated descriptor walk structures, and
  also handles mutation of such structures more sanely. At the moment, this
  functionality has not been used "in the wild" and `umoci` doesn't have the UX
  to create such structures (yet) but these will be implemented in future
  versions. opencontainers/umoci#145
- `umoci repack` now supports `--mask-path` to ignore changes in the rootfs
  that are in a child of at least one of the provided masks when generating new
  layers. opencontainers/umoci#127

### Changed ###
- Error messages from `github.com/opencontainers/umoci/oci/cas/drivers/dir` actually
  make sense now. opencontainers/umoci#121
- `umoci unpack` now generates `config.json` blobs according to the [still
  proposed][ispec-pr492] OCI image specification conversion document.
  opencontainers/umoci#120
- `umoci repack` also now automatically adding `Config.Volumes` from the image
  configuration to the set of masked paths.  This matches recently added
  [recommendations by the spec][ispec-pr694], but is a backwards-incompatible
  change because the new default is that `Config.Volumes` **will** be masked.
  If you wish to retain the old semantics, use `--no-mask-volumes` (though make
  sure to be aware of the reasoning behind `Config.Volume` masking).
  opencontainers/umoci#127
- `umoci` now uses [`SecureJoin`][securejoin] rather than a patched version of
  `FollowSymlinkInScope`. The two implementations are roughly equivalent, but
  `SecureJoin` has a nicer API and is maintained as a separate project.
- Switched to using `golang.org/x/sys/unix` over `syscall` where possible,
  which makes the codebase significantly cleaner. opencontainers/umoci#141

[cii]: https://bestpractices.coreinfrastructure.org/projects/1084
[rspec-v1.0.0]: https://github.com/opencontainers/runtime-spec/releases/tag/v1.0.0
[ispec-v1.0.0]: https://github.com/opencontainers/image-spec/releases/tag/v1.0.0
[ispec-pr492]: https://github.com/opencontainers/image-spec/pull/492
[ispec-pr694]: https://github.com/opencontainers/image-spec/pull/694
[securejoin]: https://github.com/cyphar/filepath-securejoin

## [0.2.1] - 2017-04-12 ##
### Added ###
- `hack/release.sh` automates the process of generating all of the published
  artefacts for releases. The new script also generates signed source code
  archives. opencontainers/umoci#116

### Changed ###
- `umoci` now outputs configurations that are compliant with [`v1.0.0-rc5` of
  the OCI runtime-spec][rspec-v1.0.0-rc5]. This means that now you can use runc
  v1.0.0-rc3 with `umoci` (and rootless containers should work out of the box
  if you use a development build of runc). opencontainers/umoci#114
- `umoci unpack` no longer adds a dummy linux.seccomp entry, and instead just
  sets it to null. opencontainers/umoci#114

[rspec-v1.0.0-rc5]: https://github.com/opencontainers/runtime-spec/releases/tag/v1.0.0-rc5

## [0.2.0] - 2017-04-11 ##
### Added ###
- `umoci` now has some automated scripts for generated RPMs that are used in
  openSUSE to automatically submit packages to OBS. opencontainers/umoci#101
- `--clear=config.{cmd,entrypoint}` is now supported. While this interface is a
  bit weird (`cmd` and `entrypoint` aren't treated atomically) this makes the
  UX more consistent while we come up with a better `cmd` and `entrypoint` UX.
  opencontainers/umoci#107
- New subcommand: `umoci raw runtime-config`. It generates the runtime-spec
  config.json for a particular image without also unpacking the root
  filesystem, allowing for users of `umoci` that are regularly parsing
  `config.json` without caring about the root filesystem to be more efficient.
  However, a downside of this approach is that some image-spec fields
  (`Config.User`) require a root filesystem in order to make sense, which is
  why this command is hidden under the `umoci-raw(1)` subcommand (to make sure
  only users that understand what they're doing use it). opencontainers/umoci#110

### Changed ###
- `umoci`'s `oci/cas` and `oci/config` libraries have been massively refactored
  and rewritten, to allow for third-parties to use the OCI libraries. The plan
  is for these to eventually become part of an OCI project. opencontainers/umoci#90
- The `oci/cas` interface has been modifed to switch from `*ispec.Descriptor`
  to `ispec.Descriptor`. This is a breaking, but fairly insignificant, change.
  opencontainers/umoci#89

### Fixed ###
- `umoci` now uses an updated version of `go-mtree`, which has a complete
  rewrite of `Vis` and `Unvis`. The rewrite ensures that unicode handling is
  handled in a far more consistent and sane way. opencontainers/umoci#88
- `umoci` used to set `process.user.additionalGids` to the "normal value" when
  unpacking an image in rootless mode, causing issues when trying to actually
  run said bundle with runC. opencontainers/umoci#109

## [0.1.0] - 2017-02-11 ##
### Added ###
- `CHANGELOG.md` has now been added. opencontainers/umoci#76

### Changed ###
- `umoci` now supports `v1.0.0-rc4` images, which has made fairly minimal
  changes to the schema (mainly related to `mediaType`s). While this change
  **is** backwards compatible (several fields were removed from the schema, but
  the specification allows for "additional fields"), tools using older versions
  of the specification may fail to operate on newer OCI images. There was no UX
  change associated with this update.

### Fixed ###
- `umoci tag` would fail to clobber existing tags, which was in contrast to how
  the rest of the tag clobbering commands operated. This has been fixed and is
  now consistent with the other commands. opencontainers/umoci#78
- `umoci repack` now can correctly handle unicode-encoded filenames, allowing
  the creation of containers that have oddly named files. This required fixes
  to go-mtree (where the issue was). opencontainers/umoci#80

## [0.0.0] - 2017-02-07 ##
### Added ###
- Unit tests are massively expanded, as well as the integration tests.
  opencontainers/umoci#68 opencontainers/umoci#69
- Full coverage profiles (unit+integration) are generated to get all
  information about how much code is tested. opencontainers/umoci#68
  opencontainers/umoci#69

### Fixed ###
- Static compilation now works properly. opencontainers/umoci#64
- 32-bit architecture builds are fixed. opencontainers/umoci#70

### Changed ###
- Unit tests can now be run inside `%check` of an `rpmbuild` script, allowing
  for proper testing. opencontainers/umoci#65.
- The logging output has been cleaned up to be much nicer for end-users to
  read. opencontainers/umoci#73
- Project has been moved to an openSUSE project. opencontainers/umoci#75

## [0.0.0-rc3] - 2016-12-19 ##
### Added ###
- `unpack`, `repack`: `xattr` support which also handles `security.selinux.*`
  difficulties. opencontainers/umoci#49 opencontainers/umoci#52
- `config`, `unpack`: Ensure that environment variables are not duplicated in
  the extracted or stored configurations. opencontainers/umoci#30
- Add support for read-only CAS operations for read-only filesystems.
  opencontainers/umoci#47
- Add some helpful output about `--rootless` if `umoci` fails with `EPERM`.
- Enable stack traces with errors if the `--debug` flag was given to `umoci`.
  This requires a patch to `pkg/errors`.

### Changed ###
- `gc`: Garbage collection now also garbage collects temporary directories.
  opencontainers/umoci#17
- Clean-ups to vendoring of `go-mtree` so that it's much more
  upstream-friendly.

## [0.0.0-rc2] - 2016-12-12 ##
### Added ###
- `unpack`, `repack`: Support for rootless unpacking and repacking.
  opencontainers/umoci#26
- `unpack`, `repack`: UID and GID mapping when unpacking and repacking.
  opencontainers/umoci#26
- `tag`, `rm`, `ls`: Tag modification commands such as `umoci tag`, `umoci rm`
  and `umoci ls`. opencontainers/umoci#6 opencontainers/umoci#27
- `stat`: Output information about an image. Currently only shows the history
  information. Only the **JSON** output is stable. opencontainers/umoci#38
- `init`, `new`: New commands have been created to allow for image creation
  from scratch. opencontainers/umoci#5 opencontainers/umoci#42
- `gc`: Garbage collection of images. opencontainers/umoci#6
- Full integration and unit testing, with OCI validation to ensure that we
  always create valid images. opencontainers/umoci#12

### Changed ###
- `unpack`, `repack`: Create history entries automatically (with options to
  modify the entries). opencontainers/umoci#36
- `unpack`: Store information about its source to ensure consistency when doing
  a `repack`. opencontainers/umoci#14
- The `--image` and `--from` arguments have been combined into a single
  `<path>[:<tag>]` argument for `--image`. opencontainers/umoci#39
- `unpack`: Configuration annotations are now extracted, though there are still
  some discussions happening upstream about the correct way of doing this.
  opencontainers/umoci#43

### Fixed ###
- `repack`: Errors encountered during generation of delta layers are now
  correctly propagated. opencontainers/umoci#33
- `unpack`: Hardlinks are now extracted as real hardlinks. opencontainers/umoci#25

### Security ###
- `unpack`, `repack`: Symlinks are now correctly resolved inside the unpacked
  rootfs. opencontainers/umoci#27

## 0.0.0-rc1 - 2016-11-10 ##
### Added ###
- Proof of concept with major functionality implemented.
  + `unpack`
  + `repack`
  + `config`

[Unreleased]: https://github.com/opencontainers/umoci/compare/v0.4.7...HEAD
[0.4.7]: https://github.com/opencontainers/umoci/compare/v0.4.6...v0.4.7
[0.4.6]: https://github.com/opencontainers/umoci/compare/v0.4.5...v0.4.6
[0.4.5]: https://github.com/opencontainers/umoci/compare/v0.4.4...v0.4.5
[0.4.4]: https://github.com/opencontainers/umoci/compare/v0.4.3...v0.4.4
[0.4.3]: https://github.com/opencontainers/umoci/compare/v0.4.2...v0.4.3
[0.4.2]: https://github.com/opencontainers/umoci/compare/v0.4.1...v0.4.2
[0.4.1]: https://github.com/opencontainers/umoci/compare/v0.4.0...v0.4.1
[0.4.0]: https://github.com/opencontainers/umoci/compare/v0.3.1...v0.4.0
[0.3.1]: https://github.com/opencontainers/umoci/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/opencontainers/umoci/compare/v0.2.1...v0.3.0
[0.2.1]: https://github.com/opencontainers/umoci/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/opencontainers/umoci/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/opencontainers/umoci/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/opencontainers/umoci/compare/v0.0.0-rc3...v0.0.0
[0.0.0-rc3]: https://github.com/opencontainers/umoci/compare/v0.0.0-rc2...v0.0.0-rc3
[0.0.0-rc2]: https://github.com/opencontainers/umoci/compare/v0.0.0-rc1...v0.0.0-rc2
