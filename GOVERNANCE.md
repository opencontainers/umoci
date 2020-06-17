<!--
+++
# Hugo Front-matter
title = "Governance"
aliases = ["/GOVERNANCE.md"]
+++
-->

## Governance Model ##

umoci is an OCI project, and is thus governed under the [OCI
Charter][oci-charter]. &sect;5(b)(viii) of the Charter tasks the maintainers of
each OCI Project with:

> Creating, maintaining and enforcing governance guidelines for the TDC [of
> that OCI Project], approved by the maintainers, and which shall be posted
> visibly for the TDC [of that OCI Project].

This document describes the governance rules for umoci, and is the
authortiative document that describes the governance procedure for umoci. Any
change to this document requires a motion and vote (as described below) in
order to be a valid modification of this governance document.

If there are any perceived or real conflicts between this document and the OCI
Charter, the OCI Charter takes precedence.

[oci-charter]: https://github.com/opencontainers/tob/blob/master/CHARTER.md

### Code of Conduct ###

The conduct of all members of the umoci TDC MUST abide by the Open Containers
Initative's [Code of Conduct][oci-coc].

[oci-coc]: https://github.com/opencontainers/.github/blob/master/CODE_OF_CONDUCT.md

### Code Changes ###

Any code or documentation changes to the umoci codebase will be made via [pull
requests][umoci-pr]. All proposed changes MUST contain a "Signed-off-by" line,
indicating that the contribution abides by the [Developer Certificate of
Origin][dco].

In order for a change to be accepted into the umoci codebase, it MUST be
reviewed and accepted by two (2) maintainers (individuals who are listed in the
`MAINTAINERS` file at the root of the umoci repository). Maintainers indicate
their approval through the use of an "LGTM" comment. Maintainers MAY formally
reject a comment by posting "NACK", but this is only an indication to other
maintainers of their view and has no impact on the two-LGTM rule. If a
contribution was authored by a maintainer, they MAY approve their own change
(meaning that only one (1) additional maintainer need review and approve it).
Merge commits SHOULD include information about which maintainers approved the
change.

The above review procedure MAY be done in private for patches fixing security
issues, but the set of maintainers which approved the patches MUST be publicly
known after the patch has been merged into the repository (such as listing them
in the merge commit). All private reviews MUST be done via email, with the
<security@opencontainers.org> mailing list in Cc to ensure that this system is
not being abused.

Recommendations for contributors may be found in
[`CONTRIBUTING.md`][contributing].

[umoci-pr]: https://github.com/opencontainers/umoci/compare
[dco]: https://developercertificate.org/
[contributing]: /CONTRIBUTING.md

### Releases ###

umoci uses [Semantic Versioning][semver], and all releases MUST follow the
SemVer definition of major, minor, and patch releases. The general procedure
for preparing a new release is described in [`RELEASES.md`][releases], but all
releases MUST have a specific commit which will be tagged as the release.

A pull request is opened by the individual proposing a release, and then voting
is conducted on the release. There are two procedures depending on which part
of the version number is being proposed to change:

 * If the major number is being changed from the previous release, a formal
   vote as described below is required. Due to their disruptive nature, major
   updates SHOULD be done only in exceptional circumstances after umoci `1.0.0`
   has been released, and SHOULD be done after significant discussion and
   agremeent between all umoci maintainers and the umoci TDC.

 * If the minor or patch number (but not the major number) are being changed,
   then the release MAY be approved by the simplified two-LGTM requirement as
   required by ordinary code changes.

All releases SHOULD be announced on the <dev@opencontainers.org> mailing list,
and the release artefacts SHOULD be made available as soon as possible.

[semver]: https://semver.org/
[releases]: /RELEASES.md

### Other Changes ###

All other changes to the umoci project (such as the modification of this
document or the addition or removal of maintainers) require a formal voting
procedure. Any individual MAY propose a motion (either by creating an issue in
the project's issue tracker, or posting a message on the
<dev@opencontainers.org> mailing list) but only maintainers' votes on the
motion are considered. Any motion which involves merging a change into the
codebase (such as motions to change this document, or change the major version
number of umoci) MUST include the specific commit which is being proposed for
merging.

All votes MUST have a reasonable deadline (two (2) weeks is most common) listed
when the motion is first posted, and the deadline is chosen by the initiator of
the motion.

If the motion is posted to the mailing list, the subject line SHOULD be:

```
[umoci VOTE] {motion description} (closes {deadline})
```

But if the motion is posted as an issue, then the subject SHOULD begin with the
prefix `[VOTE]`.

In order for a motion to pass, a qualified super-majority (at least two-thirds,
with abstain and non-votes counting as votes *against* the motion) of the
project's maintainers MUST vote in favour of the motion. Votes MUST be
indicated as follows:

  * Votes in favour with "LGTM" or "+1".
  * Votes against with "NACK", "REJECT", or "-1".
  * Abstain votes with "ABSTAIN".

Maintainers MAY vote multiple times, with their final vote (at the close of the
voting period) being treated as their decision.

Once a vote has completed, the vote totals and a brief description of the
motion MUST be posted on the <dev@opencontainers.org> mailing list. The subject
line of the vote tally SHOULD be:

```
[umoci {ACCEPTED|REJECTED}] {motion description} (+{LGTMs} -{REJECTs} #{ABSTAINs})
```

[oci-ml]: mailto:dev@opencontainers.org
