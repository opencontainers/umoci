// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/mtreefilter"
)

// Repack repacks a bundle into an image adding a new layer for the changed
// data in the bundle.
//
// If layerCompressor is nil, the compression algorithm is auto-selected.
func Repack(engineExt casext.Engine, tagName, bundlePath string,
	meta Meta,
	history *ispec.History,
	filters []mtreefilter.FilterFunc,
	refreshBundle bool,
	mutator *mutate.Mutator,
	layerCompressor mutate.Compressor, //nolint:staticcheck // SA1019: this interface is defined by us and we keep it for compatibility
	sourceDateEpoch *time.Time,
) (Err error) {
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
		return fmt.Errorf("open mtree: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, mfh)

	spec, err := mtree.ParseSpec(mfh)
	if err != nil {
		return fmt.Errorf("parse mtree: %w", err)
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
		return fmt.Errorf("check mtree: %w", err)
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

		err = mutator.Set(context.Background(), config.Config, imageMeta, annotations, history)
		if err != nil {
			return err
		}
	} else {
		reader, err := layer.GenerateLayer(fullRootfsPath, diffs, &layer.RepackOptions{
			OnDiskFormat: layer.DirRootfs{
				MapOptions: meta.MapOptions,
			},
			SourceDateEpoch: sourceDateEpoch,
		})
		if err != nil {
			return fmt.Errorf("generate diff layer: %w", err)
		}
		defer funchelpers.VerifyClose(&Err, reader)

		if _, err := mutator.Add(context.Background(), ispec.MediaTypeImageLayer, reader, history, layerCompressor, nil); err != nil {
			return fmt.Errorf("add diff layer: %w", err)
		}
	}

	newDescriptorPath, err := mutator.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("commit mutated image: %w", err)
	}

	log.Infof("new image manifest created: %s->%s", newDescriptorPath.Root().Digest, newDescriptorPath.Descriptor().Digest)

	if err := engineExt.UpdateReference(context.Background(), tagName, newDescriptorPath.Root()); err != nil {
		return fmt.Errorf("add new tag: %w", err)
	}

	log.Infof("created new tag for image manifest: %s", tagName)

	if refreshBundle {
		newMtreeName := strings.Replace(newDescriptorPath.Descriptor().Digest.String(), ":", "_", 1)
		if err := GenerateBundleManifest(newMtreeName, bundlePath, fsEval); err != nil {
			return fmt.Errorf("write mtree metadata: %w", err)
		}
		if err := os.Remove(mtreePath); err != nil {
			return fmt.Errorf("remove old mtree metadata: %w", err)
		}
		meta.From = newDescriptorPath
		if err := WriteBundleMeta(bundlePath, meta); err != nil {
			return fmt.Errorf("write umoci.json metadata: %w", err)
		}
	}
	return nil
}
