package mtree

// Unvis is a wrapper for the C implementation of unvis, which decodes a string
// that potentially has characters that are encoded with Vis
func Unvis(src string) (string, error) {
	return unvis(src)
}
