<!--
+++
# Hugo Front-matter
title = "Release Procedure"
aliases = ["/RELEASES.md"]
+++
-->

## Release Procedure ##

This document describes the procedure for preparing and publishing an umoci
release. Note that in order for a release of umoci to be approved, it must
follow the [approval procedure described in `GOVERNANCE.md`][governance].

[governance]: /GOVERNANCE.md

### Changelog ###

Before anything else, you must update the `CHANGELOG.md` to match the release.
This boils down to adding a new header underneath the "Unreleased" header so
that all of the changes are listed as being part of the new release.

```diff
 ## [Unreleased]
+
+## [X.Y.Z] - 20YY-MM-DD
 - Changed FooBar to correctly Baz.
```

And then the `[X.Y.Z]` reference needs to be added and the `[Unreleased]`
reference updated. These links just show the set of commits added between
successive versions:

```markdown
[Unreleased]: https://github.com/opencontainers/umoci/compare/vX.Y.Z...HEAD
[X.Y.Z]: https://github.com/opencontainers/umoci/compare/vX.Y.W...vX.Y.Z
[X.Y.W]: https://github.com/opencontainers/umoci/compare/vX.Y.V...vX.Y.W
```

The changes to `CHANGELOG.md` will be committed in the next section.

### Pull Request ###

Releases need to be associated with a specific commit, with the release commit
being voted on by the maintainers ([as described in `GOVERNANCE.md`][governance]).
To prepare the pull request, the following steps are necessary:

```ShellSession
% git branch release-X.Y.Z origin/master
% vi CHANGELOG.md # make the changes outlined above
% echo "X.Y.Z" > VERSION
% git commit -asm "VERSION: release X.Y.Z" # this is the release commit
% echo "X.Y.Z+dev" > VERSION
% git commit -asm "VERSION: back to development"
% git push # ...
```

And then [open a pull request][new-pr] with the `release-X.Y.Z` branch. If the
release requires a formal vote, the vote motion must specify the **release
commit** which is being considered (preferably both in the body and subject).

[governance]: /GOVERNANCE.md
[new-pr]: https://github.com/opencontainers/umoci/compare

### Release Notes ###

The release notes (as published on the [releases page][releases]) are largely
based on the contents of `CHANGELOG.md` for the relevant release. However, any
important information (such as deprecation warnings or security fixes) should
be included in the release notes above the changelog.

In addition, the release notes should contain a list of the contributors to the
release. The simplest way of doing this is to just use the author information
from Git:

```ShellSession
% git log --pretty=format:'%aN <%ae>' $1 | sort -u
```

Thus the final release notes should look something like the following:

```Markdown
**WARNING**: Feature FooBar is now considered deprecated and will be removed in
release X.Y.FOO.

 + Change FooBarBaz to support Xyz.
 * Update documentation of FooBar.
 - Remove broken XyzFoo implementation.

Thanks to all of the people who made this release possible:

 * Jane Citizen <jane.citizen@example.com>
 * Joe Citizen <joe.citizen@example.com>

Signed-off-by: [Your Name and Email Here]
```

[releases]: https://github.com/opencontainers/umoci/releases

### Tag ###

All release tags must be signed by a maintainer, and annotated with the release
notes which will be used on releases page. This can be done with `git tag -as`.
The set of valid keys to sign releases and artefacts is maintained [as part of
this repository in the `gpg-offline` format][umoci-keyring].

[umoci-keyring]: /umoci.keyring

### Artefacts ###

Once a release has been approved and tagged, the only thing left is to create
the set of release artefacts to attach to the release. This can be easily done
with `make release`, which will run `hack/release.sh` with the correct version
information and the release artefacts will be in `release/X.Y.Z`.

Note that you may need to explicitly specify the correct GPG key to use to sign
the artefacts, which can be done by specifying `GPG_KEYID=` during the `make
release` invocation.
