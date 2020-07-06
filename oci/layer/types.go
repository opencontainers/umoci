package layer

import (
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

type WhiteoutMode int

const (
	// OCIStandardWhiteout does the standard OCI thing: a file named
	// .wh.foo indicates you should rm -rf foo.
	OCIStandardWhiteout WhiteoutMode = iota

	// OverlayFSWhiteout generates a rootfs suitable for use in overlayfs,
	// so it follows the overlayfs whiteout protocol:
	//     .wh.foo => mknod c 0 0 foo
	OverlayFSWhiteout
)

type UnpackOptions struct {
	// MapOptions are the UID and GID mappings used when unpacking an image
	MapOptions MapOptions

	// KeepDirlinks is essentially the same as rsync's optio
	// --keep-dirlinks: if, on extraction, a directory would be created
	// where a symlink to a directory previously existed, KeepDirlinks
	// doesn't create that directory, but instead just uses the existing
	// symlink.
	KeepDirlinks bool

	// AfterLayerUnpack is a function that's called after every layer is
	// unpacked.
	AfterLayerUnpack AfterLayerUnpackCallback

	// StartFrom is the descriptor in the manifest to start from
	StartFrom ispec.Descriptor

	// WhiteoutMode is the type of whiteout to write to the filesystem.
	WhiteoutMode WhiteoutMode
}
