// +build !linux

package mtree

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
)

var (
	unameKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("uname=%s", hdr.Uname), nil
		}
		return "", nil
	}
	uidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("uid=%d", hdr.Uid), nil
		}
		return "", nil
	}
	gidKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		if hdr, ok := info.Sys().(*tar.Header); ok {
			return fmt.Sprintf("gid=%d", hdr.Gid), nil
		}
		return "", nil
	}
	nlinkKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		return "", nil
	}
	xattrKeywordFunc = func(path string, info os.FileInfo, r io.Reader) (string, error) {
		return "", nil
	}
)
