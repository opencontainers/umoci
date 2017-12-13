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

	"github.com/apex/log"
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
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
			return errors.Errorf("invalid number of positional arguments: expected <new-tag>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("new tag cannot be empty")
		}
		if !refRegexp.MatchString(ctx.Args().First()) {
			return errors.Errorf("new tag is an invalid reference")
		}
		ctx.App.Metadata["new-tag"] = ctx.Args().First()
		return nil
	},
}

func tagAdd(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	tagName := ctx.App.Metadata["new-tag"].(string)

	layout, err := umoci.OpenLayout(imagePath)
	if err != nil {
		return errors.Wrap(err, "open layout")
	}
	defer layout.Close()

	if err := layout.Tag(fromName, tagName); err != nil {
		return errors.Wrap(err, "create tag")
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

	Action: tagRemove,
}

func tagRemove(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	tagName := ctx.App.Metadata["--image-tag"].(string)

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	// Remove it.
	if err := engineExt.DeleteReference(context.Background(), tagName); err != nil {
		return errors.Wrap(err, "delete reference")
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

	Action: tagList,
}

func tagList(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)

	layout, err := umoci.OpenLayout(imagePath)
	if err != nil {
		return errors.Wrap(err, "open layout")
	}
	defer layout.Close()

	names, err := layout.ListTags()
	if err != nil {
		return errors.Wrap(err, "list tags")
	}

	for _, name := range names {
		fmt.Println(name)
	}
	return nil
}
