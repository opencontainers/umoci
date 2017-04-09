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
	"regexp"
	"strings"

	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// refRegexp defines the regexp that a given OCI tag must obey.
var refRegexp = regexp.MustCompile(`^([A-Za-z0-9._-]+)+$`)

func flattenCommands(cmds []cli.Command) []*cli.Command {
	var flatten []*cli.Command
	for idx, cmd := range cmds {
		flatten = append(flatten, &cmds[idx])
		flatten = append(flatten, flattenCommands(cmd.Subcommands)...)
	}
	return flatten
}

// uxHistory adds the full set of --history.* flags to the given cli.Command as
// well as adding relevant validation logic to the .Before of the command. The
// values will be stored in ctx.Metadata with the keys "--history.author",
// "--history.created", "--history.created_by", "--history.comment", with
// string values. If they are not set the value will be nil.
func uxHistory(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, []cli.Flag{
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
	}...)

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify --history.author.
		if ctx.IsSet("history.author") {
			ctx.App.Metadata["--history.author"] = ctx.String("history.author")
		}
		// Verify --history.comment.
		if ctx.IsSet("history.comment") {
			ctx.App.Metadata["--history.comment"] = ctx.String("history.comment")
		}
		// Verify --history.created.
		if ctx.IsSet("history.created") {
			ctx.App.Metadata["--history.created"] = ctx.String("history.created")
		}
		// Verify --history.created_by.
		if ctx.IsSet("history.created_by") {
			ctx.App.Metadata["--history.created_by"] = ctx.String("history.created_by")
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
		Usage: "tag name",
	})

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		// Verify tag value.
		if ctx.IsSet("tag") {
			tag := ctx.String("tag")
			if !refRegexp.MatchString(tag) {
				return errors.Wrap(fmt.Errorf("tag contains invalid characters: '%s'", tag), "invalid --tag")
			}
			if tag == "" {
				return errors.Wrap(fmt.Errorf("tag is empty"), "invalid --tag")
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
			sep := strings.LastIndex(image, ":")
			if sep == -1 {
				dir = image
				tag = "latest"
			} else {
				dir = image[:sep]
				tag = image[sep+1:]
			}

			// Verify directory value.
			if strings.Contains(dir, ":") {
				return errors.Wrap(fmt.Errorf("path contains ':' character: '%s'", dir), "invalid --image")
			}
			if dir == "" {
				return errors.Wrap(fmt.Errorf("path is empty"), "invalid --image")
			}

			// Verify tag value.
			if !refRegexp.MatchString(tag) {
				return errors.Wrap(fmt.Errorf("tag contains invalid characters: '%s'", tag), "invalid --image")
			}
			if tag == "" {
				return errors.Wrap(fmt.Errorf("tag is empty"), "invalid --image")
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
				return errors.Wrap(fmt.Errorf("path contains ':' character: '%s'", layout), "invalid --layout")
			}
			if layout == "" {
				return errors.Wrap(fmt.Errorf("path is empty"), "invalid --layout")
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
