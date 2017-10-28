+++
title = "Rootless Containers"
weight = 50
+++

umoci has first class support for [rootless containers][rootlesscontaine.rs],
and in particular it supports rootless unpacking. This means that an
unprivileged user can unpack and repack and image (which is not traditionally
possible for most images), as well as generate a runtime configuration that can
be used by runc to start a rootless container.

{{% notice info %}}
It should noted that the root filesystem created as an unprivileged user will
likely not match the root filesystem that a privileged user would create. The
reason for this is that there are a set of security restrictions imposed by the
operating system that stop us from creating certain device inodes and set-uid
binaries. umoci will do its best to try to emulate the correct behaviour, and
the runtime configuration generated will further try to emulate the correct
behaviour.
{{% /notice %}}

```text
% id -u
1000
% umoci unpack --rootless --image opensuse:42.2 bundle
   • rootless{usr/bin/ping} ignoring (usually) harmless EPERM on setxattr "security.capability"
   • rootless{usr/bin/ping6} ignoring (usually) harmless EPERM on setxattr "security.capability"
% runc run -b bundle rootless-ctr
bash-4.3# whoami
root
bash-4.3# tee /hostname </proc/sys/kernel/hostname
mrsdalloway
% umoci repack --image opensuse:new bundle
```

{{% notice tip %}}
The above warnings can be safely ignored, they are caused by umoci not having
sufficient privileges in this context. They are output purely to ensure that
users are aware that the root filesystem they get might not be precisely the
same as the one they'd get if they extracted it as a privileged user.
{{% /notice %}}

[rootlesscontaine.rs]: https://rootlesscontaine.rs/
