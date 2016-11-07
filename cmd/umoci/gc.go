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

	"github.com/cyphar/umoci/image/cas"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var gcCommand = cli.Command{
	Name:  "gc",
	Usage: "garbage-collects an OCI image's blobs",
	ArgsUsage: `--image <image-path>

Where "<image-path>" is the path to the OCI image.

This command will do a mark-and-sweep garbage collection of the provided OCI
image, only retaining blobs which can be reached by a descriptor path from the
root set of references. All other blobs will be removed.`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
	},

	Action: gc,
}

func gc(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.String("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	// Run the GC.
	return cas.GC(context.TODO(), engine)
}
