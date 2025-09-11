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

// Package main is the cli implementation of umoci.
package main

import (
	"errors"
	"fmt"
	"os"
	"runtime/pprof"

	"github.com/apex/log"
	logcli "github.com/apex/log/handlers/cli"
	"github.com/urfave/cli"

	"github.com/opencontainers/umoci"
)

const (
	usage = `umoci modifies Open Container images`

	// Categories used to automatically monkey-patch flags to commands.
	categoryLayout = "layout"
	categoryImage  = "image"
)

// Main is the underlying main() implementation. You can call this directly as
// though it were the command-line arguments of the umoci binary (this is
// needed for umoci's integration test hacks you can find in main_test.go).
func Main(args []string) error {
	app := cli.NewApp()
	app.Name = "umoci"
	app.Usage = usage
	app.Authors = []cli.Author{
		{
			Name:  "Aleksa Sarai",
			Email: "asarai@suse.com",
		},
	}
	app.Version = umoci.FullVersion()

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
		cli.StringFlag{
			Name:   "cpu-profile",
			Usage:  "profile umoci during execution and output it to a file",
			Hidden: true,
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
				return fmt.Errorf("[internal error] failure auto-setting --log=info: %w", err)
			}
		}
		level, err := log.ParseLevel(ctx.GlobalString("log"))
		if err != nil {
			return fmt.Errorf("parsing log level: %w", err)
		}
		log.SetLevel(level)

		if path := ctx.GlobalString("cpu-profile"); path != "" {
			fh, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("opening cpu-profile path: %w", err)
			}
			if err := pprof.StartCPUProfile(fh); err != nil {
				return fmt.Errorf("start cpu-profile: %w", err)
			}
		}
		return nil
	}

	app.After = func(*cli.Context) error {
		pprof.StopCPUProfile()
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
		rawSubcommand,
		insertCommand,
	}

	app.Metadata = map[string]any{}

	// In order to make the uxXyz wrappers not too cumbersome we automatically
	// add them to images with categories set to categoryImage or
	// categoryLayout. Monkey patching was never this neat.
	foreachSubcommand(app.Commands, func(cmd *cli.Command) {
		switch cmd.Category {
		case categoryImage:
			oldBefore := cmd.Before
			cmd.Before = func(ctx *cli.Context) error {
				if _, ok := ctx.App.Metadata["--image-path"]; !ok {
					return errors.New("missing mandatory argument: --image")
				}
				if _, ok := ctx.App.Metadata["--image-tag"]; !ok {
					return errors.New("missing mandatory argument: --image")
				}
				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}
			*cmd = uxImage(*cmd)
		case categoryLayout:
			oldBefore := cmd.Before
			cmd.Before = func(ctx *cli.Context) error {
				if _, ok := ctx.App.Metadata["--image-path"]; !ok {
					return errors.New("missing mandatory argument: --layout")
				}
				if oldBefore != nil {
					return oldBefore(ctx)
				}
				return nil
			}
			*cmd = uxLayout(*cmd)
		}
	})

	err := app.Run(args)
	if err != nil {
		// If an error is a permission based error, give a hint to the user
		// that --rootless might help. We probably should only be doing this if
		// we're an unprivileged user.
		if errors.Is(err, os.ErrPermission) {
			log.Warn("umoci encountered a permission error: maybe --rootless will help?")
		}
		log.Debugf("%+v", err)
	}
	return err
}

func main() {
	if err := Main(os.Args); err != nil {
		log.Fatalf("%v", err)
	}
}
