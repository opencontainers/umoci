// +build cgo,cvis

package mtree

import (
	"github.com/vbatts/go-mtree/cvis"
)

func unvis(src string) (string, error) {
	return cvis.Unvis(src)
}
