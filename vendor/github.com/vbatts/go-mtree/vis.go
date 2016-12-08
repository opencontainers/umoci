package mtree

import "unicode"

// Vis is a wrapper of the C implementation of the function vis, which encodes
// a character with a particular format/style.
// For most use-cases use DefaultVisFlags.
func Vis(src string, flags VisFlag) (string, error) {
	return vis(src, flags)
}

// DefaultVisFlags are the typical flags used for encoding strings in mtree
// manifests.
var DefaultVisFlags = VisWhite | VisOctal | VisGlob

// VisFlag sets the extent of charactures to be encoded
type VisFlag int

// flags for encoding
const (
	// to select alternate encoding format
	VisOctal  VisFlag = 0x01 // use octal \ddd format
	VisCstyle VisFlag = 0x02 // use \[nrft0..] where appropriate

	// to alter set of characters encoded (default is to encode all non-graphic
	// except space, tab, and newline).
	VisSp    VisFlag = 0x04 // also encode space
	VisTab   VisFlag = 0x08 // also encode tab
	VisNl    VisFlag = 0x10 // also encode newline
	VisWhite VisFlag = (VisSp | VisTab | VisNl)
	VisSafe  VisFlag = 0x20 // only encode "unsafe" characters

	// other
	VisNoSlash   VisFlag = 0x40  // inhibit printing '\'
	VisHttpstyle VisFlag = 0x80  // http-style escape % HEX HEX
	VisGlob      VisFlag = 0x100 // encode glob(3) magics

)

// errors used in the tokenized decoding strings
const (
	// unvis return codes
	unvisValid            unvisErr = 1  // character valid
	unvisValidPush        unvisErr = 2  // character valid, push back passed char
	unvisNochar           unvisErr = 3  // valid sequence, no character produced
	unvisErrSynbad        unvisErr = -1 // unrecognized escape sequence
	unvisErrUnrecoverable unvisErr = -2 // decoder in unknown state (unrecoverable)
	unvisNone             unvisErr = 0

	// unvisEnd means there are no more characters
	unvisEnd VisFlag = 1 // no more characters
)

// unvisErr are the return conditions for Unvis
type unvisErr int

func (ue unvisErr) Error() string {
	switch ue {
	case unvisValid:
		return "character valid"
	case unvisValidPush:
		return "character valid, push back passed char"
	case unvisNochar:
		return "valid sequence, no character produced"
	case unvisErrSynbad:
		return "unrecognized escape sequence"
	case unvisErrUnrecoverable:
		return "decoder in unknown state (unrecoverable)"
	}
	return "Unknown Error"
}

func ishex(r rune) bool {
	lr := unicode.ToLower(r)
	return (lr >= '0' && lr <= '9') || (lr >= 'a' && lr <= 'f')
}

func isoctal(r rune) bool {
	return r <= '7' && r >= '0'
}

// the ctype isgraph is "any printable character except space"
func isgraph(r rune) bool {
	return unicode.IsPrint(r) && !unicode.IsSpace(r)
}

func isalnum(r rune) bool {
	return unicode.IsNumber(r) || unicode.IsLetter(r)
}
