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
	flagsKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		// ideally this will pull in from here https://www.freebsd.org/cgi/man.cgi?query=chflags&sektion=2
		return emptyKV, nil
	}

	unameKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("uname=%s", hdr.Uname)), nil
		}

		stat := info.Sys().(*syscall.Stat_t)
		u, err := user.LookupId(fmt.Sprintf("%d", stat.Uid))
		if err != nil {
			return emptyKV, err
		}
		return KeyVal(fmt.Sprintf("uname=%s", u.Username)), nil
	}
	uidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("uid=%d", hdr.Uid)), nil
		}
		stat := info.Sys().(*syscall.Stat_t)
		return KeyVal(fmt.Sprintf("uid=%d", stat.Uid)), nil
	}
	gidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("gid=%d", hdr.Gid)), nil
		}
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			return KeyVal(fmt.Sprintf("gid=%d", stat.Gid)), nil
		}
		return emptyKV, nil
	}
	nlinkKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if stat, ok := info.Sys().(*syscall.Stat_t); ok {
			return KeyVal(fmt.Sprintf("nlink=%d", stat.Nlink)), nil
		}
		return emptyKV, nil
	}
	xattrKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		return emptyKV, nil
	}
)
