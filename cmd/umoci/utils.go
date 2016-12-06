package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"

	"github.com/cyphar/umoci/image/layer"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// FIXME: This should be moved to a library. Too much of this code is in the
//        cmd/... code, but should really be refactored to the point where it
//        can be useful to other people. This is _particularly_ true for the
//        code which repacks images (the changes to the config, manifest and
//        CAS should be made into a library).

// UmociMetaName is the name of umoci's metadata file that is stored in all
// bundles extracted by umoci.
const UmociMetaName = "umoci.json"

// UmociMeta represents metadata about how umoci unpacked an image to a bundle
// and other similar information. It is used to keep track of information that
// is required when repacking an image and other similar bundle information.
type UmociMeta struct {
	// Version is the version of umoci used to unpack the bundle. This is used
	// to future-proof the umoci.json information.
	Version string `json:"umoci_version"`

	// From is a copy of the descriptor pointing to the image manifest that was
	// used to unpack the bundle. Essentially it's a resolved form of the
	// --from argument to umoci-unpack(1).
	From ispec.Descriptor `json:"from_descriptor"`

	// MapOptions is the parsed version of --uid-map, --gid-map and --rootless
	// arguments to umoci-unpack(1). While all of these options technically do
	// not need to be the same for corresponding umoci-unpack(1) and
	// umoci-repack(1) calls, changing them is not recommended and so the
	// default should be that they are the same.
	MapOptions layer.MapOptions `json:"map_options"`
}

// WriteTo writes a JSON-serialised version of UmociMeta to the given io.Writer.
func (m UmociMeta) WriteTo(w io.Writer) (int64, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(io.MultiWriter(buf, w)).Encode(m)
	return int64(buf.Len()), err
}

// WriteBundleMeta writes an umoci.json file to the given bundle path.
func WriteBundleMeta(bundle string, meta UmociMeta) error {
	fh, err := os.Create(filepath.Join(bundle, UmociMetaName))
	if err != nil {
		return err
	}
	defer fh.Close()

	_, err = meta.WriteTo(fh)
	return err
}

// ReadBundleMeta reads and parses the umoci.json file from a given bundle path.
func ReadBundleMeta(bundle string) (UmociMeta, error) {
	var meta UmociMeta

	fh, err := os.Open(filepath.Join(bundle, UmociMetaName))
	if err != nil {
		return meta, err
	}
	defer fh.Close()

	err = json.NewDecoder(fh).Decode(&meta)
	return meta, err
}
