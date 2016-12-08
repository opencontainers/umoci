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

	"github.com/Sirupsen/logrus"
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
		createCommand,
		tagCommand,
		statCommand,
	}

	// Actually run umoci.
	if err := app.Run(os.Args); err != nil {
		logrus.Fatal(err)
	}
}
