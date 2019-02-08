package umoci

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/openSUSE/umoci/mutate"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/openSUSE/umoci/pkg/mtreefilter"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/vbatts/go-mtree"
	"golang.org/x/net/context"
)

// Repack repacks a bundle into an image adding a new layer for the changed
// data in the bundle.
func Repack(engineExt casext.Engine, tagName string, bundlePath string, meta Meta, history *ispec.History, maskedPaths []string, refreshBundle bool, mutator *mutate.Mutator) error {
	mtreeName := strings.Replace(meta.From.Descriptor().Digest.String(), ":", "_", 1)
	mtreePath := filepath.Join(bundlePath, mtreeName+".mtree")
	fullRootfsPath := filepath.Join(bundlePath, layer.RootfsName)

	log.WithFields(log.Fields{
		"bundle": bundlePath,
		"rootfs": layer.RootfsName,
		"mtree":  mtreePath,
	}).Debugf("umoci: repacking OCI image")

	mfh, err := os.Open(mtreePath)
	if err != nil {
		return errors.Wrap(err, "open mtree")
	}
	defer mfh.Close()

	spec, err := mtree.ParseSpec(mfh)
	if err != nil {
		return errors.Wrap(err, "parse mtree")
	}

	log.WithFields(log.Fields{
		"keywords": MtreeKeywords,
	}).Debugf("umoci: parsed mtree spec")

	fsEval := fseval.DefaultFsEval
	if meta.MapOptions.Rootless {
		fsEval = fseval.RootlessFsEval
	}

	log.Info("computing filesystem diff ...")
	diffs, err := mtree.Check(fullRootfsPath, spec, MtreeKeywords, fsEval)
	if err != nil {
		return errors.Wrap(err, "check mtree")
	}
	log.Info("... done")

	log.WithFields(log.Fields{
		"ndiff": len(diffs),
	}).Debugf("umoci: checked mtree spec")

	diffs = mtreefilter.FilterDeltas(diffs,
		mtreefilter.MaskFilter(maskedPaths),
		mtreefilter.SimplifyFilter(diffs))

	reader, err := layer.GenerateLayer(fullRootfsPath, diffs, &meta.MapOptions)
	if err != nil {
		return errors.Wrap(err, "generate diff layer")
	}
	defer reader.Close()

	// TODO: We should add a flag to allow for a new layer to be made
	//       non-distributable.
	if err := mutator.Add(context.Background(), reader, history); err != nil {
		return errors.Wrap(err, "add diff layer")
	}

	newDescriptorPath, err := mutator.Commit(context.Background())
	if err != nil {
		return errors.Wrap(err, "commit mutated image")
	}

	log.Infof("new image manifest created: %s->%s", newDescriptorPath.Root().Digest, newDescriptorPath.Descriptor().Digest)

	if err := engineExt.UpdateReference(context.Background(), tagName, newDescriptorPath.Root()); err != nil {
		return errors.Wrap(err, "add new tag")
	}

	log.Infof("created new tag for image manifest: %s", tagName)

	if refreshBundle {
		newMtreeName := strings.Replace(newDescriptorPath.Descriptor().Digest.String(), ":", "_", 1)
		if err := GenerateBundleManifest(newMtreeName, bundlePath, fsEval); err != nil {
			return errors.Wrap(err, "write mtree metadata")
		}
		if err := os.Remove(mtreePath); err != nil {
			return errors.Wrap(err, "remove old mtree metadata")
		}
		meta.From = newDescriptorPath
		if err := WriteBundleMeta(bundlePath, meta); err != nil {
			return errors.Wrap(err, "write umoci.json metadata")
		}
	}
	return nil
}
