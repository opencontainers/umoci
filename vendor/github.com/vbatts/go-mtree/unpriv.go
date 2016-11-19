package mtree

import (
	"io"
	"os"

	// NOTE: This is a hack done only for the umoci codebase. When this is
	//       being handled upstream, we'll have to vendor this or otherwise
	//       split it from umoci.
	"github.com/cyphar/umoci/pkg/unpriv"
)

type operator struct {
	Rootless bool
}

// open is a wrapper around unpriv.Open and os.Open, and will call the right
// wrapper depending on whether o.Rootless is set.
func (o operator) open(path string) (*os.File, error) {
	if o.Rootless {
		return unpriv.Open(path)
	}
	return os.Open(path)
}

// readlink is a wrapper around unpriv.Readlink and os.Readlink, and will call
// the right wrapper depending on whether o.Rootless is set.
func (o operator) readlink(path string) (string, error) {
	if o.Rootless {
		return unpriv.Readlink(path)
	}
	return os.Readlink(path)
}

// lstat is a wrapper around unpriv.Lstat and os.Lstat, and will call the right
// wrapper depending on whether o.Rootless is set.
func (o operator) lstat(path string) (os.FileInfo, error) {
	if o.Rootless {
		return unpriv.Lstat(path)
	}
	return os.Lstat(path)
}

// readdir is a wrapper around unpriv.Readdir and os.Open.Readdir, and will
// call the right wrapper depending on whether o.Rootless is set.
func (o operator) readdir(path string) ([]os.FileInfo, error) {
	if o.Rootless {
		return unpriv.Readdir(path)
	}

	// There's no wrapper for this in os.*, so we have to do it ourselves.
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	return fh.Readdir(-1)
}

// keywordFunc will conditionally wrap a KeywordFunc with unpriv.Wrap, if
// o.Rootless is set. Otherwise it returns the provided keywordFunc.
func (o operator) keywordFunc(fn KeywordFunc) KeywordFunc {
	if !o.Rootless {
		return fn
	}

	return func(path string, info os.FileInfo, r io.Reader) (KeyVal, error) {
		var kv KeyVal
		err := unpriv.Wrap(path, func(path string) error {
			var err error
			kv, err = fn(path, info, r)
			return err
		})
		return kv, err
	}
}
