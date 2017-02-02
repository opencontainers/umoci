/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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
	"strings"
	"time"

	"github.com/apex/log"
	"github.com/cyphar/umoci/oci/cas"
	igen "github.com/cyphar/umoci/oci/generate"
	"github.com/cyphar/umoci/pkg/idtools"
	"github.com/cyphar/umoci/pkg/system"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	rgen "github.com/opencontainers/runtime-tools/generate"
	"github.com/pkg/errors"
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
			return errors.Wrap(err, "read next entry")
		}
		if err := te.unpackEntry(root, hdr, tr); err != nil {
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
	return mediaType == ispec.MediaTypeImageLayer || mediaType == ispec.MediaTypeImageLayerNonDistributable
}

// UnpackManifest extracts all of the layers in the given manifest, as well as
// generating a runtime bundle and configuration. The rootfs is extracted to
// <bundle>/<layer.RootfsName>. Some verification is done during image
// extraction.
//
// FIXME: This interface is ugly.
func UnpackManifest(ctx context.Context, engine cas.Engine, bundle string, manifest ispec.Manifest, opt *MapOptions) error {
	var mapOptions MapOptions
	if opt != nil {
		mapOptions = *opt
	}

	// Create the bundle directory. We only error out if config.json or rootfs/
	// already exists, because we cannot be sure that the user intended us to
	// extract over an existing bundle.
	if err := os.MkdirAll(bundle, 0755); err != nil {
		return errors.Wrap(err, "mkdir bundle")
	}

	configPath := filepath.Join(bundle, "config.json")
	rootfsPath := filepath.Join(bundle, RootfsName)

	if _, err := os.Lstat(configPath); !os.IsNotExist(err) {
		if err == nil {
			err = fmt.Errorf("config.json already exists")
		}
		return errors.Wrap(err, "bundle path empty")
	}

	if _, err := os.Lstat(rootfsPath); !os.IsNotExist(err) {
		if err == nil {
			err = fmt.Errorf("%s already exists", RootfsName)
		}
		return errors.Wrap(err, "bundle path empty")
	}

	if err := os.Mkdir(rootfsPath, 0755); err != nil {
		return errors.Wrap(err, "mkdir rootfs")
	}

	// Make sure that the owner is correct.
	rootUID, err := idtools.ToHost(0, opt.UIDMappings)
	if err != nil {
		return errors.Wrap(err, "ensure rootuid has mapping")
	}
	rootGID, err := idtools.ToHost(0, opt.GIDMappings)
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
	configBlob, err := cas.FromDescriptor(ctx, engine, &manifest.Config)
	if err != nil {
		return errors.Wrap(err, "get config blob")
	}
	defer configBlob.Close()
	if configBlob.MediaType != ispec.MediaTypeImageConfig {
		return errors.Errorf("unpack manifest: config blob is not correct mediatype %s: %s", ispec.MediaTypeImageConfig, configBlob.MediaType)
	}
	config := configBlob.Data.(*ispec.Image)

	// We can't understand non-layer images.
	if config.RootFS.Type != "layers" {
		return errors.Errorf("unpack manifest: config: unsupported rootfs.type: %s", config.RootFS.Type)
	}

	// Layer extraction.
	for idx, layerDescriptor := range manifest.Layers {
		layerDiffID := config.RootFS.DiffIDs[idx]
		log.Infof("unpack layer: %s", layerDescriptor.Digest)

		layerBlob, err := cas.FromDescriptor(ctx, engine, &layerDescriptor)
		if err != nil {
			return errors.Wrap(err, "get layer blob")
		}
		defer layerBlob.Close()
		if !isLayerType(layerBlob.MediaType) {
			return errors.Errorf("unpack manifest: layer %s: blob is not correct mediatype: %s", layerBlob.Digest, layerBlob.MediaType)
		}
		layerGzip := layerBlob.Data.(io.ReadCloser)

		// We have to extract a gzip'd version of the above layer. Also note
		// that we have to check the DiffID we're extracting (which is the
		// sha256 sum of the *uncompressed* layer).
		layerRaw, err := gzip.NewReader(layerGzip)
		if err != nil {
			return errors.Wrap(err, "create gzip reader")
		}
		layerHash := sha256.New()
		layer := io.TeeReader(layerRaw, layerHash)

		if err := UnpackLayer(rootfsPath, layer, opt); err != nil {
			return errors.Wrap(err, "unpack layer")
		}
		layerGzip.Close()

		layerDigest := fmt.Sprintf("%s:%x", cas.BlobAlgorithm, layerHash.Sum(nil))
		if layerDigest != layerDiffID {
			return errors.Errorf("unpack manifest: layer %s: diffid mismatch: got %s expected %s", layerDescriptor.Digest, layerDigest, layerDiffID)
		}
	}

	// Generate a runtime configuration file from ispec.Image.
	log.Infof("unpack configuration: %s", configBlob.Digest)

	g := rgen.New()
	if err := igen.MutateRuntimeSpec(g, rootfsPath, *config, manifest); err != nil {
		return errors.Wrap(err, "generate config.json")
	}

	// Add UIDMapping / GIDMapping options.
	if len(mapOptions.UIDMappings) > 0 || len(mapOptions.GIDMappings) > 0 {
		g.AddOrReplaceLinuxNamespace("user", "")
	}
	g.ClearLinuxUIDMappings()
	for _, m := range mapOptions.UIDMappings {
		g.AddLinuxUIDMapping(m.HostID, m.ContainerID, m.Size)
	}
	g.ClearLinuxGIDMappings()
	for _, m := range mapOptions.GIDMappings {
		g.AddLinuxGIDMapping(m.HostID, m.ContainerID, m.Size)
	}
	if mapOptions.Rootless {
		ToRootless(g.Spec())
		g.AddBindMount("/etc/resolv.conf", "/etc/resolv.conf", []string{"bind", "ro"})
	}

	// Save the config.json.
	if err := g.SaveToFile(configPath, rgen.ExportOptions{}); err != nil {
		return errors.Wrap(err, "write config.json")
	}
	return nil
}

// ToRootless converts a specification to a version that works with rootless
// containers. This is done by removing options and other settings that clash
// with unprivileged user namespaces.
func ToRootless(spec *rspec.Spec) {
	var namespaces []rspec.Namespace

	// Remove networkns from the spec.
	for _, ns := range spec.Linux.Namespaces {
		switch ns.Type {
		case rspec.NetworkNamespace, rspec.UserNamespace:
			// Do nothing.
		default:
			namespaces = append(namespaces, ns)
		}
	}
	// Add userns to the spec.
	namespaces = append(namespaces, rspec.Namespace{
		Type: rspec.UserNamespace,
	})
	spec.Linux.Namespaces = namespaces

	// Fix up mounts.
	var mounts []rspec.Mount
	for _, mount := range spec.Mounts {
		// Ignore all mounts that are under /sys.
		if strings.HasPrefix(mount.Destination, "/sys") {
			continue
		}

		// Remove all gid= and uid= mappings.
		var options []string
		for _, option := range mount.Options {
			if !strings.HasPrefix(option, "gid=") && !strings.HasPrefix(option, "uid=") {
				options = append(options, option)
			}
		}

		mount.Options = options
		mounts = append(mounts, mount)
	}
	// Add the sysfs mount as an rbind.
	mounts = append(mounts, rspec.Mount{
		Source:      "/sys",
		Destination: "/sys",
		Type:        "none",
		Options:     []string{"rbind", "nosuid", "noexec", "nodev", "ro"},
	})
	spec.Mounts = mounts

	// Remove cgroup settings.
	spec.Linux.Resources = nil
}
