package umoci

import (
	"runtime"
	"time"

	"github.com/apex/log"
	"github.com/openSUSE/umoci/oci/casext"
	igen "github.com/openSUSE/umoci/oci/config/generate"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// NewImage creates a new empty image (tag) in the existing layout.
func NewImage(engineExt casext.Engine, tagName string) error {
	// Create a new manifest.
	log.WithFields(log.Fields{
		"tag": tagName,
	}).Debugf("creating new manifest")

	// Create a new image config.
	g := igen.New()
	createTime := time.Now()

	// Set all of the defaults we need.
	g.SetCreated(createTime)
	g.SetOS(runtime.GOOS)
	g.SetArchitecture(runtime.GOARCH)
	g.ClearHistory()

	// Make sure we have no diffids.
	g.SetRootfsType("layers")
	g.ClearRootfsDiffIDs()

	// Update config and create a new blob for it.
	config := g.Image()
	configDigest, configSize, err := engineExt.PutBlobJSON(context.Background(), config)
	if err != nil {
		return errors.Wrap(err, "put config blob")
	}

	log.WithFields(log.Fields{
		"digest": configDigest,
		"size":   configSize,
	}).Debugf("umoci: added new config")

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
		Layers: []ispec.Descriptor{},
	}

	manifestDigest, manifestSize, err := engineExt.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		return errors.Wrap(err, "put manifest blob")
	}

	log.WithFields(log.Fields{
		"digest": manifestDigest,
		"size":   manifestSize,
	}).Debugf("umoci: added new manifest")

	// Now create a new reference, and either add it to the engine or spew it
	// to stdout.

	descriptor := ispec.Descriptor{
		// FIXME: Support manifest lists.
		MediaType: ispec.MediaTypeImageManifest,
		Digest:    manifestDigest,
		Size:      manifestSize,
	}

	log.Infof("new image manifest created: %s", descriptor.Digest)

	if err := engineExt.UpdateReference(context.Background(), tagName, descriptor); err != nil {
		return errors.Wrap(err, "add new tag")
	}

	log.Infof("created new tag for image manifest: %s", tagName)
	return nil
}
