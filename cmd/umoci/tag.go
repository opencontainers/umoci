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

	"github.com/apex/log"
	"github.com/urfave/cli"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
)

var tagAddCommand = cli.Command{
	Name:  "tag",
	Usage: "creates a new tag in an OCI image",
	ArgsUsage: `--image <image-path>[:<tag>] <new-tag>

Where "<image-path>" is the path to the OCI image, "<tag>" is the old name of
the tag and "<new-tag>" is the new name of the tag.`,

	// tag modifies an image layout.
	Category: "image",

	Action: tagAdd,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.New("invalid number of positional arguments: expected <new-tag>")
		}
		if ctx.Args().First() == "" {
			return errors.New("new tag cannot be empty")
		}
		if !casext.IsValidReferenceName(ctx.Args().First()) {
			return errors.New("new tag is an invalid reference")
		}
		ctx.App.Metadata["new-tag"] = ctx.Args().First()
		return nil
	},
}

func tagAdd(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")
	fromName := mustFetchMeta[string](ctx, "--image-tag")
	tagName := mustFetchMeta[string](ctx, "new-tag")

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer funchelpers.VerifyClose(&Err, engine)

	// Get original descriptor.
	descriptorPaths, err := engineExt.ResolveReference(context.Background(), fromName)
	if err != nil {
		return fmt.Errorf("get descriptor: %w", err)
	}
	if len(descriptorPaths) == 0 {
		return fmt.Errorf("tag not found: %s", fromName)
	}
	if len(descriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return fmt.Errorf("tag is ambiguous: %s", fromName)
	}
	descriptor := descriptorPaths[0].Descriptor()

	// Add it.
	if err := engineExt.UpdateReference(context.Background(), tagName, descriptor); err != nil {
		return fmt.Errorf("put reference: %w", err)
	}

	log.Infof("created new tag: %q -> %q", tagName, fromName)
	return nil
}

var tagRemoveCommand = cli.Command{
	Name:    "remove",
	Aliases: []string{"rm"},
	Usage:   "removes a tag from an OCI image",
	ArgsUsage: `--image <image-path>[:<tag>]


Where "<image-path>" is the path to the OCI image, "<tag>" is the name of the
tag to remove.`,

	// tag modifies an image layout.
	Category: "image",

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 0 {
			return errors.New("invalid number of positional arguments: expected none")
		}
		return nil
	},

	Action: tagRemove,
}

func tagRemove(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")
	tagName := mustFetchMeta[string](ctx, "--image-tag")

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer funchelpers.VerifyClose(&Err, engine)

	// Remove it.
	if err := engineExt.DeleteReference(context.Background(), tagName); err != nil {
		return fmt.Errorf("delete reference: %w", err)
	}

	log.Infof("removed tag: %s", tagName)
	return nil
}

var tagListCommand = cli.Command{
	Name:    "list",
	Aliases: []string{"ls"},
	Usage:   "lists the set of tags in an OCI layout",
	ArgsUsage: `--layout <image-path>

Where "<image-path>" is the path to the OCI layout.

Gives the full list of tags in an OCI layout, with each tag name on a single
line. See umoci-stat(1) to get more information about each tagged image.`,

	// tag modifies an image layout.
	Category: "layout",

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 0 {
			return errors.New("invalid number of positional arguments: expected none")
		}
		return nil
	},

	Action: tagList,
}

func tagList(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer funchelpers.VerifyClose(&Err, engine)

	names, err := engineExt.ListReferences(context.Background())
	if err != nil {
		return fmt.Errorf("list references: %w", err)
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
