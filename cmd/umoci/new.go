/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017, 2018 SUSE LLC.
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
	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var newCommand = cli.Command{
	Name:  "new",
	Usage: "creates a blank tagged OCI image",
	ArgsUsage: `--image <image-path>:<new-tag>

Where "<image-path>" is the path to the OCI image, and "<new-tag>" is the name
of the tag for the empty manifest.

Once you create a new image with umoci-new(1) you can directly use the image
with umoci-unpack(1), umoci-repack(1), and umoci-config(1) to modify the new
manifest as you see fit. This allows you to create entirely new images without
needing a base image to start from.`,

	// new modifies an image layout.
	Category: "image",

	Action: newImage,
}

func newImage(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	tagName := ctx.App.Metadata["--image-tag"].(string)

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	return umoci.NewImage(engineExt, tagName)
}
