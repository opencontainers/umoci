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
	"os"
	"time"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
	igen "github.com/opencontainers/umoci/oci/config/generate"
)

var rawAddLayerCommand = uxCompress(uxHistory(uxTag(cli.Command{
	Name:  "add-layer",
	Usage: "add a layer archive verbatim to an image",
	ArgsUsage: `--image <image-path>[:<tag>] <new-layer.tar>

Where "<image-path>" is the path to the OCI image, "<tag>" is the name of the
tagged image to modify (if not specified, defaults to "latest"),
"<new-layer.tar>" is the new layer to add (it must be uncompressed).

Note that using your own layer archives may result in strange behaviours (for
instance, you may need to use --keep-dirlink with umoci-unpack(1) in order to
avoid breaking certain entries).

At the moment, umoci-raw-add-layer(1) will only *append* layers to an image and
only supports uncompressed archives.`,

	// unpack reads manifest information.
	Category: "image",

	Action: rawAddLayer,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.New("invalid number of positional arguments: expected <newlayer.tar>")
		}
		if ctx.Args().First() == "" {
			return errors.New("<new-layer.tar> path cannot be empty")
		}
		ctx.App.Metadata["newlayer"] = ctx.Args().First()
		return nil
	},
})))

func rawAddLayer(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")
	fromName := mustFetchMeta[string](ctx, "--image-tag")
	newLayerPath := mustFetchMeta[string](ctx, "newlayer")

	var compressAlgo blobcompress.Algorithm
	if algo, ok := fetchMeta[blobcompress.Algorithm](ctx, "--compress"); ok {
		compressAlgo = algo
	}

	// Overide the from tag by default, otherwise use the one specified.
	tagName := fromName
	if overrideTagName, ok := fetchMeta[string](ctx, "--tag"); ok {
		tagName = overrideTagName
	}

	var meta umoci.Meta
	meta.Version = umoci.MetaVersion

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer funchelpers.VerifyClose(&Err, engine)

	fromDescriptorPaths, err := engineExt.ResolveReference(context.Background(), fromName)
	if err != nil {
		return fmt.Errorf("get descriptor: %w", err)
	}
	if len(fromDescriptorPaths) == 0 {
		return fmt.Errorf("tag not found: %s", fromName)
	}
	if len(fromDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return fmt.Errorf("tag is ambiguous: %s", fromName)
	}
	meta.From = fromDescriptorPaths[0]

	// Create the mutator.
	mutator, err := mutate.New(engine, meta.From)
	if err != nil {
		return fmt.Errorf("create mutator for base image: %w", err)
	}

	newLayer, err := os.Open(newLayerPath)
	if err != nil {
		return fmt.Errorf("open new layer archive: %w", err)
	}
	if fi, err := newLayer.Stat(); err != nil {
		return fmt.Errorf("stat new layer archive: %w", err)
	} else if fi.IsDir() {
		return errors.New("new layer archive is a directory")
	}
	// TODO: Verify that the layer is actually uncompressed.
	defer funchelpers.VerifyClose(&Err, newLayer)

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return fmt.Errorf("get image metadata: %w", err)
	}

	sourceDateEpoch, err := parseSourceDateEpoch()
	if err != nil {
		return err
	}

	var history *ispec.History
	if !ctx.Bool("no-history") {
		created := time.Now()
		if sourceDateEpoch != nil {
			created = *sourceDateEpoch
		}
		history = &ispec.History{
			Author:     imageMeta.Author,
			Comment:    "",
			Created:    &created,
			CreatedBy:  "umoci raw add-layer", // XXX: Should we append argv to this?
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

	if _, err := mutator.Add(context.Background(), ispec.MediaTypeImageLayer, newLayer, history, compressAlgo, nil); err != nil {
		return fmt.Errorf("add diff layer: %w", err)
	}

	newDescriptorPath, err := mutator.Commit(context.Background())
	if err != nil {
		return fmt.Errorf("commit mutated image: %w", err)
	}

	log.Infof("new image manifest created: %s->%s", newDescriptorPath.Root().Digest, newDescriptorPath.Descriptor().Digest)

	if err := engineExt.UpdateReference(context.Background(), tagName, newDescriptorPath.Root()); err != nil {
		return fmt.Errorf("add new tag: %w", err)
	}

	log.Infof("created new tag for image manifest: %s", tagName)
	return nil
}
