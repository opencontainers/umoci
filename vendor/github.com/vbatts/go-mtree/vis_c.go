// +build cgo,cvis

package mtree

import (
	"github.com/vbatts/go-mtree/cvis"
)

func vis(src string, flags VisFlag) (string, error) {
	return cvis.Vis(src, int(flags))
}
