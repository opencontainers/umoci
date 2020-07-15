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

package main

import (
	"os"
	"time"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	igen "github.com/opencontainers/umoci/oci/config/generate"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var rawAddLayerCommand = uxHistory(uxTag(cli.Command{
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
			return errors.Errorf("invalid number of positional arguments: expected <newlayer.tar>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("<new-layer.tar> path cannot be empty")
		}
		ctx.App.Metadata["newlayer"] = ctx.Args().First()
		return nil
	},
}))

func rawAddLayer(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	newLayerPath := ctx.App.Metadata["newlayer"].(string)

	// Overide the from tag by default, otherwise use the one specified.
	tagName := fromName
	if overrideTagName, ok := ctx.App.Metadata["--tag"]; ok {
		tagName = overrideTagName.(string)
	}

	var meta umoci.Meta
	meta.Version = umoci.MetaVersion

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
		return errors.Errorf("tag not found: %s", fromName)
	}
	if len(fromDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", fromName)
	}
	meta.From = fromDescriptorPaths[0]

	// Create the mutator.
	mutator, err := mutate.New(engine, meta.From)
	if err != nil {
		return errors.Wrap(err, "create mutator for base image")
	}

	newLayer, err := os.Open(newLayerPath)
	if err != nil {
		return errors.Wrap(err, "open new layer archive")
	}
	if fi, err := newLayer.Stat(); err != nil {
		return errors.Wrap(err, "stat new layer archive")
	} else if fi.IsDir() {
		return errors.Errorf("new layer archive is a directory")
	}
	// TODO: Verify that the layer is actually uncompressed.
	defer newLayer.Close()

	imageMeta, err := mutator.Meta(context.Background())
	if err != nil {
		return errors.Wrap(err, "get image metadata")
	}

	var history *ispec.History
	if !ctx.Bool("no-history") {
		created := time.Now()
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
				return errors.Wrap(err, "parsing --history.created")
			}
			history.Created = &created
		}
		if ctx.IsSet("history.created_by") {
			history.CreatedBy = ctx.String("history.created_by")
		}
	}

	// TODO: We should add a flag to allow for a new layer to be made
	//       non-distributable.
	if _, err := mutator.Add(context.Background(), newLayer, history); err != nil {
		return errors.Wrap(err, "add diff layer")
	}

	newDescriptorPath, err := mutator.Commit(context.Background())
	if err != nil {
		return errors.Wrap(err, "commit mutated image")
	}

	log.Infof("new image manifest created: %s->%s", newDescriptorPath.Root().Digest, newDescriptorPath.Descriptor().Digest)

	if err := engineExt.UpdateReference(context.Background(), tagName, newDescriptorPath.Root()); err != nil {
		return errors.Wrap(err, "add new tag")
	}

	log.Infof("created new tag for image manifest: %s", tagName)
	return nil
}
