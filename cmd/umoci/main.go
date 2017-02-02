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
	"os"

	"github.com/apex/log"
	logcli "github.com/apex/log/handlers/cli"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// version is version ID for the source, read from VERSION in the source and
// populated on build by make.
var version = ""

// gitCommit is the commit hash that the binary was built from and will be
// populated on build by make.
var gitCommit = ""

const (
	usage = `umoci modifies Open Container images`

	// Categories used to automatically monkey-patch flags to commands.
	categoryLayout = "layout"
	categoryImage  = "image"
)

func main() {
	// TODO: Add some form of --verbose flag.
	log.SetHandler(logcli.New(os.Stderr))

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

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "debug",
			Usage: "set log level to debug",
		},
	}

	app.Before = func(ctx *cli.Context) error {
		if ctx.GlobalBool("debug") {
			log.SetLevel(log.DebugLevel)
			errors.Debug(true)
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

	// In order to make the uxXyz wrappers not too cumbersome we automatically
	// add them to images with categories set to categoryImage or
	// categoryLayout. Monkey patching was never this neat.
	for idx, cmd := range app.Commands {
		switch cmd.Category {
		case categoryImage:
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
			cmd = uxImage(cmd)
		case categoryLayout:
			oldBefore := cmd.Before
			cmd.Before = func(ctx *cli.Context) error {
				if _, ok := ctx.App.Metadata["--image-path"]; !ok {
					return errors.Errorf("missing mandatory argument: --layout")
				}
				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}
			cmd = uxLayout(cmd)
		}
		app.Commands[idx] = cmd
	}

	// Actually run umoci.
	if err := app.Run(os.Args); err != nil {
		// If an error is a permission based error, give a hint to the user
		// that --rootless might help. We probably should only be doing this if
		// we're an unprivileged user.
		if os.IsPermission(errors.Cause(err)) {
			log.Info("umoci encountered a permission error: maybe --rootless will help?")
		}
		log.Fatalf("%v", err)
	}
}
