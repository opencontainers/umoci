// +build !linux,!darwin,!freebsd,!netbsd,!openbsd

package mtree

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
)

var (
	// this is bsd specific https://www.freebsd.org/cgi/man.cgi?query=chflags&sektion=2
	flagsKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		return emptyKV, nil
	}
	unameKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("uname=%s", hdr.Uname)), nil
		}
		return emptyKV, nil
	}
	uidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("uid=%d", hdr.Uid)), nil
		}
		return emptyKV, nil
	}
	gidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return KeyVal(fmt.Sprintf("gid=%d", hdr.Gid)), nil
		}
		return emptyKV, nil
	}
	nlinkKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		return emptyKV, nil
	}
	xattrKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		return emptyKV, nil
	}
)
