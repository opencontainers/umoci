# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]
### Added
- `umoci` now passes all of the requirements for the [CII best practices bading
  program][cii]. openSUSE/umoci#134
- `umoci` also now has more extensive architecture, quick-start and roadmap
  documentation. openSUSE/umoci#134
- `umoci` now supports [`1.0.0` of the OCI image
  specification][ispec-v1.0.0] and [`1.0.0` of the OCI runtime
  specification][rspec-v1.0.0], which are the first milestone release. Note
  that there are still some remaining UX issues with `--image` and other parts
  of `umoci` which may be subject to change in future versions. In particular,
  this update of the specification now means that images may have ambiguous
  tags. `umoci` will warn you if an operation may have an ambiguous result, but
  we plan to improve this functionality far more in the future.
  openSUSE/umoci#133 openSUSE/umoci#142
- `umoci` also now supports more complicated descriptor walk structures, and
  also handles mutation of such structures more sanely. At the moment, this
  functionality has not been used "in the wild" and `umoci` doesn't have the UX
  to create such structures (yet) but these will be implemented in future
  versions. openSUSE/umoci#145
- `umoci repack` now supports `--mask-path` to ignore changes in the rootfs
  that are in a child of at least one of the provided masks when generating new
  layers. openSUSE/umoci#127

### Changed
- Error messages from `github.com/openSUSE/umoci/oci/cas/drivers/dir` actually
  make sense now. openSUSE/umoci#121
- `umoci unpack` now generates `config.json` blobs according to the [still
  proposed][ispec-pr492] OCI image specification conversion document.
  openSUSE/umoci#120
- `umoci repack` also now automatically adding `Config.Volumes` from the image
  configuration to the set of masked paths.  This matches recently added
  [recommendations by the spec][ispec-pr694], but is a backwards-incompatible
  change because the new default is that `Config.Volumes` **will** be masked.
  If you wish to retain the old semantics, use `--no-mask-volumes` (though make
  sure to be aware of the reasoning behind `Config.Volume` masking).
  openSUSE/umoci#127

[cii]: https://bestpractices.coreinfrastructure.org/projects/1084
[rspec-v1.0.0]: https://github.com/opencontainers/runtime-spec/releases/tag/v1.0.0
[ispec-v1.0.0]: https://github.com/opencontainers/image-spec/releases/tag/v1.0.0
[ispec-pr492]: https://github.com/opencontainers/image-spec/pull/492
[ispec-pr694]: https://github.com/opencontainers/image-spec/pull/694

## [0.2.1] - 2017-04-12
### Added
- `hack/release.sh` automates the process of generating all of the published
  artefacts for releases. The new script also generates signed source code
  archives. openSUSE/umoci#116

### Changed
- `umoci` now outputs configurations that are compliant with [`v1.0.0-rc5` of
  the OCI runtime-spec][rspec-v1.0.0-rc5]. This means that now you can use runc
  v1.0.0-rc3 with `umoci` (and rootless containers should work out of the box
  if you use a development build of runc). openSUSE/umoci#114
- `umoci unpack` no longer adds a dummy linux.seccomp entry, and instead just
  sets it to null. openSUSE/umoci#114

[rspec-v1.0.0-rc5]: https://github.com/opencontainers/runtime-spec/releases/tag/v1.0.0-rc5

## [0.2.0] - 2017-04-11
### Added
- `umoci` now has some automated scripts for generated RPMs that are used in
  openSUSE to automatically submit packages to OBS. openSUSE/umoci#101
- `--clear=config.{cmd,entrypoint}` is now supported. While this interface is a
  bit weird (`cmd` and `entrypoint` aren't treated atomically) this makes the
  UX more consistent while we come up with a better `cmd` and `entrypoint` UX.
  openSUSE/umoci#107
- New subcommand: `umoci raw runtime-config`. It generates the runtime-spec
  config.json for a particular image without also unpacking the root
  filesystem, allowing for users of `umoci` that are regularly parsing
  `config.json` without caring about the root filesystem to be more efficient.
  However, a downside of this approach is that some image-spec fields
  (`Config.User`) require a root filesystem in order to make sense, which is
  why this command is hidden under the `umoci-raw(1)` subcommand (to make sure
  only users that understand what they're doing use it). openSUSE/umoci#110

### Changed
- `umoci`'s `oci/cas` and `oci/config` libraries have been massively refactored
  and rewritten, to allow for third-parties to use the OCI libraries. The plan
  is for these to eventually become part of an OCI project. openSUSE/umoci#90
- The `oci/cas` interface has been modifed to switch from `*ispec.Descriptor`
  to `ispec.Descriptor`. This is a breaking, but fairly insignificant, change.
  openSUSE/umoci#89

### Fixed
- `umoci` now uses an updated version of `go-mtree`, which has a complete
  rewrite of `Vis` and `Unvis`. The rewrite ensures that unicode handling is
  handled in a far more consistent and sane way. openSUSE/umoci#88
- `umoci` used to set `process.user.additionalGids` to the "normal value" when
  unpacking an image in rootless mode, causing issues when trying to actually
  run said bundle with runC. openSUSE/umoci#109

## [0.1.0] - 2017-02-11
### Added
- `CHANGELOG.md` has now been added. openSUSE/umoci#76

### Changed
- `umoci` now supports `v1.0.0-rc4` images, which has made fairly minimal
  changes to the schema (mainly related to `mediaType`s). While this change
  **is** backwards compatible (several fields were removed from the schema, but
  the specification allows for "additional fields"), tools using older versions
  of the specification may fail to operate on newer OCI images. There was no UX
  change associated with this update.

### Fixed
- `umoci tag` would fail to clobber existing tags, which was in contrast to how
  the rest of the tag clobbering commands operated. This has been fixed and is
  now consistent with the other commands. openSUSE/umoci#78
- `umoci repack` now can correctly handle unicode-encoded filenames, allowing
  the creation of containers that have oddly named files. This required fixes
  to go-mtree (where the issue was). openSUSE/umoci#80

## [0.0.0] - 2017-02-07
### Added
- Unit tests are massively expanded, as well as the integration tests.
  openSUSE/umoci#68 openSUSE/umoci#69
- Full coverage profiles (unit+integration) are generated to get all
  information about how much code is tested. openSUSE/umoci#68
  openSUSE/umoci#69

### Fixed
- Static compilation now works properly. openSUSE/umoci#64
- 32-bit architecture builds are fixed. openSUSE/umoci#70

### Changed
- Unit tests can now be run inside `%check` of an `rpmbuild` script, allowing
  for proper testing. openSUSE/umoci#65.
- The logging output has been cleaned up to be much nicer for end-users to
  read. openSUSE/umoci#73
- Project has been moved to an openSUSE project. openSUSE/umoci#75

## [0.0.0-rc3] - 2016-12-19
### Added
- `unpack`, `repack`: `xattr` support which also handles `security.selinux.*`
  difficulties. openSUSE/umoci#49 openSUSE/umoci#52
- `config`, `unpack`: Ensure that environment variables are not duplicated in
  the extracted or stored configurations. openSUSE/umoci#30
- Add support for read-only CAS operations for read-only filesystems.
  openSUSE/umoci#47
- Add some helpful output about `--rootless` if `umoci` fails with `EPERM`.
- Enable stack traces with errors if the `--debug` flag was given to `umoci`.
  This requires a patch to `pkg/errors`.

### Changed
- `gc`: Garbage collection now also garbage collects temporary directories.
  openSUSE/umoci#17
- Clean-ups to vendoring of `go-mtree` so that it's much more
  upstream-friendly.

## [0.0.0-rc2] - 2016-12-12
### Added
- `unpack`, `repack`: Support for rootless unpacking and repacking.
  openSUSE/umoci#26
- `unpack`, `repack`: UID and GID mapping when unpacking and repacking.
  openSUSE/umoci#26
- `tag`, `rm`, `ls`: Tag modification commands such as `umoci tag`, `umoci rm`
  and `umoci ls`. openSUSE/umoci#6 openSUSE/umoci#27
- `stat`: Output information about an image. Currently only shows the history
  information. Only the **JSON** output is stable. openSUSE/umoci#38
- `init`, `new`: New commands have been created to allow for image creation
  from scratch. openSUSE/umoci#5 openSUSE/umoci#42
- `gc`: Garbage collection of images. openSUSE/umoci#6
- Full integration and unit testing, with OCI validation to ensure that we
  always create valid images. openSUSE/umoci#12

### Changed
- `unpack`, `repack`: Create history entries automatically (with options to
  modify the entries). openSUSE/umoci#36
- `unpack`: Store information about its source to ensure consistency when doing
  a `repack`. openSUSE/umoci#14
- The `--image` and `--from` arguments have been combined into a single
  `<path>[:<tag>]` argument for `--image`. openSUSE/umoci#39
- `unpack`: Configuration annotations are now extracted, though there are still
  some discussions happening upstream about the correct way of doing this.
  openSUSE/umoci#43

### Fixed
- `repack`: Errors encountered during generation of delta layers are now
  correctly propagated. openSUSE/umoci#33
- `unpack`: Hardlinks are now extracted as real hardlinks. openSUSE/umoci#25

### Security
- `unpack`, `repack`: Symlinks are now correctly resolved inside the unpacked
  rootfs. openSUSE/umoci#27

## 0.0.0-rc1 - 2016-11-10
### Added
- Proof of concept with major functionality implemented.
  + `unpack`
  + `repack`
  + `config`

[Unreleased]: https://github.com/openSUSE/umoci/compare/v0.2.1...HEAD
[0.2.1]: https://github.com/openSUSE/umoci/compare/v0.2.0...v0.2.1
[0.2.0]: https://github.com/openSUSE/umoci/compare/v0.1.0...v0.2.0
[0.1.0]: https://github.com/openSUSE/umoci/compare/v0.0.0...v0.1.0
[0.0.0]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc3...v0.0.0
[0.0.0-rc3]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc2...v0.0.0-rc3
[0.0.0-rc2]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc1...v0.0.0-rc2
