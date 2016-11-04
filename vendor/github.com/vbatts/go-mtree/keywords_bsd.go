// +build darwin freebsd netbsd openbsd

package mtree

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"os/user"
	"syscall"
)

var (
	flagsKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		// ideally this will pull in from here https://www.freebsd.org/cgi/man.cgi?query=chflags&sektion=2
		return "", nil
	}

	unameKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("uname=%s", hdr.Uname), nil
		}

		stat := info.Sys().(*syscall.Stat_t)
		u, err := user.LookupId(fmt.Sprintf("%d", stat.Uid))
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("uname=%s", u.Username), nil
	}
	uidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("uid=%d", hdr.Uid), nil
		}
		stat := info.Sys().(*syscall.Stat_t)
		return fmt.Sprintf("uid=%d", stat.Uid), nil
	}
	gidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("gid=%d", hdr.Gid), nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			return fmt.Sprintf("gid=%d", stat.Gid), nil
		}
		return "", nil
	}
	nlinkKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			return fmt.Sprintf("nlink=%d", stat.Nlink), nil
		}
		return "", nil
	}
	xattrKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		return "", nil
	}
)
