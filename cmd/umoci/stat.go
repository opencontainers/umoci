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
	"encoding/json"
	"fmt"
	"os"

	"github.com/openSUSE/umoci"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var statCommand = cli.Command{
	Name:  "stat",
	Usage: "displays status information of an image manifest",
	ArgsUsage: `--image <image-path>[:<tag>]

Where "<image-path>" is the path to the OCI image, and "<tag>" is the name of
the tagged image to stat.

WARNING: Do not depend on the output of this tool unless you're using --json.
The intention of the default formatting of this tool is that it is easy for
humans to read, and might change in future versions.`,

	// stat gives information about a manifest.
	Category: "image",

	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "json",
			Usage: "output the stat information as a JSON encoded blob",
		},
	},

	Action: stat,
}

func stat(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	tagName := ctx.App.Metadata["--image-tag"].(string)

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	manifestDescriptorPaths, err := engineExt.ResolveReference(context.Background(), tagName)
	if err != nil {
		return errors.Wrap(err, "get descriptor")
	}
	if len(manifestDescriptorPaths) == 0 {
		return errors.Errorf("tag not found: %s", tagName)
	}
	if len(manifestDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", tagName)
	}
	manifestDescriptor := manifestDescriptorPaths[0].Descriptor()

	// FIXME: Implement support for manifest lists.
	if manifestDescriptor.MediaType != ispec.MediaTypeImageManifest {
		return errors.Wrap(fmt.Errorf("descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestDescriptor.MediaType), "invalid saved from descriptor")
	}

	// Get stat information.
	ms, err := umoci.Stat(context.Background(), engineExt, manifestDescriptor)
	if err != nil {
		return errors.Wrap(err, "stat")
	}

	// Output the stat information.
	if ctx.Bool("json") {
		// Use JSON.
		if err := json.NewEncoder(os.Stdout).Encode(ms); err != nil {
			return errors.Wrap(err, "encoding stat")
		}
	} else {
		if err := ms.Format(os.Stdout); err != nil {
			return errors.Wrap(err, "format stat")
		}
	}

	return nil
}
