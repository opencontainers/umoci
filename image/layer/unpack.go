/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016 SUSE LLC.
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

package layer

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	igen "github.com/cyphar/umoci/image/generator"
	"github.com/cyphar/umoci/pkg/idtools"
	"github.com/cyphar/umoci/system"
	"github.com/opencontainers/image-spec/specs-go/v1"
	rgen "github.com/opencontainers/runtime-tools/generate"
	"golang.org/x/net/context"
)

// UnpackLayer unpacks the tar stream representing an OCI layer at the given
// root. It ensures that the state of the root is as close as possible to the
// state used to create the layer. If an error is returned, the state of root
// is undefined (unpacking is not guaranteed to be atomic).
func UnpackLayer(root string, layer io.Reader, opt *MapOptions) error {
	var mapOptions MapOptions
	if opt != nil {
		mapOptions = *opt
	}
	te := newTarExtractor(mapOptions)
	tr := tar.NewReader(layer)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if err := te.unpackEntry(root, hdr, tr); err != nil {
			return err
		}
	}
	return nil
}

// RootfsName is the name of the rootfs directory inside the bundle path when
// generated.
const RootfsName = "rootfs"

// isLayerType returns if the given MediaType is the media type of an image
// layer blob. This includes both distributable and non-distributable images.
func isLayerType(mediaType string) bool {
	return mediaType == v1.MediaTypeImageLayer || mediaType == v1.MediaTypeImageLayerNonDistributable
}

// UnpackManifest extracts all of the layers in the given manifest, as well as
// generating a runtime bundle and configuration. The rootfs is extracted to
// <bundle>/<layer.RootfsName>. Some verification is done during image
// extraction.
//
// FIXME: This interface is ugly.
func UnpackManifest(ctx context.Context, engine cas.Engine, bundle string, manifest v1.Manifest, opt *MapOptions) error {
	// Create the bundle directory. We only error out if config.json or rootfs/
	// already exists, because we cannot be sure that the user intended us to
	// extract over an existing bundle.
	if err := os.MkdirAll(bundle, 0755); err != nil {
		return err
	}

	configPath := filepath.Join(bundle, "config.json")
	rootfsPath := filepath.Join(bundle, RootfsName)

	if _, err := os.Lstat(configPath); !os.IsNotExist(err) {
		if err == nil {
			err = fmt.Errorf("config.json: file already exists")
		}
		return fmt.Errorf("unpack manifest: checking bundle path is empty: %s", err)
	}

	if _, err := os.Lstat(rootfsPath); !os.IsNotExist(err) {
		if err == nil {
			err = fmt.Errorf("%s: file already exists", RootfsName)
		}
		return fmt.Errorf("unpack manifest: checking bundle path is empty: %s", err)
	}

	if err := os.Mkdir(rootfsPath, 0755); err != nil {
		return fmt.Errorf("unpack manifest: creating rootfs: %s", err)
	}

	// Make sure that the owner is correct.
	rootUID, err := idtools.ToHost(0, opt.UIDMappings)
	if err != nil {
		return fmt.Errorf("unpack manifest: creating rootfs: tohost(uid): %s", err)
	}
	rootGID, err := idtools.ToHost(0, opt.GIDMappings)
	if err != nil {
		return fmt.Errorf("unpack manifest: creating rootfs: tohost(gid): %s", err)
	}
	if err := os.Lchown(rootfsPath, rootUID, rootGID); err != nil {
		return fmt.Errorf("unpack manifest: creating rootfs: chown: %s", err)
	}

	// Currently, many different images in the wild don't specify what the
	// atime/mtime of the root directory is. This is a huge pain because it
	// means that we can't ensure consistent unpacking. In order to get around
	// this, we first set the mtime of the root directory to the Unix epoch
	// (which is as good of an arbitrary choice as any).
	epoch := time.Unix(0, 0)
	if err := system.Lutimes(rootfsPath, epoch, epoch); err != nil {
		return fmt.Errorf("unpack manifest: setting initial root time: %s", err)
	}

	// In order to verify the DiffIDs as we extract layers, we have to get the
	// .Config blob first. But we can't extract it (generate the runtime
	// config) until after we have the full rootfs generated.
	configBlob, err := cas.FromDescriptor(ctx, engine, &manifest.Config)
	if err != nil {
		return err
	}
	defer configBlob.Close()
	if configBlob.MediaType != v1.MediaTypeImageConfig {
		return fmt.Errorf("unpack manifest: config blob is not correct mediatype %s: %s", v1.MediaTypeImageConfig, configBlob.MediaType)
	}
	config := configBlob.Data.(*v1.Image)

	// We can't understand non-layer images.
	if config.RootFS.Type != "layers" {
		return fmt.Errorf("unpack manifest: config: unsupported rootfs.type: %s", config.RootFS.Type)
	}

	// Layer extraction.
	for idx, layerDescriptor := range manifest.Layers {
		layerDiffID := config.RootFS.DiffIDs[idx]
		logrus.WithFields(logrus.Fields{
			"diffid": layerDiffID,
		}).Infof("unpack manifest: unpacking layer %s", layerDescriptor.Digest)

		layerBlob, err := cas.FromDescriptor(ctx, engine, &layerDescriptor)
		if err != nil {
			return err
		}
		defer layerBlob.Close()
		if !isLayerType(layerBlob.MediaType) {
			return fmt.Errorf("unpack manifest: layer %s: blob is not correct mediatype: %s", layerBlob.Digest, layerBlob.MediaType)
		}
		layerGzip := layerBlob.Data.(io.ReadCloser)

		// We have to extract a gzip'd version of the above layer. Also note
		// that we have to check the DiffID we're extracting (which is the
		// sha256 sum of the *uncompressed* layer).
		layerRaw, err := gzip.NewReader(layerGzip)
		if err != nil {
			return err
		}
		layerHash := sha256.New()
		layer := io.TeeReader(layerRaw, layerHash)

		if err := UnpackLayer(rootfsPath, layer, opt); err != nil {
			return fmt.Errorf("unpack manifest: layer %s: %s", layerBlob.Digest, err)
		}
		layerGzip.Close()

		layerDigest := fmt.Sprintf("%s:%x", cas.BlobAlgorithm, layerHash.Sum(nil))
		if layerDigest != layerDiffID {
			return fmt.Errorf("unpack manifest: layer %s: diffid mismatch: got %s expected %s", layerDescriptor.Digest, layerDigest, layerDiffID)
		}
	}

	// Generate a runtime configuration file from v1.Image.
	logrus.WithFields(logrus.Fields{
		"config": manifest.Config.Digest,
	}).Infof("unpack manifest: unpacking config")

	g := rgen.New()
	if err := igen.MutateRuntimeSpec(g, rootfsPath, *config); err != nil {
		return fmt.Errorf("unpack manifest: generating config.json: %s", err)
	}
	if err := g.SaveToFile(configPath, rgen.ExportOptions{}); err != nil {
		return fmt.Errorf("failed to write new config.json: %s", err)
	}

	return nil
}
