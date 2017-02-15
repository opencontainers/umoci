// +build linux

package mtree

import (
	"archive/tar"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/user"
	"strings"
	"syscall"

	"github.com/vbatts/go-mtree/pkg/govis"
	"github.com/vbatts/go-mtree/xattr"
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
		if hdr, ok := info.Sys().(*tar.Header); ok {
			if len(hdr.Xattrs) == 0 {
				return emptyKV, nil
			}
			klist := []KeyVal{}
			for k, v := range hdr.Xattrs {
				encKey, err := govis.Vis(k, DefaultVisFlags)
				if err != nil {
					return emptyKV, err
				}
				klist = append(klist, KeyVal(fmt.Sprintf("xattr.%s=%s", encKey, base64.StdEncoding.EncodeToString([]byte(v)))))
			}
			return KeyVal(strings.Join(KeyValToString(klist), " ")), nil
		}
		if !info.Mode().IsRegular() && !info.Mode().IsDir() {
			return emptyKV, nil
		}

		xlist, err := xattr.List(path)
		if err != nil {
			return emptyKV, err
		}
		klist := make([]KeyVal, len(xlist))
		for i := range xlist {
			data, err := xattr.Get(path, xlist[i])
			if err != nil {
				return emptyKV, err
			}
			encKey, err := govis.Vis(xlist[i], DefaultVisFlags)
			if err != nil {
				return emptyKV, err
			}
			klist[i] = KeyVal(fmt.Sprintf("xattr.%s=%s", encKey, base64.StdEncoding.EncodeToString(data)))
		}
		return KeyVal(strings.Join(KeyValToString(klist), " ")), nil
	}
)
