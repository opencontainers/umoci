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

package layer

import (
	"archive/tar"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	// Import is necessary for go-digest.
	_ "crypto/sha256"

	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"

	"github.com/opencontainers/umoci/internal"
	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/idtools"
	"github.com/opencontainers/umoci/internal/system"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
	"github.com/opencontainers/umoci/oci/casext/mediatype"
	iconv "github.com/opencontainers/umoci/oci/config/convert"
	"github.com/opencontainers/umoci/pkg/fseval"
)

// AfterLayerUnpackCallback is called after each layer is unpacked.
type AfterLayerUnpackCallback func(manifest ispec.Manifest, desc ispec.Descriptor) error

// UnpackLayer unpacks the tar stream representing an OCI layer at the given
// root. It ensures that the state of the root is as close as possible to the
// state used to create the layer. If an error is returned, the state of root
// is undefined (unpacking is not guaranteed to be atomic).
func UnpackLayer(root string, layer io.Reader, opt *UnpackOptions) error {
	opt = opt.fill()

	te := NewTarExtractor(opt)
	tr := tar.NewReader(layer)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read next entry: %w", err)
		}
		if err := te.UnpackEntry(root, hdr, tr); err != nil {
			return fmt.Errorf("unpack entry: %s: %w", hdr.Name, err)
		}
	}
	return nil
}

// RootfsName is the name of the rootfs directory inside the bundle path when
// generated.
const RootfsName = "rootfs"

func isLayerType(mediaType string) bool {
	layerMediaType, _ := mediatype.SplitMediaTypeSuffix(mediaType)
	switch layerMediaType {
	case ispec.MediaTypeImageLayerNonDistributable: //nolint:staticcheck // we need to support this deprecated media-type
		log.Infof("image contains layers using the deprecated 'non-distributable' media type %q", layerMediaType)
		fallthrough
	case ispec.MediaTypeImageLayer:
		return true
	}
	return false
}

func getLayerCompressAlgorithm(mediaType string) (string, blobcompress.Algorithm, error) {
	_, compressType := mediatype.SplitMediaTypeSuffix(mediaType)
	algorithm := blobcompress.GetAlgorithm(compressType)
	if algorithm == nil {
		return "", nil, fmt.Errorf("unsupported layer media type %q: compression method %q unsupported", mediaType, compressType)
	}
	return compressType, algorithm, nil
}

// UnpackManifest extracts all of the layers in the given manifest, as well as
// generating a runtime bundle and configuration. The rootfs is extracted to
// <bundle>/<layer.RootfsName>.
//
// FIXME: This interface is ugly.
func UnpackManifest(ctx context.Context, engine cas.Engine, bundle string, manifest ispec.Manifest, opt *UnpackOptions) (Err error) {
	opt = opt.fill()

	// Create the bundle directory. We only error out if config.json or rootfs/
	// already exists, because we cannot be sure that the user intended us to
	// extract over an existing bundle.
	if err := os.MkdirAll(bundle, 0o755); err != nil {
		return fmt.Errorf("mkdir bundle: %w", err)
	}
	// We change the mode of the bundle directory to 0700. A user can easily
	// change this after-the-fact, but we do this explicitly to avoid cases
	// where an unprivileged user could recurse into an otherwise unsafe image
	// (giving them potential root access through setuid binaries for example).
	if err := os.Chmod(bundle, 0o700); err != nil {
		return fmt.Errorf("chmod bundle 0700: %w", err)
	}

	configPath := filepath.Join(bundle, "config.json")
	rootfsPath := filepath.Join(bundle, RootfsName)

	if _, err := os.Lstat(configPath); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return fmt.Errorf("config.json already exists in %s", bundle)
		}
		return fmt.Errorf("problem accessing bundle config: %w", err)
	}

	defer func() {
		if Err != nil {
			fsEval := fseval.Default
			if opt != nil && opt.MapOptions().Rootless {
				fsEval = fseval.Rootless
			}
			// It's too late to care about errors.
			_ = fsEval.RemoveAll(rootfsPath)
		}
	}()

	if _, err := os.Lstat(rootfsPath); !errors.Is(err, os.ErrNotExist) && opt.StartFrom.MediaType == "" {
		if err == nil {
			err = fmt.Errorf("%s already exists", rootfsPath)
		}
		return fmt.Errorf("detecting rootfs: %w", err)
	}

	log.Infof("unpack rootfs: %s", rootfsPath)
	if err := UnpackRootfs(ctx, engine, rootfsPath, manifest, opt); err != nil {
		return fmt.Errorf("unpack rootfs: %w", err)
	}

	// Generate a runtime configuration file from ispec.Image.
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("open config.json: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, configFile)

	mapOptions := opt.MapOptions()
	if err := UnpackRuntimeJSON(ctx, engine, configFile, rootfsPath, manifest, &mapOptions); err != nil {
		return fmt.Errorf("unpack config.json: %w", err)
	}
	return nil
}

// UnpackRootfs extracts all of the layers in the given manifest.
// Some verification is done during image extraction.
func UnpackRootfs(ctx context.Context, engine cas.Engine, rootfsPath string, manifest ispec.Manifest, opt *UnpackOptions) (Err error) {
	opt = opt.fill()

	// TODO: For now, unpacking layers into a bundle with the overlayfs on-disk
	// format is not supported, because we still unpack everything into a
	// single rootfs directory. For more information about outstanding issues,
	// see <https://github.com/opencontainers/umoci/issues/574>.
	if _, isOciFmt := opt.OnDiskFormat.(DirRootfs); !isOciFmt {
		return fmt.Errorf("%w: umoci cannot yet unpack a manifest into a bundle using anything other than the dir-rootfs on-disk format", internal.ErrUnimplemented)
	}

	engineExt := casext.NewEngine(engine)

	if err := os.Mkdir(rootfsPath, 0o755); err != nil && !os.IsExist(err) {
		return fmt.Errorf("mkdir rootfs: %w", err)
	}

	// In order to avoid having a broken rootfs in the case of an error, we
	// remove the rootfs. In the case of rootless this is particularly
	// important (`rm -rf` won't work on most distro rootfs's).
	defer func() {
		if Err != nil {
			fsEval := fseval.Default
			if opt != nil && opt.MapOptions().Rootless {
				fsEval = fseval.Rootless
			}
			// It's too late to care about errors.
			_ = fsEval.RemoveAll(rootfsPath)
		}
	}()

	// Make sure that the owner is correct.
	rootUID, err := idtools.ToHost(0, opt.MapOptions().UIDMappings)
	if err != nil {
		return fmt.Errorf("ensure rootuid has mapping: %w", err)
	}
	rootGID, err := idtools.ToHost(0, opt.MapOptions().GIDMappings)
	if err != nil {
		return fmt.Errorf("ensure rootgid has mapping: %w", err)
	}
	if err := os.Lchown(rootfsPath, rootUID, rootGID); err != nil {
		return fmt.Errorf("chown rootfs: %w", err)
	}

	// Currently, many different images in the wild don't specify what the
	// atime/mtime of the root directory is. This is a huge pain because it
	// means that we can't ensure consistent unpacking. In order to get around
	// this, we first set the mtime of the root directory to the Unix epoch
	// (which is as good of an arbitrary choice as any).
	epoch := time.Unix(0, 0)
	if err := system.Lutimes(rootfsPath, epoch, epoch); err != nil {
		return fmt.Errorf("set initial root time: %w", err)
	}

	// In order to verify the DiffIDs as we extract layers, we have to get the
	// .Config blob first. But we can't extract it (generate the runtime
	// config) until after we have the full rootfs generated.
	configBlob, err := engineExt.FromDescriptor(ctx, manifest.Config)
	if err != nil {
		return fmt.Errorf("get config blob: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, configBlob)
	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return fmt.Errorf("unpack rootfs: config blob is not correct mediatype %s: %s", ispec.MediaTypeImageConfig, configBlob.Descriptor.MediaType)
	}
	config, ok := configBlob.Data.(ispec.Image)
	if !ok {
		// Should _never_ be reached.
		return fmt.Errorf("[internal error] unknown config blob type: %s", configBlob.Descriptor.MediaType)
	}

	// We can't understand non-layer images.
	if config.RootFS.Type != "layers" {
		return fmt.Errorf("unpack rootfs: config: unsupported rootfs.type: %s", config.RootFS.Type)
	}

	// Layer extraction.
	found := false
	for idx, layerDescriptor := range manifest.Layers {
		if !found && opt.StartFrom.MediaType != "" && layerDescriptor.Digest.String() != opt.StartFrom.Digest.String() {
			continue
		}
		found = true

		layerDiffID := config.RootFS.DiffIDs[idx]
		log.Infof("unpack layer: %s", layerDescriptor.Digest)

		layerBlob, err := engineExt.FromDescriptor(ctx, layerDescriptor)
		if err != nil {
			return fmt.Errorf("get layer blob: %w", err)
		}
		defer layerBlob.Close() //nolint:errcheck // in the non-error path this is a double-close we can ignore
		if !isLayerType(layerBlob.Descriptor.MediaType) {
			return fmt.Errorf("unpack rootfs: layer %s: layer data is an unsupported mediatype: %s", layerBlob.Descriptor.Digest, layerBlob.Descriptor.MediaType)
		}
		layerData, ok := layerBlob.Data.(io.ReadCloser)
		if !ok {
			// Should _never_ be reached.
			return errors.New("[internal error] layerBlob was not an io.ReadCloser")
		}

		layerRaw := layerData

		// Pick the decompression algorithm based on the media-type.
		if compressType, compressAlgo, err := getLayerCompressAlgorithm(layerBlob.Descriptor.MediaType); err != nil {
			return fmt.Errorf("unpack rootfs: layer %s: could not decompress layer: %w", layerBlob.Descriptor.Digest, err)
		} else if compressAlgo != nil {
			// We have to extract a compressed version of the above layer. Also
			// note that we have to check the DiffID we're extracting (which is
			// the sha256 sum of the *uncompressed* layer).
			layerRaw, err = compressAlgo.Decompress(layerData)
			if err != nil {
				return fmt.Errorf("create %s reader: %w", compressType, err)
			}
		}

		layerDigester := digest.SHA256.Digester()
		layer := io.TeeReader(layerRaw, layerDigester.Hash())

		if err := UnpackLayer(rootfsPath, layer, opt); err != nil {
			return fmt.Errorf("unpack layer: %w", err)
		}
		// Different tar implementations can have different levels of redundant
		// padding and other similar weird behaviours. While on paper they are
		// all entirely valid archives, Go's tar.Reader implementation doesn't
		// guarantee that the entire stream will be consumed (which can result
		// in the later diff_id check failing because the digester didn't get
		// the whole uncompressed stream). Just blindly consume anything left
		// in the layer.
		if n, err := system.Copy(io.Discard, layer); err != nil {
			return fmt.Errorf("discard trailing archive bits: %w", err)
		} else if n != 0 {
			log.Debugf("unpack manifest: layer %s: ignoring %d trailing 'junk' bytes in the tar stream -- probably from GNU tar", layerDescriptor.Digest, n)
		}
		// Same goes for compressed layers -- it seems like some gzip
		// implementations add trailing NUL bytes, which Go doesn't slurp up.
		// Just eat up the rest of the remaining bytes and discard them.
		//
		// FIXME: We use layerData here because pgzip returns io.EOF from
		// WriteTo, which causes havoc with system.Copy. Ideally we would use
		// layerRaw. See <https://github.com/klauspost/pgzip/issues/38>.
		if n, err := system.Copy(io.Discard, layerData); err != nil {
			return fmt.Errorf("discard trailing raw bits: %w", err)
		} else if n != 0 {
			log.Warnf("unpack manifest: layer %s: ignoring %d trailing 'junk' bytes in the blob stream -- this may indicate a bug in the tool which built this image", layerDescriptor.Digest, n)
		}
		if err := layerData.Close(); err != nil {
			return fmt.Errorf("close layer data: %w", err)
		}

		layerDigest := layerDigester.Digest()
		if layerDigest != layerDiffID {
			return fmt.Errorf("unpack manifest: layer %s: diffid mismatch: got %s expected %s", layerDescriptor.Digest, layerDigest, layerDiffID)
		}

		if opt.AfterLayerUnpack != nil {
			if err := opt.AfterLayerUnpack(manifest, layerDescriptor); err != nil {
				return err
			}
		}
	}

	return nil
}

// UnpackRuntimeJSON converts a given manifest's configuration to a runtime
// configuration and writes it to the given writer. If rootfs is specified, it
// is sourced during the configuration generation (for conversion of
// Config.User and other similar jobs -- which will error out if the user could
// not be parsed). If rootfs is not specified (is an empty string) then all
// conversions that require sourcing the rootfs will be set to their default
// values.
//
// XXX: I don't like this API. It has way too many arguments.
func UnpackRuntimeJSON(ctx context.Context, engine cas.Engine, configFile io.Writer, rootfs string, manifest ispec.Manifest, opt *MapOptions) (Err error) {
	engineExt := casext.NewEngine(engine)

	var mapOptions MapOptions
	if opt != nil {
		mapOptions = *opt
	}

	// In order to verify the DiffIDs as we extract layers, we have to get the
	// .Config blob first. But we can't extract it (generate the runtime
	// config) until after we have the full rootfs generated.
	configBlob, err := engineExt.FromDescriptor(ctx, manifest.Config)
	if err != nil {
		return fmt.Errorf("get config blob: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, configBlob)
	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return fmt.Errorf("unpack manifest: config blob is not correct mediatype %s: %s", ispec.MediaTypeImageConfig, configBlob.Descriptor.MediaType)
	}
	config, ok := configBlob.Data.(ispec.Image)
	if !ok {
		// Should _never_ be reached.
		return fmt.Errorf("[internal error] unknown config blob type: %s", configBlob.Descriptor.MediaType)
	}

	spec, err := iconv.ToRuntimeSpec(rootfs, config)
	if err != nil {
		return fmt.Errorf("generate config.json: %w", err)
	}

	// Add UIDMapping / GIDMapping options.
	if len(mapOptions.UIDMappings) > 0 || len(mapOptions.GIDMappings) > 0 {
		var namespaces []rspec.LinuxNamespace
		for _, ns := range spec.Linux.Namespaces {
			if ns.Type == "user" {
				continue
			}
			namespaces = append(namespaces, ns)
		}
		spec.Linux.Namespaces = append(namespaces, rspec.LinuxNamespace{
			Type: "user",
		})
	}
	spec.Linux.UIDMappings = mapOptions.UIDMappings
	spec.Linux.GIDMappings = mapOptions.GIDMappings
	if mapOptions.Rootless {
		if err := iconv.ToRootless(&spec); err != nil {
			return fmt.Errorf("convert spec to rootless: %w", err)
		}
	}

	// Save the config.json.
	enc := json.NewEncoder(configFile)
	enc.SetIndent("", "\t")
	if err := enc.Encode(spec); err != nil {
		return fmt.Errorf("write config.json: %w", err)
	}
	return nil
}
