// +build !linux

package mtree

import (
	"os"
	"time"
)

func xattrUpdateKeywordFunc(path string, kv KeyVal) (os.FileInfo, error) {
	return os.Lstat(path)
}

func lchtimes(name string, atime time.Time, mtime time.Time) error {
	return nil
}
