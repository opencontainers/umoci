package mtree

// #include "vis.h"
// #include <stdlib.h>
import "C"
import (
	"fmt"
	"math"
	"unsafe"
)

// Vis is a wrapper of the C implementation of the function vis, which encodes
// a character with a particular format/style
func Vis(src string) (string, error) {
	// dst needs to be 4 times the length of str, must check appropriate size
	if uint32(len(src)*4+1) >= math.MaxUint32/4 {
		return "", fmt.Errorf("failed to encode: %q", src)
	}
	dst := string(make([]byte, 4*len(src)+1))
	cDst, cSrc := C.CString(dst), C.CString(src)
	defer C.free(unsafe.Pointer(cDst))
	defer C.free(unsafe.Pointer(cSrc))
	C.strvis(cDst, cSrc, C.VIS_WHITE|C.VIS_OCTAL|C.VIS_GLOB)

	return C.GoString(cDst), nil
}
