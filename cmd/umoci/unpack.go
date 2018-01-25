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

package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	"github.com/openSUSE/umoci/pkg/fseval"
	"github.com/openSUSE/umoci/pkg/idtools"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
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
			Usage: "specifies a uid mapping to use when repacking (container:host:size)",
		},
		cli.StringSliceFlag{
			Name:  "gid-map",
			Usage: "specifies a gid mapping to use when repacking (container:host:size)",
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

func unpack(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	bundlePath := ctx.App.Metadata["bundle"].(string)

	var meta umoci.UmociMeta
	meta.Version = umoci.UmociMetaVersion

	// Parse map options.
	// We need to set mappings if we're in rootless mode.
	meta.MapOptions.Rootless = ctx.Bool("rootless")
	if meta.MapOptions.Rootless {
		if !ctx.IsSet("uid-map") {
			ctx.Set("uid-map", fmt.Sprintf("0:%d:1", os.Geteuid()))
		}
		if !ctx.IsSet("gid-map") {
			ctx.Set("gid-map", fmt.Sprintf("0:%d:1", os.Getegid()))
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

	log.WithFields(log.Fields{
		"map.uid": meta.MapOptions.UIDMappings,
		"map.gid": meta.MapOptions.GIDMappings,
	}).Debugf("parsed mappings")

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	fromDescriptorPaths, err := engineExt.ResolveReference(context.Background(), fromName)
	if err != nil {
		return errors.Wrap(err, "get descriptor")
	}
	if len(fromDescriptorPaths) == 0 {
		return errors.Errorf("tag is not found: %s", fromName)
	}
	if len(fromDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", fromName)
	}
	meta.From = fromDescriptorPaths[0]

	manifestBlob, err := engineExt.FromDescriptor(context.Background(), meta.From.Descriptor())
	if err != nil {
		return errors.Wrap(err, "get manifest")
	}
	defer manifestBlob.Close()

	if manifestBlob.MediaType != ispec.MediaTypeImageManifest {
		return errors.Wrap(fmt.Errorf("descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestBlob.MediaType), "invalid --image tag")
	}

	mtreeName := strings.Replace(meta.From.Descriptor().Digest.String(), ":", "_", 1)
	log.WithFields(log.Fields{
		"image":  imagePath,
		"bundle": bundlePath,
		"ref":    fromName,
		"rootfs": layer.RootfsName,
	}).Debugf("umoci: unpacking OCI image")

	// Get the manifest.
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.MediaType)
	}

	// Unpack the runtime bundle.
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return errors.Wrap(err, "create bundle path")
	}
	// XXX: We should probably defer os.RemoveAll(bundlePath).

	// FIXME: Currently we only support OCI layouts, not tar archives. This
	//        should be fixed once the CAS engine PR is merged into
	//        image-tools. https://github.com/opencontainers/image-tools/pull/5
	log.Info("unpacking bundle ...")
	if err := layer.UnpackManifest(context.Background(), engineExt, bundlePath, manifest, &meta.MapOptions); err != nil {
		return errors.Wrap(err, "create runtime bundle")
	}
	log.Info("... done")

	fsEval := fseval.DefaultFsEval
	if meta.MapOptions.Rootless {
		fsEval = fseval.RootlessFsEval
	}

	if err := umoci.GenerateBundleManifest(mtreeName, bundlePath, fsEval); err != nil {
		return errors.Wrap(err, "write mtree")
	}

	log.WithFields(log.Fields{
		"version":     meta.Version,
		"from":        meta.From,
		"map_options": meta.MapOptions,
	}).Debugf("umoci: saving UmociMeta metadata")

	if err := umoci.WriteBundleMeta(bundlePath, meta); err != nil {
		return errors.Wrap(err, "write umoci.json metadata")
	}

	log.Infof("unpacked image bundle: %s", bundlePath)
	return nil
}
