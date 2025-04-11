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

package main

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
	igen "github.com/opencontainers/umoci/oci/config/generate"
	"github.com/opencontainers/umoci/pkg/mtreefilter"
	"github.com/urfave/cli"
)

var repackCommand = uxCompress(uxHistory(cli.Command{
	Name:  "repack",
	Usage: "repacks an OCI runtime bundle into a reference",
	ArgsUsage: `--image <image-path>[:<new-tag>] <bundle>

Where "<image-path>" is the path to the OCI image, "<new-tag>" is the name of
the tag that the new image will be saved as (if not specified, defaults to
"latest"), and "<bundle>" is the bundle from which to generate the required
layers.

The "<image-path>" MUST be the same image that was used to create "<bundle>"
(using umoci-unpack(1)). Otherwise umoci will not be able to modify the
original manifest to add the diff layer.

All uid-map and gid-map settings are automatically loaded from the bundle
metadata (which is generated by umoci-unpack(1)) so if you unpacked an image
using a particular mapping then the same mapping will be used to generate the
new layer.

It should be noted that this is not the same as oci-create-layer because it
uses go-mtree to create diff layers from runtime bundles unpacked with
umoci-unpack(1). In addition, it modifies the image so that all of the relevant
manifest and configuration information uses the new diff atop the old manifest.`,

	// repack creates a new image, with a given tag.
	Category: "image",

	Flags: []cli.Flag{
		cli.StringSliceFlag{
			Name:  "mask-path",
			Usage: "set of path prefixes in which deltas will be ignored when generating new layers",
		},
		cli.BoolFlag{
			Name:  "no-mask-volumes",
			Usage: "do not add the Config.Volumes of the image to the set of masked paths",
		},
		cli.BoolFlag{
			Name:  "refresh-bundle",
			Usage: "update the bundle metadata to reflect the packed rootfs",
		},
	},

	Action: repack,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.New("invalid number of positional arguments: expected <bundle>")
		}
		if ctx.Args().First() == "" {
			return errors.New("bundle path cannot be empty")
		}
		ctx.App.Metadata["bundle"] = ctx.Args().First()
		return nil
	},
}))

func repack(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	tagName := ctx.App.Metadata["--image-tag"].(string)
	bundlePath := ctx.App.Metadata["bundle"].(string)

	var compressAlgo blobcompress.Algorithm
	if algo, ok := ctx.App.Metadata["--compress"].(blobcompress.Algorithm); ok {
		compressAlgo = algo
	}

	// Read the metadata first.
	meta, err := umoci.ReadBundleMeta(bundlePath)
	if err != nil {
		return fmt.Errorf("read umoci.json metadata: %w", err)
	}

	log.WithFields(log.Fields{
		"version":     meta.Version,
		"from":        meta.From,
		"map_options": meta.MapOptions,
	}).Debugf("umoci: loaded Meta metadata")

	if meta.From.Descriptor().MediaType != ispec.MediaTypeImageManifest {
		return fmt.Errorf("invalid saved from descriptor: descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", meta.From.Descriptor().MediaType)
	}

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	// Create the mutator.
	mutator, err := mutate.New(engineExt, meta.From)
	if err != nil {
		return fmt.Errorf("create mutator for base image: %w", err)
	}

	// We need to mask config.Volumes.
	config, err := mutator.Config(context.Background())
	if err != nil {
		return fmt.Errorf("get config: %w", err)
	}

	maskedPaths := ctx.StringSlice("mask-path")
	if !ctx.Bool("no-mask-volumes") {
		for v := range config.Config.Volumes {
			maskedPaths = append(maskedPaths, v)
		}
	}

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return fmt.Errorf("get image metadata: %w", err)
	}

	var history *ispec.History
	if !ctx.Bool("no-history") {
		created := time.Now()
		history = &ispec.History{
			Author:     imageMeta.Author,
			Comment:    "",
			Created:    &created,
			CreatedBy:  "umoci repack", // XXX: Should we append argv to this?
			EmptyLayer: false,
		}

		if ctx.IsSet("history.author") {
			history.Author = ctx.String("history.author")
		}
		if ctx.IsSet("history.comment") {
			history.Comment = ctx.String("history.comment")
		}
		if ctx.IsSet("history.created") {
			created, err := time.Parse(igen.ISO8601, ctx.String("history.created"))
			if err != nil {
				return fmt.Errorf("parsing --history.created: %w", err)
			}
			history.Created = &created
		}
		if ctx.IsSet("history.created_by") {
			history.CreatedBy = ctx.String("history.created_by")
		}
	}

	filters := []mtreefilter.FilterFunc{
		mtreefilter.MaskFilter(maskedPaths),
	}

	return umoci.Repack(engineExt, tagName, bundlePath, meta, history, filters, ctx.Bool("refresh-bundle"), mutator, compressAlgo)
}
