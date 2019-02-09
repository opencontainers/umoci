/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2018 SUSE LLC.
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

	"github.com/apex/log"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/layer"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var rawUnpackCommand = uxRemap(cli.Command{
	Name:  "unpack",
	Usage: "unpacks a reference into a rootfs",
	ArgsUsage: `--image <image-path>[:<tag>] <rootfs>

Where "<image-path>" is the path to the OCI image, "<tag>" is the name of the
tagged image to unpack (if not specified, defaults to "latest") and "<rootfs>"
is the destination to unpack the image to.`,

	// unpack reads manifest information.
	Category: "image",

	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "keep-dirlinks",
			Usage: "don't clobber underlying symlinks to directories",
		},
	},

	Action: rawUnpack,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.Errorf("invalid number of positional arguments: expected <rootfs>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("rootfs path cannot be empty")
		}
		ctx.App.Metadata["rootfs"] = ctx.Args().First()
		return nil
	},
})

func rawUnpack(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	rootfsPath := ctx.App.Metadata["rootfs"].(string)

	var meta umoci.Meta
	meta.Version = umoci.MetaVersion

	// Parse map options.
	// We need to set mappings if we're in rootless mode.
	err := umoci.ParseIdmapOptions(&meta, ctx)
	if err != nil {
		return err
	}

	meta.MapOptions.KeepDirlinks = ctx.Bool("keep-dirlinks")

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

	if manifestBlob.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return errors.Wrap(fmt.Errorf("descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestBlob.Descriptor.MediaType), "invalid --image tag")
	}

	log.WithFields(log.Fields{
		"image":  imagePath,
		"rootfs": rootfsPath,
		"ref":    fromName,
	}).Debugf("umoci: unpacking OCI image")

	// Get the manifest.
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return errors.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.Descriptor.MediaType)
	}

	log.Warnf("unpacking rootfs ...")
	if err := layer.UnpackRootfs(context.Background(), engineExt, rootfsPath, manifest, &meta.MapOptions); err != nil {
		return errors.Wrap(err, "create rootfs")
	}
	log.Warnf("... done")

	log.Warnf("unpacked image rootfs: %s", rootfsPath)
	return nil
}
