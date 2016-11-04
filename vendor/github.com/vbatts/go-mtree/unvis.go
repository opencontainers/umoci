package mtree

// #include "vis.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"unsafe"
)

// Unvis is a wrapper for the C implementation of unvis, which decodes a string
// that potentially has characters that are encoded with Vis
func Unvis(src string) (string, error) {
	cDst, cSrc := C.CString(string(make([]byte, len(src)+1))), C.CString(src)
	defer C.free(unsafe.Pointer(cDst))
	defer C.free(unsafe.Pointer(cSrc))
	ret := C.strunvis(cDst, cSrc)
	if ret == -1 {
		return "", fmt.Errorf("failed to decode: %q", src)
	}
	return C.GoString(cDst), nil
}
