/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2019 SUSE LLC.
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
	"os"
	"strings"

	"github.com/apex/log"
	logcli "github.com/apex/log/handlers/cli"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

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
				return errors.Wrap(fmt.Errorf("path is empty"), "invalid --image")
			}

			// Verify tag value.
			if !casext.IsValidReferenceName(tag) {
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

func flattenCommands(cmds []cli.Command) []*cli.Command {
	var flatten []*cli.Command
	for idx, cmd := range cmds {
		flatten = append(flatten, &cmds[idx])
		flatten = append(flatten, flattenCommands(cmd.Subcommands)...)
	}
	return flatten
}

func main() {
	app := cli.NewApp()
	app.Name = "umoci-ociv2"
	app.Usage = `experimental umoci support tool for OCIv2`
	app.Authors = []cli.Author{
		{
			Name:  "Aleksa Sarai",
			Email: "asarai@suse.com",
		},
	}

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "alias for --log=info",
		},
		cli.StringFlag{
			Name:  "log",
			Usage: "set the log level (debug, info, [warn], error, fatal)",
			Value: "warn",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		log.SetHandler(logcli.New(os.Stderr))

		if ctx.GlobalBool("verbose") {
			if ctx.GlobalIsSet("log") {
				return errors.New("--log=* and --verbose are mutually exclusive")
			}
			if err := ctx.GlobalSet("log", "info"); err != nil {
				// Should _never_ be reached.
				return errors.Wrap(err, "[internal error] failure auto-setting --log=info")
			}
		}

		level, err := log.ParseLevel(ctx.GlobalString("log"))
		if err != nil {
			return errors.Wrap(err, "parsing log level")
		}
		log.SetLevel(level)
		return nil
	}

	app.Commands = []cli.Command{
		snapshotCommand,
	}

	app.Metadata = map[string]interface{}{}

	// In order to make the uxXyz wrappers not too cumbersome we automatically
	// add them to images with categories set to categoryImage or
	// categoryLayout. Monkey patching was never this neat.
	for _, cmd := range flattenCommands(app.Commands) {
		switch cmd.Category {
		case "image":
			oldBefore := cmd.Before
			cmd.Before = func(ctx *cli.Context) error {
				if _, ok := ctx.App.Metadata["--image-path"]; !ok {
					return errors.Errorf("missing mandatory argument: --image")
				}
				if _, ok := ctx.App.Metadata["--image-tag"]; !ok {
					return errors.Errorf("missing mandatory argument: --image")
				}
				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}
			*cmd = uxImage(*cmd)
		}
	}

	// Actually run command.
	if err := app.Run(os.Args); err != nil {
		log.Fatalf("%v", err)
		log.Debugf("%+v", err)
	}
}
