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
	"strings"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/opencontainers/umoci/pkg/fseval"
)

// Unpack unpacks an image to the specified bundle path.
func Unpack(engineExt casext.Engine, fromName, bundlePath string, unpackOptions layer.UnpackOptions) (Err error) {
	meta := Meta{
		Version:    MetaVersion,
		MapOptions: unpackOptions.MapOptions(),
	}

	fromDescriptorPaths, err := engineExt.ResolveReference(context.Background(), fromName)
	if err != nil {
		return fmt.Errorf("get descriptor: %w", err)
	}
	if len(fromDescriptorPaths) == 0 {
		return fmt.Errorf("tag is not found: %s", fromName)
	}
	if len(fromDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return fmt.Errorf("tag is ambiguous: %s", fromName)
	}
	meta.From = fromDescriptorPaths[0]

	manifestBlob, err := engineExt.FromDescriptor(context.Background(), meta.From.Descriptor())
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, manifestBlob)

	if manifestBlob.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return fmt.Errorf("invalid --image tag: descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestBlob.Descriptor.MediaType)
	}

	mtreeName := strings.Replace(meta.From.Descriptor().Digest.String(), ":", "_", 1)
	log.WithFields(log.Fields{
		"bundle": bundlePath,
		"ref":    fromName,
		"rootfs": layer.RootfsName,
	}).Debugf("umoci: unpacking OCI image")

	// Get the manifest.
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return fmt.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.Descriptor.MediaType)
	}

	// Unpack the runtime bundle.
	if err := os.MkdirAll(bundlePath, 0o755); err != nil {
		return fmt.Errorf("create bundle path: %w", err)
	}
	// XXX: We should probably defer os.RemoveAll(bundlePath).

	log.Info("unpacking bundle ...")
	if err := layer.UnpackManifest(context.Background(), engineExt, bundlePath, manifest, &unpackOptions); err != nil {
		return fmt.Errorf("create runtime bundle: %w", err)
	}
	log.Info("... done")

	fsEval := fseval.Default
	if meta.MapOptions.Rootless {
		fsEval = fseval.Rootless
	}

	if err := GenerateBundleManifest(mtreeName, bundlePath, fsEval); err != nil {
		return fmt.Errorf("write mtree: %w", err)
	}

	log.WithFields(log.Fields{
		"version":     meta.Version,
		"from":        meta.From,
		"map_options": meta.MapOptions,
	}).Debugf("umoci: saving Meta metadata")

	if err := WriteBundleMeta(bundlePath, meta); err != nil {
		return fmt.Errorf("write umoci.json metadata: %w", err)
	}

	log.Infof("unpacked image bundle: %s", bundlePath)
	return nil
}
