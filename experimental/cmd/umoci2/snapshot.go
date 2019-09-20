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

	"github.com/openSUSE/umoci/experimental/ociv2/snapshot"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var snapshotCommand = cli.Command{
	Name:      "snapshot",
	Usage:     "snapshots a rootfs to produce an OCIv2 tree",
	ArgsUsage: `--image <image-path>[:<tag>] <root>`,

	Category: "image",

	Action: doSnapshot,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.Errorf("invalid number of positional arguments: expected <root>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("root path cannot be empty")
		}
		ctx.App.Metadata["root"] = ctx.Args().First()
		return nil
	},
}

func doSnapshot(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	toName := ctx.App.Metadata["--image-tag"].(string)
	rootPath := ctx.App.Metadata["root"].(string)

	casCtx := context.Background()

	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	rootDesc, err := snapshot.Snapshot(casCtx, engineExt, rootPath)
	if err != nil {
		return errors.Wrap(err, "snapshot root")
	}

	err = engineExt.UpdateReference(casCtx, toName, *rootDesc)
	if err != nil {
		return errors.Wrap(err, "add reference")
	}

	fmt.Printf("new root descriptor: %+v\n", *rootDesc)
	return nil
}
