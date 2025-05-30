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
	"encoding/json"
	"errors"
	"fmt"
	"os"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
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

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 0 {
			return errors.New("invalid number of positional arguments: expected none")
		}
		return nil
	},

	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "json",
			Usage: "output the stat information as a JSON encoded blob",
		},
	},

	Action: stat,
}

func stat(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")
	tagName := mustFetchMeta[string](ctx, "--image-tag")

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return fmt.Errorf("open CAS: %w", err)
	}
	engineExt := casext.NewEngine(engine)
	defer funchelpers.VerifyClose(&Err, engine)

	manifestDescriptorPaths, err := engineExt.ResolveReference(context.Background(), tagName)
	if err != nil {
		return fmt.Errorf("get descriptor: %w", err)
	}
	if len(manifestDescriptorPaths) == 0 {
		return fmt.Errorf("tag not found: %s", tagName)
	}
	if len(manifestDescriptorPaths) != 1 {
		// TODO: Handle this more nicely.
		return fmt.Errorf("tag is ambiguous: %s", tagName)
	}
	manifestDescriptor := manifestDescriptorPaths[0].Descriptor()

	// FIXME: Implement support for manifest lists.
	if manifestDescriptor.MediaType != ispec.MediaTypeImageManifest {
		return fmt.Errorf("invalid saved from descriptor: descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestDescriptor.MediaType)
	}

	// Get stat information.
	ms, err := umoci.Stat(context.Background(), engineExt, manifestDescriptor)
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	// Output the stat information.
	if ctx.Bool("json") {
		// Use JSON.
		if err := json.NewEncoder(os.Stdout).Encode(ms); err != nil {
			return fmt.Errorf("encoding stat: %w", err)
		}
	} else {
		if err := ms.Format(os.Stdout); err != nil {
			return fmt.Errorf("format stat: %w", err)
		}
	}

	return nil
}
