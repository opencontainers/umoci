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
	"github.com/apex/log"
	"github.com/openSUSE/umoci"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var initCommand = cli.Command{
	Name:  "init",
	Usage: "create a new OCI layout",
	ArgsUsage: `--layout <image-path>

Where "<image-path>" is the path to the OCI image to be created.

The new OCI image does not contain any references or blobs, but those can be
created through the use of umoci-new(1), umoci-tag(1) and other similar
commands.`,

	// create modifies an image layout.
	Category: "layout",

	Action: initLayout,
}

func initLayout(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)

	layout, err := umoci.CreateLayout(imagePath)
	if err != nil {
		return errors.Wrap(err, "create layout")
	}
	layout.Close()
	log.Infof("created new OCI image: %s", imagePath)
	return nil
}
