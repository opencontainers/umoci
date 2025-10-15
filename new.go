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
	"time"

	"github.com/apex/log"
	"github.com/containerd/platforms"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/oci/casext"
	igen "github.com/opencontainers/umoci/oci/config/generate"
)

// NewImage creates a new empty image (tag) in the existing layout.
func NewImage(engineExt casext.Engine, tagName string, sourceDateEpoch *time.Time) error {
	// Create a new manifest.
	log.WithFields(log.Fields{
		"tag": tagName,
	}).Debugf("creating new manifest")

	// Create a new image config.
	g := igen.New()
	createTime := time.Now()
	if sourceDateEpoch != nil {
		createTime = *sourceDateEpoch
	}

	g.SetCreated(createTime)
	g.ClearHistory()

	hostPlatform := platforms.DefaultSpec()
	g.SetPlatformOS(hostPlatform.OS)
	g.SetPlatformArchitecture(hostPlatform.Architecture)
	g.SetPlatformVariant(hostPlatform.Variant)

	// Make sure we have no diffids.
	g.SetRootfsType("layers")
	g.ClearRootfsDiffIDs()

	// Update config and create a new blob for it.
	config := g.Image()
	configDigest, configSize, err := engineExt.PutBlobJSON(context.Background(), config)
	if err != nil {
		return fmt.Errorf("put config blob: %w", err)
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
		MediaType: ispec.MediaTypeImageManifest,
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageConfig,
			Digest:    configDigest,
			Size:      configSize,
		},
		Layers: []ispec.Descriptor{},
	}

	manifestDigest, manifestSize, err := engineExt.PutBlobJSON(context.Background(), manifest)
	if err != nil {
		return fmt.Errorf("put manifest blob: %w", err)
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
		return fmt.Errorf("add new tag: %w", err)
	}

	log.Infof("created new tag for image manifest: %s", tagName)
	return nil
}
