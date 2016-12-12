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

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	"github.com/cyphar/umoci/image/layer"
	"github.com/cyphar/umoci/pkg/idtools"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"github.com/vbatts/go-mtree"
	"golang.org/x/net/context"
)

var unpackCommand = cli.Command{
	Name:  "unpack",
	Usage: "unpacks a reference into an OCI runtime bundle",
	ArgsUsage: `--image <image-path>[:<tag>] <bundle>

Where "<image-path>" is the path to the OCI image, "<tag>" is the name of the
tagged image to unpack (if not specified, defaults to "latest") and "<bundle>"
is the destination to unpack the image to.

It should be noted that this is not the same as oci-create-runtime-bundle,
because this command also will create an mtree specification to allow for layer
creation with umoci-repack(1).`,

	// unpack reads manifest information.
	Category: "image",

	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "uid-map",
			Usage: "specifies a uid mapping to use when repacking",
		},
		cli.StringSliceFlag{
			Name:  "gid-map",
			Usage: "specifies a gid mapping to use when repacking",
		},
		cli.BoolFlag{
			Name:  "rootless",
			Usage: "enable rootless unpacking support",
		},
	},

	Action: unpack,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.Errorf("invalid number of positional arguments: expected <bundle>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("bundle path cannot be empty")
		}
		ctx.App.Metadata["bundle"] = ctx.Args().First()
		return nil
	},
}

func getConfig(ctx context.Context, engine cas.Engine, manDescriptor *v1.Descriptor) (v1.Image, error) {
	// FIXME: Implement support for manifest lists.
	if manDescriptor.MediaType != v1.MediaTypeImageManifest {
		return v1.Image{}, errors.Wrap(fmt.Errorf("descriptor does not point to v1.MediaTypeImageManifest: not implemented: %s", manDescriptor.MediaType), "invalid --image tag")
	}

	manBlob, err := cas.FromDescriptor(ctx, engine, manDescriptor)
	if err != nil {
		return v1.Image{}, err
	}

	configBlob, err := cas.FromDescriptor(ctx, engine, &manBlob.Data.(*v1.Manifest).Config)
	if err != nil {
		return v1.Image{}, err
	}

	return *configBlob.Data.(*v1.Image), nil
}

func unpack(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	bundlePath := ctx.App.Metadata["bundle"].(string)

	var meta UmociMeta
	meta.Version = ctx.App.Version

	// Parse map options.
	// We need to set mappings if we're in rootless mode.
	meta.MapOptions.Rootless = ctx.Bool("rootless")
	if meta.MapOptions.Rootless {
		if !ctx.IsSet("uid-map") {
			ctx.Set("uid-map", fmt.Sprintf("%d:0:1", os.Geteuid()))
			logrus.WithFields(logrus.Fields{
				"map.uid": ctx.StringSlice("uid-map"),
			}).Info("setting default rootless --uid-map option")
		}
		if !ctx.IsSet("gid-map") {
			ctx.Set("gid-map", fmt.Sprintf("%d:0:1", os.Getegid()))
			logrus.WithFields(logrus.Fields{
				"map.gid": ctx.StringSlice("gid-map"),
			}).Info("setting default rootless --gid-map option")
		}
	}
	// Parse and set up the mapping options.
	for _, uidmap := range ctx.StringSlice("uid-map") {
		idMap, err := idtools.ParseMapping(uidmap)
		if err != nil {
			return errors.Wrapf(err, "failure parsing --uid-map %s: %s", uidmap)
		}
		meta.MapOptions.UIDMappings = append(meta.MapOptions.UIDMappings, idMap)
	}
	for _, gidmap := range ctx.StringSlice("gid-map") {
		idMap, err := idtools.ParseMapping(gidmap)
		if err != nil {
			return errors.Wrapf(err, "failure parsing --gid-map %s: %s", gidmap)
		}
		meta.MapOptions.GIDMappings = append(meta.MapOptions.GIDMappings, idMap)
	}
	logrus.WithFields(logrus.Fields{
		"map.uid": meta.MapOptions.UIDMappings,
		"map.gid": meta.MapOptions.GIDMappings,
	}).Infof("parsed mappings")

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	defer engine.Close()

	fromDescriptor, err := engine.GetReference(context.Background(), fromName)
	if err != nil {
		return errors.Wrap(err, "get descriptor")
	}
	meta.From = *fromDescriptor

	manifestBlob, err := cas.FromDescriptor(context.Background(), engine, &meta.From)
	if err != nil {
		return errors.Wrap(err, "get manifest")
	}
	defer manifestBlob.Close()

	// FIXME: Implement support for manifest lists.
	if manifestBlob.MediaType != v1.MediaTypeImageManifest {
		return errors.Wrap(fmt.Errorf("descriptor does not point to v1.MediaTypeImageManifest: not implemented: %s", meta.From.MediaType), "invalid --image tag")
	}

	mtreeName := strings.Replace(meta.From.Digest, "sha256:", "sha256_", 1)
	mtreePath := filepath.Join(bundlePath, mtreeName+".mtree")
	fullRootfsPath := filepath.Join(bundlePath, layer.RootfsName)

	logrus.WithFields(logrus.Fields{
		"image":  imagePath,
		"bundle": bundlePath,
		"ref":    fromName,
		"rootfs": layer.RootfsName,
	}).Debugf("umoci: unpacking OCI image")

	// Get the manifest.
	manifest := manifestBlob.Data.(*v1.Manifest)

	// Unpack the runtime bundle.
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return errors.Wrap(err, "create bundle path")
	}
	// XXX: We should probably defer os.RemoveAll(bundlePath).

	// FIXME: Currently we only support OCI layouts, not tar archives. This
	//        should be fixed once the CAS engine PR is merged into
	//        image-tools. https://github.com/opencontainers/image-tools/pull/5
	//
	// FIXME: This also currently requires root privileges in order to extract
	//        something owned by root, which is a real shame. There are some
	//        PRs to fix this though. https://github.com/opencontainers/image-tools/pull/3
	//
	// FIXME: This also currently doesn't correctly extract a bundle (the
	//        modified/create time is not preserved after doing the
	//        extraction). I'm considering reimplementing it just so there are
	//        competing implementations of this extraction functionality.
	//           https://github.com/opencontainers/image-tools/issues/74
	if err := layer.UnpackManifest(context.Background(), engine, bundlePath, *manifest, &meta.MapOptions); err != nil {
		return errors.Wrap(err, "create runtime bundle")
	}

	// Create the mtree manifest.
	keywords := append(mtree.DefaultKeywords[:], "sha256digest")
	for idx, kw := range keywords {
		// We have to use tar_time because we're extracting from a tar archive.
		if kw == "time" {
			keywords[idx] = "tar_time"
		}
	}

	logrus.WithFields(logrus.Fields{
		"keywords": keywords,
		"mtree":    mtreePath,
	}).Debugf("umoci: generating mtree manifest")

	dh, err := mtree.Walk(fullRootfsPath, nil, keywords, meta.MapOptions.Rootless)
	if err != nil {
		return errors.Wrap(err, "generate mtree spec")
	}

	fh, err := os.OpenFile(mtreePath, os.O_EXCL|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return errors.Wrap(err, "open mtree")
	}
	defer fh.Close()

	logrus.Debugf("umoci: saving mtree manifest")

	if _, err := dh.WriteTo(fh); err != nil {
		return errors.Wrap(err, "write mtree")
	}

	logrus.WithFields(logrus.Fields{
		"version":     meta.Version,
		"from":        meta.From,
		"map_options": meta.MapOptions,
	}).Debugf("umoci: saving UmociMeta metadata")

	if err := WriteBundleMeta(bundlePath, meta); err != nil {
		return errors.Wrap(err, "write umoci.json metadata")
	}

	logrus.Debugf("umoci: unpacking complete")
	return nil
}
