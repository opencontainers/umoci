# Change Log
All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## [Unreleased]
### Added
- `CHANGELOG.md` has now been added. openSUSE/umoci#76

### Fixed
- `umoci tag` would fail to clobber existing tags, which was in contrast to how
  the rest of the tag clobbering commands operated. This has been fixed and is
  now consistent with the other commands. openSUSE/umoci#78

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

[Unreleased]: https://github.com/openSUSE/umoci/compare/v0.0.0...HEAD
[0.0.0]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc3...v0.0.0
[0.0.0-rc3]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc2...v0.0.0-rc3
[0.0.0-rc2]: https://github.com/openSUSE/umoci/compare/v0.0.0-rc1...v0.0.0-rc2
