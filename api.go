package umoci

import (
	"fmt"
	"io"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// Layout represents an OCI image layout.
type Layout struct {
	ext casext.Engine
}

// OpenLayout opens an existing OCI image layout, and fails if it does not
// exist.
func OpenLayout(imagePath string) (*Layout, error) {
	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return nil, errors.Wrap(err, "open CAS")
	}

	l := &Layout{}
	l.ext = casext.NewEngine(engine)

	return l, nil
}

// CreateLayout creates an existing OCI image layout, and fails if it already
// exists.
func CreateLayout(imagePath string) (*Layout, error) {
	err := dir.Create(imagePath)
	if err != nil {
		return nil, err
	}

	return OpenLayout(imagePath)
}

func (l *Layout) resolve(fromName string) (ispec.Descriptor, error) {
	descriptorPaths, err := l.ext.ResolveReference(context.Background(), fromName)
	if err != nil {
		return ispec.Descriptor{}, err
	}

	if err != nil {
		return ispec.Descriptor{}, errors.Wrap(err, "get descriptor")
	}
	if len(descriptorPaths) == 0 {
		return ispec.Descriptor{}, errors.Errorf("tag not found: %s", fromName)
	}
	if len(descriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return ispec.Descriptor{}, errors.Errorf("tag is ambiguous: %s", fromName)
	}
	descriptor := descriptorPaths[0].Descriptor()
	return descriptor, nil
}

// Tag adds a tag based on a previously existing descriptor.
func (l *Layout) Tag(from string, to string) error {
	descriptor, err := l.resolve(from)
	if err != nil {
		return err
	}

	if err := l.ext.UpdateReference(context.Background(), to, descriptor); err != nil {
		return errors.Wrap(err, "put reference")
	}

	return nil
}

// PutBlob adds the content of the reader to the OCI image as a blob, and
// returns a Blob describing the result.
func (l *Layout) PutBlob(b io.Reader) (digest.Digest, int64, error) {
	return l.ext.PutBlob(context.Background(), b)
}

// NewImage creates a new OCI manifest in the OCI image, and adds the specified
// layers to it.
func (l *Layout) NewImage(tagName string, i *ispec.Image, layers []ispec.Descriptor) error {
	// Update config and create a new blob for it.
	configDigest, configSize, err := l.ext.PutBlobJSON(context.Background(), i)
	if err != nil {
		return errors.Wrap(err, "put config blob")
	}

	// Create a new manifest that just points to the config and has an
	// empty layer set. FIXME: Implement ManifestList support.
	manifest := ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2, // FIXME: This is hardcoded at the moment.
		},
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: layers,
	}

	manifestDigest, manifestSize, err := l.ext.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		return errors.Wrap(err, "put manifest blob")
	}

	descriptor := ispec.Descriptor{
		// FIXME: Support manifest lists.
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}

	if err := l.ext.UpdateReference(context.Background(), tagName, descriptor); err != nil {
		return errors.Wrap(err, "add new tag")
	}

	return nil
}

// ListTags lists the tags in the OCI image.
func (l *Layout) ListTags() ([]string, error) {
	return l.ext.ListReferences(context.Background())
}

// Close closes the OCI image.
func (l *Layout) Close() error {
	return l.ext.Close()
}

func (l *Layout) LookupManifest(tag string) (ispec.Manifest, error) {
	tagDescriptor, err := l.resolve(tag)
	if err != nil {
		return ispec.Manifest{}, err
	}

	manifestBlob, err := l.ext.FromDescriptor(context.Background(), tagDescriptor)
	if err != nil {
		return ispec.Manifest{}, errors.Wrap(err, "get manifest")
	}
	defer manifestBlob.Close()

	if manifestBlob.MediaType != ispec.MediaTypeImageManifest {
		return ispec.Manifest{}, errors.Wrap(fmt.Errorf("descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestBlob.MediaType), "invalid --image tag")
	}

	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return ispec.Manifest{}, errors.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.MediaType)
	}

	return manifest, nil
}

// LayersForTag returns the layer blobs that the particular tag references.
func (l *Layout) LayersForTag(tag string) ([]*casext.Blob, error) {
	manifest, err := l.LookupManifest(tag)
	if err != nil {
		return nil, err
	}

	blobs := []*casext.Blob{}
	for _, layerDescriptor := range manifest.Layers {
		layerBlob, err := l.ext.FromDescriptor(context.Background(), layerDescriptor)
		if err != nil {
			return nil, err
		}

		blobs = append(blobs, layerBlob)
	}

	return blobs, nil
}

func (l *Layout) LookupConfig(desc ispec.Descriptor) (ispec.Image, error) {
	config, err := l.ext.FromDescriptor(context.Background(), desc)
	if err != nil {
		return ispec.Image{}, err
	}

	if config.MediaType != ispec.MediaTypeImageConfig {
		return ispec.Image{}, fmt.Errorf("bad image config: %s", config.MediaType)
	}

	img, ok := config.Data.(ispec.Image)
	if !ok {
		return ispec.Image{}, fmt.Errorf("BUG: image config not n ispec.Image?")
	}

	return img, nil
}

func (l *Layout) Unpack(tag string, path string, mo *layer.MapOptions) error {
	manifest, err := l.LookupManifest(tag)
	if err != nil {
		return err
	}

	return layer.UnpackManifest(context.Background(), l.ext, path, manifest, mo)
}
