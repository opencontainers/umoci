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
	"errors"
	"fmt"
	"strings"

	"github.com/urfave/cli"

	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/casext/blobcompress"
)

// foreachSubcommand runs the given closure on every command and (recursively)
// every subcommand, allowing you to apply filters to all commands and
// subcommands.
func foreachSubcommand(cmds []cli.Command, fn func(*cli.Command)) {
	for idx, cmd := range cmds {
		fn(&cmds[idx])
		foreachSubcommand(cmd.Subcommands, fn)
	}
}

// uxHistory adds the full set of --history.* flags to the given cli.Command as
// well as adding relevant validation logic to the .Before of the command. The
// values will be stored in ctx.Metadata with the keys "--history.author",
// "--history.created", "--history.created_by", "--history.comment", with
// string values. If they are not set the value will be nil.
func uxHistory(cmd cli.Command) cli.Command {
	historyFlags := []cli.Flag{
		cli.BoolFlag{
			Name:  "no-history",
			Usage: "do not create a history entry",
		},
		cli.StringFlag{
			Name:  "history.author",
			Usage: "author value for the history entry",
		},
		cli.StringFlag{
			Name:  "history.comment",
			Usage: "comment for the history entry",
		},
		cli.StringFlag{
			Name:  "history.created",
			Usage: "created value for the history entry",
		},
		cli.StringFlag{
			Name:  "history.created_by",
			Usage: "created_by value for the history entry",
		},
	}
	cmd.Flags = append(cmd.Flags, historyFlags...)

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// --no-history is incompatible with other --history.* options.
		if ctx.Bool("no-history") {
			for _, flag := range historyFlags {
				if name := flag.GetName(); name == "no-history" {
					continue
				} else if ctx.IsSet(name) {
					return fmt.Errorf("--no-history and --%s may not be specified together", name)
				}
			}
		}

		// Include any old befores set.
		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

// uxCompress adds the --compress flag to the given cli.Command as well as
// adding relevant validation logic to the .Before of the command. The value
// will be stored in ctx.Metadata["--compress"] as a string (or nil if --tag
// was not specified).
func uxCompress(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, cli.StringFlag{
		Name:  "compress",
		Usage: "compression algorithm for newly created layer blobs",
		Value: "auto",
	})

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify compression algorithm value.
		if ctx.IsSet("compress") {
			compressType := ctx.String("compress")
			if compressType == "none" {
				compressType = ""
			}
			var layerCompressor blobcompress.Algorithm
			if compressType != "auto" {
				layerCompressor = blobcompress.GetAlgorithm(compressType)
				if layerCompressor == nil {
					return fmt.Errorf("invalid --compress: unknown layer compression type %q", ctx.String("compress"))
				}
			}
			ctx.App.Metadata["--compress"] = layerCompressor
		}

		// Include any old befores set.
		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

// uxTag adds a --tag flag to the given cli.Command as well as adding relevant
// validation logic to the .Before of the command. The value will be stored in
// ctx.Metadata["--tag"] as a string (or nil if --tag was not specified).
func uxTag(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, cli.StringFlag{
		Name:  "tag",
		Usage: "new tag name (if empty, overwrite --image tag)",
	})

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify tag value.
		if ctx.IsSet("tag") {
			tag := ctx.String("tag")
			if !casext.IsValidReferenceName(tag) {
				return fmt.Errorf("invalid --tag: tag contains invalid characters: %q", tag)
			}
			if tag == "" {
				return errors.New("invalid --tag: tag is empty")
			}
			ctx.App.Metadata["--tag"] = tag
		}

		// Include any old befores set.
		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

// uxImage adds an --image flag to the given cli.Command as well as adding
// relevant validation logic to the .Before of the command. The values (image,
// tag) will be stored in ctx.Metadata["--image-path"] and
// ctx.Metadata["--image-tag"] as strings (both will be nil if --image is not
// specified).
func uxImage(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, cli.StringFlag{
		Name:  "image",
		Usage: "OCI image URI of the form 'path[:tag]'",
	})

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify and parse --image.
		if ctx.IsSet("image") {
			image := ctx.String("image")

			var dir, tag string
			sep := strings.Index(image, ":")
			if sep == -1 {
				dir = image
				tag = "latest"
			} else {
				dir = image[:sep]
				tag = image[sep+1:]
			}

			// Verify directory value.
			if dir == "" {
				return errors.New("invalid --image: path is empty")
			}

			// Verify tag value.
			if !casext.IsValidReferenceName(tag) {
				return fmt.Errorf("invalid --image: tag contains invalid characters: %q", tag)
			}
			if tag == "" {
				return errors.New("invalid --image: tag is empty")
			}

			ctx.App.Metadata["--image-path"] = dir
			ctx.App.Metadata["--image-tag"] = tag
		}

		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

// uxLayout adds an --layout flag to the given cli.Command as well as adding
// relevant validation logic to the .Before of the command. The value is stored
// in ctx.App.Metadata["--image-path"] as a string (or nil --layout was not set).
func uxLayout(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, cli.StringFlag{
		Name:  "layout",
		Usage: "path to an OCI image layout",
	})

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify and parse --layout.
		if ctx.IsSet("layout") {
			layout := ctx.String("layout")

			// Verify directory value.
			if strings.Contains(layout, ":") {
				return fmt.Errorf("invalid --layout: path contains ':' character: %q", layout)
			}
			if layout == "" {
				return errors.New("invalid --layout: path is empty")
			}

			ctx.App.Metadata["--image-path"] = layout
		}

		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

func uxRemap(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, []cli.Flag{
		cli.StringSliceFlag{
			Name:  "uid-map",
			Usage: "specifies a uid mapping to use (container:host:size)",
		},
		cli.StringSliceFlag{
			Name:  "gid-map",
			Usage: "specifies a gid mapping to use (container:host:size)",
		},
		cli.BoolFlag{
			Name:  "rootless",
			Usage: "enable rootless command support",
		},
	}...)

	return cmd
}

// fetchMeta returns the requested metadata from the current [cli.Context] as
// the requested type. fetchMeta will panic if the type parameter T does not
// match the actual type of the metadata entry.
func fetchMeta[T any](ctx *cli.Context, metaName string) (T, bool) {
	val, ok := ctx.App.Metadata[metaName]
	if !ok || val == nil {
		return *new(T), false
	}
	realVal, ok := val.(T)
	if !ok {
		// this is a programmer error
		panic(fmt.Sprintf("umoci cli internal error: fetching cli metadata %s with wrong type %T", metaName, *new(T)))
	}
	return realVal, true
}

// mustFetchMeta is like fetchMeta except that it will also panic if the
// metadata is not present in the current [cli.Context].
func mustFetchMeta[T any](ctx *cli.Context, metaName string) T {
	val, ok := fetchMeta[T](ctx, metaName)
	if !ok {
		panic("umoci cli internal error: required cli metadata " + metaName + " missing")
	}
	return val
}
