## `third_party/symlink` ##

This came from `github.com/docker/docker/pkg/symlink`, and has some
modifications to remove windows support and also remove the usage of Docker's
`system` pkg.

Package symlink implements EvalSymlinksInScope which is an extension of
filepath.EvalSymlinks, as well as a Windows long-path aware version of
filepath.EvalSymlinks from the [Go standard library][filepath].

The code from filepath.EvalSymlinks has been adapted in `fs.go`. Please read
the `LICENSE.BSD` file that governs `fs{,_unix}.go` and `LICENSE.APACHE` for
`fs_unix_test.go`.

[filepath]: https://golang.org/pkg/path/filepath
