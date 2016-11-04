// +build !linux

package xattr

func Get(path, name string) ([]byte, error) {
	return nil, nil
}
func Set(path, name string, value []byte) error {
	return nil
}
func List(path string) ([]string, error) {
	return nil, nil
}
