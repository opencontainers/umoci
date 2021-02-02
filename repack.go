/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package umoci

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/mtreefilter"
	"github.com/pkg/errors"
	"github.com/vbatts/go-mtree"
)

// Repack repacks a bundle into an image adding a new layer for the changed
// data in the bundle.
func Repack(engineExt casext.Engine, tagName string, bundlePath string, meta Meta, history *ispec.History, filters []mtreefilter.FilterFunc, refreshBundle bool, mutator *mutate.Mutator) error {
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

	fsEval := fseval.Default
	if meta.MapOptions.Rootless {
		fsEval = fseval.Rootless
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

	allFilters := append(filters, mtreefilter.SimplifyFilter(diffs))
	diffs = mtreefilter.FilterDeltas(diffs, allFilters...)

	if len(diffs) == 0 {
		config, err := mutator.Config(context.Background())
		if err != nil {
			return err
		}

		imageMeta, err := mutator.Meta(context.Background())
		if err != nil {
			return err
		}

		annotations, err := mutator.Annotations(context.Background())
		if err != nil {
			return err
		}

		err = mutator.Set(context.Background(), config, imageMeta, annotations, history)
		if err != nil {
			return err
		}
	} else {
		packOptions := layer.RepackOptions{MapOptions: meta.MapOptions}
		if meta.WhiteoutMode == layer.OverlayFSWhiteout {
			packOptions.TranslateOverlayWhiteouts = true
		}
		reader, err := layer.GenerateLayer(fullRootfsPath, diffs, &packOptions)
		if err != nil {
			return errors.Wrap(err, "generate diff layer")
		}
		defer reader.Close()

		// TODO: We should add a flag to allow for a new layer to be made
		//       non-distributable.
		if _, err := mutator.Add(context.Background(), ispec.MediaTypeImageLayer, reader, history, mutate.GzipCompressor); err != nil {
			return errors.Wrap(err, "add diff layer")
		}
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
