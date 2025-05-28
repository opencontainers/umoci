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

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
)

var rawConfigCommand = uxRemap(cli.Command{
	Name:    "runtime-config",
	Aliases: []string{"config"},
	Usage:   "generates an OCI runtime configuration for an image",
	ArgsUsage: `--image <image-path>[:<tag>] [--rootfs <rootfs>] <config.json>

Where "<image-path>" is the path to the OCI image, "<tag>" is the name of the
tagged image to unpack (if not specified, defaults to "latest"), "<rootfs>" is
a rootfs to use as a supplementary "source of truth" for certain generation
operations and "<config.json>" is the destination to write the runtime
configuration to.

Note that the results of this may not agree with umoci-unpack(1) because the
--rootfs flag affects how certain properties are interpreted.`,

	// unpack reads manifest information.
	Category: "image",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "rootfs",
			Usage: "path to secondary source of truth (root filesystem)",
		},
	},

	Action: rawConfig,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.New("invalid number of positional arguments: expected <config.json>")
		}
		if ctx.Args().First() == "" {
			return errors.New("config.json path cannot be empty")
		}
		ctx.App.Metadata["config"] = ctx.Args().First()
		return nil
	},
})

func rawConfig(ctx *cli.Context) (Err error) {
	imagePath := mustFetchMeta[string](ctx, "--image-path")
	fromName := mustFetchMeta[string](ctx, "--image-tag")
	configPath := mustFetchMeta[string](ctx, "config")

	var meta umoci.Meta
	meta.Version = umoci.MetaVersion

	// Parse and set up the mapping options.
	err := umoci.ParseIdmapOptions(&meta, ctx)
	if err != nil {
		return err
	}

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

	manifestBlob, err := engineExt.FromDescriptor(context.Background(), meta.From.Descriptor())
	if err != nil {
		return fmt.Errorf("get manifest: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, manifestBlob)

	if manifestBlob.Descriptor.MediaType != ispec.MediaTypeImageManifest {
		return fmt.Errorf("invalid --image tag: descriptor does not point to ispec.MediaTypeImageManifest: not implemented: %s", manifestBlob.Descriptor.MediaType)
	}

	// Get the manifest.
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return fmt.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.Descriptor.MediaType)
	}

	// Generate the configuration.
	configFile, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("opening config path: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, configFile)

	// Write out the generated config.
	log.Info("generating config.json")
	if err := layer.UnpackRuntimeJSON(context.Background(), engineExt, configFile, ctx.String("rootfs"), manifest, &meta.MapOptions); err != nil {
		return fmt.Errorf("generate config: %w", err)
	}
	return nil
}
