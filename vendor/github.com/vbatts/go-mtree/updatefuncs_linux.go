// +build linux

package mtree

import (
	"encoding/base64"
	"os"
	"syscall"
	"time"
	"unsafe"

	"github.com/vbatts/go-mtree/xattr"
)

func xattrUpdateKeywordFunc(path string, kv KeyVal) (os.FileInfo, error) {
	buf, err := base64.StdEncoding.DecodeString(kv.Value())
	if err != nil {
		return nil, err
	}
	if err := xattr.Set(path, kv.Keyword().Suffix(), buf); err != nil {
		return nil, err
	}
	return os.Lstat(path)
}

func lchtimes(name string, atime time.Time, mtime time.Time) error {
	var utimes [2]syscall.Timespec
	utimes[0] = syscall.NsecToTimespec(atime.UnixNano())
	utimes[1] = syscall.NsecToTimespec(mtime.UnixNano())
	if e := utimensat(atFdCwd, name, (*[2]syscall.Timespec)(unsafe.Pointer(&utimes[0])), atSymlinkNofollow); e != nil {
		return &os.PathError{Op: "chtimes", Path: name, Err: e}
	}
	return nil

}

// from uapi/linux/fcntl.h
// don't follow symlinks
const atSymlinkNofollow = 0x100

// special value for utimes as the FD for the current working directory
const atFdCwd = -0x64

func utimensat(dirfd int, path string, times *[2]syscall.Timespec, flags int) (err error) {
	if len(times) != 2 {
		return syscall.EINVAL
	}
	var _p0 *byte
	_p0, err = syscall.BytePtrFromString(path)
	if err != nil {
		return
	}
	_, _, e1 := syscall.Syscall6(syscall.SYS_UTIMENSAT, uintptr(dirfd), uintptr(unsafe.Pointer(_p0)), uintptr(unsafe.Pointer(times)), uintptr(flags), 0, 0)
	if e1 != 0 {
		err = syscall.Errno(e1)
	}
	return
}
