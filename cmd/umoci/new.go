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
	"runtime"
	"time"

	"github.com/apex/log"
	"github.com/openSUSE/umoci"
	igen "github.com/openSUSE/umoci/oci/config/generate"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var newCommand = cli.Command{
	Name:  "new",
	Usage: "creates a blank tagged OCI image",
	ArgsUsage: `--image <image-path>:<new-tag>

Where "<image-path>" is the path to the OCI image, and "<new-tag>" is the name
of the tag for the empty manifest.

Once you create a new image with umoci-new(1) you can directly use the image
with umoci-unpack(1), umoci-repack(1), and umoci-config(1) to modify the new
manifest as you see fit. This allows you to create entirely new images without
needing a base image to start from.`,

	// new modifies an image layout.
	Category: "image",

	Action: newImage,
}

func newImage(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	tagName := ctx.App.Metadata["--image-tag"].(string)

	layout, err := umoci.OpenLayout(imagePath)
	if err != nil {
		return errors.Wrap(err, "open layout")
	}
	defer layout.Close()

	// Create a new manifest.
	log.WithFields(log.Fields{
		"tag": tagName,
	}).Debugf("creating new manifest")

	// Create a new image config.
	g := igen.New()
	createTime := time.Now()

	// Set all of the defaults we need.
	g.SetCreated(createTime)
	g.SetOS(runtime.GOOS)
	g.SetArchitecture(runtime.GOARCH)
	g.ClearHistory()

	// Make sure we have no diffids.
	g.SetRootfsType("layers")
	g.ClearRootfsDiffIDs()

	p := ispec.Platform{
		Architecture: runtime.GOARCH,
		OS:           runtime.GOOS,
	}

	img := g.Image()
	if err := layout.NewImage(tagName, &img, nil, &p); err != nil {
		return errors.Wrap(err, "new image")
	}
	return nil
}
