+++
title = "Rootless Containers"
weight = 50
+++

umoci has first class support for [rootless containers][rootlesscontaine.rs],
and in particular it supports rootless unpacking. This means that an
unprivileged user can unpack and repack and image (which is not traditionally
possible for most images), as well as generate a runtime configuration that can
be used by runc to start a rootless container.

```text
% id -u
1000
% umoci unpack --rootless --image opensuse:42.2 bundle
   • restoreMetadata: ignoring EPERM on setxattr: security.capability: unpriv.lsetxattr: operation not permitted
   • restoreMetadata: ignoring EPERM on setxattr: security.capability: unpriv.lsetxattr: operation not permitted
% runc run -b bundle rootless-ctr
bash-4.3# whoami
root
bash-4.3# tee /hostname </proc/sys/kernel/hostname
mrsdalloway
% umoci repack --image opensuse:new bundle
```

[rootlesscontaine.rs]: https://rootlesscontaine.rs/
