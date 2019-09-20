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
	"github.com/openSUSE/umoci/experimental/ociv2/restore"
	"github.com/openSUSE/umoci/experimental/ociv2/restore/filestore"
	"github.com/openSUSE/umoci/experimental/ociv2/spec/v2"
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/openSUSE/umoci/oci/casext/mediatype"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var restoreCommand = cli.Command{
	Name:  "restore",
	Usage: "restores an OCIv2 tree into the given rootfs",
	ArgsUsage: `--image <image-path>[:<tag>] [--file-store <path>] <root>

If --file-store is not set, then all inodes are created wholesale without any
deduplication between different extractions.`,

	Category: "image",

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "file-store",
			Usage: "path to use for file-store deduplication",
		},
	},

	Action: doRestore,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 1 {
			return errors.Errorf("invalid number of positional arguments: expected <root>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("root path cannot be empty")
		}
		if ctx.IsSet("file-store") && ctx.String("file-store") == "" {
			return errors.Errorf("file-store path cannot be empty")
		}
		ctx.App.Metadata["root"] = ctx.Args().First()
		return nil
	},
}

func init() {
	// TODO: Just a hotfix.
	mediatype.RegisterTarget(v2.MediaTypeRoot)
}

func doRestore(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)
	rootPath := ctx.App.Metadata["root"].(string)

	casCtx := context.Background()

	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	// Get the root descriptor.
	descPaths, err := engineExt.ResolveReference(casCtx, fromName)
	if err != nil {
		return errors.Wrap(err, "resolve reference")
	}
	if len(descPaths) == 0 {
		return errors.Errorf("tag not found: %s", fromName)
	}
	if len(descPaths) != 1 {
		// TODO: Handle this more nicely.
		return errors.Errorf("tag is ambiguous: %s", fromName)
	}
	desc := descPaths[0].Descriptor()

	// Grab a handle for the underlying filestore.
	var fileStore filestore.Store
	if ctx.IsSet("file-store") {
		fileStore, err = filestore.Open(ctx.String("file-store"))
		if err != nil {
			return errors.Wrap(err, "open file-store")
		}
		defer fileStore.Close()
	}

	restorer := &restore.Restorer{
		Engine: engineExt,
		Store:  fileStore,
	}
	err = restorer.Restore(casCtx, desc, rootPath)
	if err != nil {
		return errors.Wrap(err, "restore root")
	}
	return nil
}
