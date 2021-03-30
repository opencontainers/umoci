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

package layer

import (
	"archive/tar"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	// Import is necessary for go-digest.
	_ "crypto/sha256"

	"github.com/apex/log"
	gzip "github.com/klauspost/pgzip"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/opencontainers/umoci/oci/cas"
	"github.com/opencontainers/umoci/oci/casext"
	iconv "github.com/opencontainers/umoci/oci/config/convert"
	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/idtools"
	"github.com/opencontainers/umoci/pkg/system"
	"github.com/pkg/errors"
)

// AfterLayerUnpackCallback is called after each layer is unpacked.
type AfterLayerUnpackCallback func(manifest ispec.Manifest, desc ispec.Descriptor) error

// UnpackLayer unpacks the tar stream representing an OCI layer at the given
// root. It ensures that the state of the root is as close as possible to the
// state used to create the layer. If an error is returned, the state of root
// is undefined (unpacking is not guaranteed to be atomic).
func UnpackLayer(root string, layer io.Reader, opt *UnpackOptions) error {
	var unpackOptions UnpackOptions
	if opt != nil {
		unpackOptions = *opt
	}
	te := NewTarExtractor(unpackOptions)
	tr := tar.NewReader(layer)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return errors.Wrap(err, "read next entry")
		}
		if err := te.UnpackEntry(root, hdr, tr); err != nil {
			return errors.Wrapf(err, "unpack entry: %s", hdr.Name)
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
	return mediaType == ispec.MediaTypeImageLayer || mediaType == ispec.MediaTypeImageLayerNonDistributable ||
		mediaType == ispec.MediaTypeImageLayerGzip || mediaType == ispec.MediaTypeImageLayerNonDistributableGzip
}

func needsGunzip(mediaType string) bool {
	return mediaType == ispec.MediaTypeImageLayerGzip || mediaType == ispec.MediaTypeImageLayerNonDistributableGzip
}

// UnpackManifest extracts all of the layers in the given manifest, as well as
// generating a runtime bundle and configuration. The rootfs is extracted to
// <bundle>/<layer.RootfsName>.
//
// FIXME: This interface is ugly.
func UnpackManifest(ctx context.Context, engine cas.Engine, bundle string, manifest ispec.Manifest, opt *UnpackOptions) (err error) {
	// Create the bundle directory. We only error out if config.json or rootfs/
	// already exists, because we cannot be sure that the user intended us to
	// extract over an existing bundle.
	if err := os.MkdirAll(bundle, 0755); err != nil {
		return errors.Wrap(err, "mkdir bundle")
	}
	// We change the mode of the bundle directory to 0700. A user can easily
	// change this after-the-fact, but we do this explicitly to avoid cases
	// where an unprivileged user could recurse into an otherwise unsafe image
	// (giving them potential root access through setuid binaries for example).
	if err := os.Chmod(bundle, 0700); err != nil {
		return errors.Wrap(err, "chmod bundle 0700")
	}

	configPath := filepath.Join(bundle, "config.json")
	rootfsPath := filepath.Join(bundle, RootfsName)

	if _, err := os.Lstat(configPath); !os.IsNotExist(err) {
		if err == nil {
			return errors.Errorf("config.json already exists in %s", bundle)
		}
		return errors.Wrap(err, "problem accessing bundle config")
	}

	defer func() {
		if err != nil {
			fsEval := fseval.Default
			if opt != nil && opt.MapOptions.Rootless {
				fsEval = fseval.Rootless
			}
			// It's too late to care about errors.
			// #nosec G104
			_ = fsEval.RemoveAll(rootfsPath)
		}
	}()

	if _, err := os.Lstat(rootfsPath); !os.IsNotExist(err) && opt.StartFrom.MediaType == "" {
		if err == nil {
			err = fmt.Errorf("%s already exists", rootfsPath)
		}
		return errors.Wrapf(err, "detecting rootfs")
	}

	log.Infof("unpack rootfs: %s", rootfsPath)
	if err := UnpackRootfs(ctx, engine, rootfsPath, manifest, opt); err != nil {
		return errors.Wrap(err, "unpack rootfs")
	}

	// Generate a runtime configuration file from ispec.Image.
	configFile, err := os.Create(configPath)
	if err != nil {
		return errors.Wrap(err, "open config.json")
	}
	defer configFile.Close()

	if err := UnpackRuntimeJSON(ctx, engine, configFile, rootfsPath, manifest, &opt.MapOptions); err != nil {
		return errors.Wrap(err, "unpack config.json")
	}
	return nil
}

// UnpackRootfs extracts all of the layers in the given manifest.
// Some verification is done during image extraction.
func UnpackRootfs(ctx context.Context, engine cas.Engine, rootfsPath string, manifest ispec.Manifest, opt *UnpackOptions) (err error) {
	engineExt := casext.NewEngine(engine)

	if err := os.Mkdir(rootfsPath, 0755); err != nil && !os.IsExist(err) {
		return errors.Wrap(err, "mkdir rootfs")
	}

	// In order to avoid having a broken rootfs in the case of an error, we
	// remove the rootfs. In the case of rootless this is particularly
	// important (`rm -rf` won't work on most distro rootfs's).
	defer func() {
		if err != nil {
			fsEval := fseval.Default
			if opt != nil && opt.MapOptions.Rootless {
				fsEval = fseval.Rootless
			}
			// It's too late to care about errors.
			// #nosec G104
			_ = fsEval.RemoveAll(rootfsPath)
		}
	}()

	// Make sure that the owner is correct.
	rootUID, err := idtools.ToHost(0, opt.MapOptions.UIDMappings)
	if err != nil {
		return errors.Wrap(err, "ensure rootuid has mapping")
	}
	rootGID, err := idtools.ToHost(0, opt.MapOptions.GIDMappings)
	if err != nil {
		return errors.Wrap(err, "ensure rootgid has mapping")
	}
	if err := os.Lchown(rootfsPath, rootUID, rootGID); err != nil {
		return errors.Wrap(err, "chown rootfs")
	}

	// Currently, many different images in the wild don't specify what the
	// atime/mtime of the root directory is. This is a huge pain because it
	// means that we can't ensure consistent unpacking. In order to get around
	// this, we first set the mtime of the root directory to the Unix epoch
	// (which is as good of an arbitrary choice as any).
	epoch := time.Unix(0, 0)
	if err := system.Lutimes(rootfsPath, epoch, epoch); err != nil {
		return errors.Wrap(err, "set initial root time")
	}

	// In order to verify the DiffIDs as we extract layers, we have to get the
	// .Config blob first. But we can't extract it (generate the runtime
	// config) until after we have the full rootfs generated.
	configBlob, err := engineExt.FromDescriptor(ctx, manifest.Config)
	if err != nil {
		return errors.Wrap(err, "get config blob")
	}
	defer configBlob.Close()
	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return errors.Errorf("unpack rootfs: config blob is not correct mediatype %s: %s", ispec.MediaTypeImageConfig, configBlob.Descriptor.MediaType)
	}
	config, ok := configBlob.Data.(ispec.Image)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown config blob type: %s", configBlob.Descriptor.MediaType)
	}

	// We can't understand non-layer images.
	if config.RootFS.Type != "layers" {
		return errors.Errorf("unpack rootfs: config: unsupported rootfs.type: %s", config.RootFS.Type)
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
			return errors.Wrap(err, "get layer blob")
		}
		defer layerBlob.Close()
		if !isLayerType(layerBlob.Descriptor.MediaType) {
			return errors.Errorf("unpack rootfs: layer %s: blob is not correct mediatype: %s", layerBlob.Descriptor.Digest, layerBlob.Descriptor.MediaType)
		}
		layerData, ok := layerBlob.Data.(io.ReadCloser)
		if !ok {
			// Should _never_ be reached.
			return errors.Errorf("[internal error] layerBlob was not an io.ReadCloser")
		}

		layerRaw := layerData
		if needsGunzip(layerBlob.Descriptor.MediaType) {
			// We have to extract a gzip'd version of the above layer. Also note
			// that we have to check the DiffID we're extracting (which is the
			// sha256 sum of the *uncompressed* layer).
			layerRaw, err = gzip.NewReader(layerData)
			if err != nil {
				return errors.Wrap(err, "create gzip reader")
			}
		}

		layerDigester := digest.SHA256.Digester()
		layer := io.TeeReader(layerRaw, layerDigester.Hash())

		if err := UnpackLayer(rootfsPath, layer, opt); err != nil {
			return errors.Wrap(err, "unpack layer")
		}
		// Different tar implementations can have different levels of redundant
		// padding and other similar weird behaviours. While on paper they are
		// all entirely valid archives, Go's tar.Reader implementation doesn't
		// guarantee that the entire stream will be consumed (which can result
		// in the later diff_id check failing because the digester didn't get
		// the whole uncompressed stream). Just blindly consume anything left
		// in the layer.
		if n, err := io.Copy(ioutil.Discard, layer); err != nil {
			return errors.Wrap(err, "discard trailing archive bits")
		} else if n != 0 {
			log.Debugf("unpack manifest: layer %s: ignoring %d trailing 'junk' bytes in the tar stream -- probably from GNU tar", layerDescriptor.Digest, n)
		}
		// Same goes for compressed layers -- it seems like some gzip
		// implementations add trailing NUL bytes, which Go doesn't slurp up.
		// Just eat up the rest of the remaining bytes and discard them.
		//
		// FIXME: We use layerData here because pgzip returns io.EOF from
		// WriteTo, which causes havoc with io.Copy. Ideally we would use
		// layerRaw. See <https://github.com/klauspost/pgzip/issues/38>.
		if n, err := io.Copy(ioutil.Discard, layerData); err != nil {
			return errors.Wrap(err, "discard trailing raw bits")
		} else if n != 0 {
			log.Warnf("unpack manifest: layer %s: ignoring %d trailing 'junk' bytes in the blob stream -- this may indicate a bug in the tool which built this image", layerDescriptor.Digest, n)
		}
		if err := layerData.Close(); err != nil {
			return errors.Wrap(err, "close layer data")
		}

		layerDigest := layerDigester.Digest()
		if layerDigest != layerDiffID {
			return errors.Errorf("unpack manifest: layer %s: diffid mismatch: got %s expected %s", layerDescriptor.Digest, layerDigest, layerDiffID)
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
func UnpackRuntimeJSON(ctx context.Context, engine cas.Engine, configFile io.Writer, rootfs string, manifest ispec.Manifest, opt *MapOptions) error {
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
		return errors.Wrap(err, "get config blob")
	}
	defer configBlob.Close()
	if configBlob.Descriptor.MediaType != ispec.MediaTypeImageConfig {
		return errors.Errorf("unpack manifest: config blob is not correct mediatype %s: %s", ispec.MediaTypeImageConfig, configBlob.Descriptor.MediaType)
	}
	config, ok := configBlob.Data.(ispec.Image)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown config blob type: %s", configBlob.Descriptor.MediaType)
	}

	spec, err := iconv.ToRuntimeSpec(rootfs, config)
	if err != nil {
		return errors.Wrap(err, "generate config.json")
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
			return errors.Wrap(err, "convert spec to rootless")
		}
	}

	// Save the config.json.
	enc := json.NewEncoder(configFile)
	enc.SetIndent("", "\t")
	return errors.Wrap(enc.Encode(spec), "write config.json")
}
