/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
 * Copyright (C) 2018 Cisco Systems
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
	"time"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/mutate"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	igen "github.com/opencontainers/umoci/oci/config/generate"
	"github.com/opencontainers/umoci/oci/layer"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var insertCommand = uxRemap(uxHistory(uxTag(cli.Command{
	Name:  "insert",
	Usage: "insert content into an OCI image",
	ArgsUsage: `--image <image-path>[:<tag>] [--opaque] <source> <target>
                                  --image <image-path>[:<tag>] [--whiteout] <target>

Where "<image-path>" is the path to the OCI image, and "<tag>" is the name of
the tag that the content wil be inserted into (if not specified, defaults to
"latest").

The path at "<source>" is added to the image with the given "<target>" name.
If "--whiteout" is specified, rather than inserting content into the image, a
removal entry for "<target>" is inserted instead.

If "--opaque" is specified then any paths below "<target>" (assuming it is a
directory) from previous layers will no longer be present. Only the contents
inserted by this command will be visible. This can be used to replace an entire
directory, while the default behaviour merges the old contents with the new.

Note that this command works by creating a new layer, so this should not be
used to remove (or replace) secrets from an already-built image. See
umoci-config(1) and --config.volume for how to achieve this correctly.

Some examples:
	umoci insert --image oci:foo mybinary /usr/bin/mybinary
	umoci insert --image oci:foo myconfigdir /etc/myconfigdir
	umoci insert --image oci:foo --opaque myoptdir /opt
	umoci insert --image oci:foo --whiteout /some/old/dir
`,

	Category: "image",

	Action: insert,

	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "whiteout",
			Usage: "insert a 'removal entry' for the given path",
		},
		cli.BoolFlag{
			Name:  "opaque",
			Usage: "mask any previous entries in the target directory",
		},
	},

	Before: func(ctx *cli.Context) error {
		// This command is quite weird because we need to support two different
		// positional-argument numbers. Awesome.
		numArgs := 2
		if ctx.IsSet("whiteout") {
			numArgs = 1
		}
		if ctx.NArg() != numArgs {
			return errors.Errorf("invalid number of positional arguments: expected %d", numArgs)
		}
		for idx, args := range ctx.Args() {
			if args == "" {
				return errors.Errorf("invalid positional argument %d: arguments cannot be empty", idx)
			}
		}

		// Figure out the arguments.
		var sourcePath, targetPath string
		targetPath = ctx.Args()[0]
		if !ctx.IsSet("whiteout") {
			sourcePath = targetPath
			targetPath = ctx.Args()[1]
		}

		ctx.App.Metadata["--source-path"] = sourcePath
		ctx.App.Metadata["--target-path"] = targetPath
		return nil
	},
})))

func insert(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	sourcePath := ctx.App.Metadata["--source-path"].(string)
	targetPath := ctx.App.Metadata["--target-path"].(string)

	// By default we clobber the old tag.
	tagName := fromName
	if val, ok := ctx.App.Metadata["--tag"]; ok {
		tagName = val.(string)
	}

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	descriptorPaths, err := engineExt.ResolveReference(context.Background(), fromName)
	if err != nil {
		return errors.Wrap(err, "get descriptor")
	}
	if len(descriptorPaths) == 0 {
		return errors.Errorf("tag not found: %s", fromName)
	}
	if len(descriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", fromName)
	}

	// Create the mutator.
	mutator, err := mutate.New(engine, descriptorPaths[0])
	if err != nil {
		return errors.Wrap(err, "create mutator for base image")
	}

	var meta umoci.Meta
	meta.Version = umoci.MetaVersion

	// Parse and set up the mapping options.
	err = umoci.ParseIdmapOptions(&meta, ctx)
	if err != nil {
		return err
	}

	packOptions := layer.RepackOptions{MapOptions: meta.MapOptions}
	reader := layer.GenerateInsertLayer(sourcePath, targetPath, ctx.IsSet("opaque"), &packOptions)
	defer reader.Close()

	var history *ispec.History
	if !ctx.Bool("no-history") {
		created := time.Now()
		history = &ispec.History{
			Comment:    "",
			Created:    &created,
			CreatedBy:  "umoci insert", // XXX: Should we append argv to this?
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
	if _, err := mutator.Add(context.Background(), ispec.MediaTypeImageLayer, reader, history, mutate.GzipCompressor); err != nil {
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
	log.Infof("updated tag for image manifest: %s", tagName)
	return nil
}
