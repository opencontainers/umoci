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
	"os"
	"regexp"
	"strings"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// version is version ID for the source, read from VERSION in the source and
// populated on build by make.
var version = ""

// gitCommit is the commit hash that the binary was built from and will be
// populated on build by make.
var gitCommit = ""

// refRegexp defines the regexp that a given OCI tag must obey.
var refRegexp = regexp.MustCompile(`^([A-Za-z0-9._-]+)+$`)

const (
	usage = `umoci modifies Open Container images`
)

func main() {
	app := cli.NewApp()
	app.Name = "umoci"
	app.Usage = usage
	app.Authors = []cli.Author{
		{
			Name:  "Aleksa Sarai",
			Email: "asarai@suse.com",
		},
	}

	// Fill the version.
	v := "unknown"
	if version != "" {
		v = version
	}
	if gitCommit != "" {
		v = fmt.Sprintf("%s~git%s", v, gitCommit)
	}
	app.Version = v

	// FIXME: Should --image be a global option?
	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "set log level to debug",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool("debug") {
			logrus.SetLevel(logrus.DebugLevel)
		}
		return nil
	}

	app.Commands = []cli.Command{
		configCommand,
		unpackCommand,
		repackCommand,
		gcCommand,
		initCommand,
		newCommand,
		tagAddCommand,
		tagRemoveCommand,
		tagListCommand,
		statCommand,
	}

	app.Metadata = map[string]interface{}{}

	// In order to consolidate a lot of the --image and --layout handling, we
	// have to do some monkey-patching of commands. In particular, we set up
	// the --image and --layout flags (for image and layout category commands)
	// and then add parsing code to cmd.Before so that we can validate and
	// parse the required arguments. It's definitely not pretty, but it's the
	// best we can do without making them all global flags and then having odd
	// semantics.

	for idx, cmd := range app.Commands {
		var flag cli.Flag
		oldBefore := cmd.Before

		switch cmd.Category {
		case "image":
			// Does the command modify images (manifests)?
			flag = cli.StringFlag{
				Name:  "image",
				Usage: "OCI image URI of the form 'path[:tag]'",
			}

			// Add BeforeFunc that will verify code.
			cmd.Before = func(ctx *cli.Context) error {
				// Parse --image.
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
					return errors.Wrap(fmt.Errorf("directory contains ':' character: '%s'", dir), "invalid --image")
				}
				if dir == "" {
					return errors.Wrap(fmt.Errorf("directory is empty"), "invalid --image")
				}

				// Verify tag value.
				if !refRegexp.MatchString(tag) {
					return errors.Wrap(fmt.Errorf("tag contains invalid characters: '%s'", tag), "invalid --image")
				}
				if tag == "" {
					return errors.Wrap(fmt.Errorf("tag is empty"), "invalid --image")
				}

				ctx.App.Metadata["layout"] = dir
				ctx.App.Metadata["tag"] = tag

				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}

		case "layout":
			// Does the command modify an OCI image layout itself?
			flag = cli.StringFlag{
				Name:  "layout",
				Usage: "OCI image URI of the form 'path'",
			}

			// Add BeforeFunc that will verify code.
			cmd.Before = func(ctx *cli.Context) error {
				dir := ctx.String("layout")
				// Verify directory value.
				if strings.Contains(dir, ":") {
					return errors.Wrap(fmt.Errorf("directory contains ':' character: '%s'", dir), "invalid --layout")
				}
				if dir == "" {
					return errors.Wrap(fmt.Errorf("invalid --layout: directory is empty"), "invalid --layout")
				}

				ctx.App.Metadata["layout"] = dir

				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}
		default:
			// This is a programming error. All umoci commands should fall into
			// one of the above categories.
			panic("Unknown command category: " + cmd.Category)
		}

		cmd.Flags = append([]cli.Flag{flag}, cmd.Flags...)
		app.Commands[idx] = cmd
	}

	// Actually run umoci.
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
