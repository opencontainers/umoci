+++
title = "Security Considerations"
weight = 25
+++

umoci is entrusted with taking a particular image and expanding it on the
filesystem (as well as computing deltas of the expanded filesystem). As a
result, umoci must be very careful about filesystem access to avoid
vulnerabilities that may impact other components of the system. If you feel
something is missing from this document, feel free to
[contribute][contributing.md].

If you've found a security flaw in umoci that comes from a security
consideration we haven't considered before, please follow our instructions on
[how to responsibly disclose a security issue][contributing.md]. Once it has
been resolved, feel free to contribute to this document a description of the
consideration that we were missing.

[contributing.md]: /contributing

### Path Traversal ###

The most obvious vulnerabilities that umoci has to protect against is path
traversal vulnerabilities. However, path traversal vulnerabilities in umoci
are even more dangerous because umoci has to emulate a `chroot` when
interacting with the filesystem (as well as never mutating inodes in case it
may be referenced by a path external to the root filesystem).

> **NOTE**: Users are expected to appreciate the risks of interacting with a
> foreign filesystem rootfs, if they intend to not use a virtualization system
> such as `chroot` or containers when interacting with the rootfs umoci has
> expanded.

There are multiple ways that umoci defends against this. The first and most
important is that all path computation of destination and source paths is done
using the [`filepath-securejoin`][securejoin] library (which was written by the
author of umoci<sup>[1](#foot1)</sup>). `filepath-securejoin` solves this
problem by effectively implementing a variant of `filepath.EvalSymlinks` except
that all symlink and lexical evaluation is done manually within a particular
scope. We have extensive testing (in both projects) to show that a variety of
attacks will not succeed against our implementation.

In addition, umoci has a series of sanity checks that ensures that operations
that involve modifying the filesystem based on arbitrary image input will not
affect anything outside of the intended directory root. These additions are
purely defensive in nature, as `filepath-securejoin` handles all known
instances of path traversal attacks in this context.

umoci **does not** attempt to be safe against shared filesystem access during
an `unpack` or `repack` operation. This is because it is not possible with any
modern Unix<sup>[2](#foot2)</sup> system to safely ensure that a particular
user-space path evaluation will be atomic (nor that the returned result cannot
later become unsafe given further filesystem manipulation). Once an `unpack` or
`repack` operation has completed, arbitrary filesystem modification of the root
filesystem will not cause umoci to act unsafely in future operations.

umoci also defends against inode mutation by always replacing inodes. This
actually makes the code simpler, but makes sure that hardlinks to inodes inside
a root filesystem won't see any modifications because of umoci (not that
umoci currently permits unpacking into existing directories).

<a name="foot1">1</a>: The implementation linked is loosely based on Docker's
implementation of a similar concept (to protect against similar path
traversals). Interestingly, the security vulnerabilities that alerted Docker to
this issue several years ago were discovered and fixed by this author as well.

<a name="foot2">2</a>: Interestingly, Plan9 has effectively solved this problem
by removing symlinks and instead using namespaces. However, umoci's main user
demographic is not Plan9 so this tid-bit is not of much use to us.

[securejoin]: https://github.com/cyphar/filepath-securejoin

### Arbitrary Inode and Mode Creation ###

Note that this attack is only applicable if umoci is being executed as a
**privileged user** (on GNU/Linux this means having `CAP_SYS_ADMIN` and a host
of other capabilities). umoci running in rootless mode cannot create
arbitrary inodes due to ordinary access control implemented by the operating
system (umoci only uses standard VFS operations to implement rootless mode,
which have very well-understood access control restrictions).

Effectively this attack boils down to the fact that a `tar` archive (which is
what makes up the layers in OCI images) contains inode information that can be
used to cause umoci to create block devices with arbitrary major and minor
numbers. In the absolute worst case, this could allow a user to create a
world-writeable inode that corresponds to the host's hard-drive (or
`/dev/kmem`). There are a variety of other possible attacks that can occur.
Note that the default umoci runtime configuration defends against containers
from being able to mess with such files, but this doesn't help against
host-side attackers. This attack also could be used to provide an unprivileged
user (in the host) access unsafe set-uid binaries, allowing for possible
privilege escalation.

umoci's defence against this attack is to make the `bundle` directory `chmod
go-rwx` which will ensure that unprivileged users won't be able to resolve the
dangerous inode or setuid binary (note that bind-mounts can circumvent this,
but a user cannot create a bind-mount without being able to resolve the path).

### Compression Bomb Attacks ###

OCI images specifically require implementations to support layers compressed
with `gzip`. This results in [gzip bomb attacks][gzip-bomb] being potentially
practical. In the worst cases this can result in denial-of-service attacks, if
umoci is run in a context without any storage or I/O quotas.

umoci uses the [Go standard library implementation of `gzip`][go-gzip], which
*does not appear* to support multiple-round decompression. This somewhat
reduces the amplification nature of this attack, but the attack is still
present.

**umoci currently has no specific defences against this attack.** There is a
very trivial solution that we can add, which is a `--maximum-layer-size` flag
that will abort an `unpack` operation if any single layer is deemed too large.
Unfortunately, it does not seem reasonable to enable such an option for all
`unpack` operations, and picking a reasonable default is an even more serious
concern. There does not appear to be much analysis on heuristics that can be
used to detect compression bombs, but I am under the impression that that would
be the only reasonable way to detect this in the general case (a heuristic
based on the Shannon entropy or the compression ratio would be very
interesting).

A workaround for this problem is to place umoci inside of the relevant
cgroups so that it will not drain the system resources too heavily if
encountering a compression bomb. As mentioned above, detecting generic
compression bombs does not appear to be a solved problem, so this workaround
only helps to reduce the impact and does not mitigate it.

[gzip-bomb]: https://www.rapid7.com/db/modules/auxiliary/dos/http/gzip_bomb_dos
[go-gzip]: https://golang.org/pkg/compress/gzip/
