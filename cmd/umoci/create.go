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
	"runtime"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	igen "github.com/cyphar/umoci/image/generator"
	ispec "github.com/opencontainers/image-spec/specs-go"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var createCommand = cli.Command{
	Name:  "create",
	Usage: "creates an OCI image and/or base manifest",
	ArgsUsage: `--image <image-path> [--tag <new-manifest>]

Where "<image-path>" is the path to the OCI image, and "<new-manifest>" is an
optional tag which will be linked to a new empty manifest.

If --tag is specified, the empty manifest created is such that you can use
umoci-unpack(1), umoci-repack(1), and umoci-config(1) to modify the new
manifest as you see fit. This allows you to create entirely new images without
needing a base image to start from.`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
		cli.StringFlag{
			Name:  "tag",
			Usage: "tag name for new manifest",
		},
	},

	Action: create,
}

func create(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.String("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	if fi, err := os.Stat(imagePath); os.IsNotExist(err) {
		logrus.Infof("creating non-existent image")
		if err := cas.CreateLayout(imagePath); err != nil {
			return err
		}
	} else if err != nil {
		return err
	} else if !fi.IsDir() {
		return fmt.Errorf("image path %s is not a directory", imagePath)
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	// Create a new manifest.
	tagName := ctx.String("tag")
	if tagName != "" {
		logrus.WithFields(logrus.Fields{
			"tag": tagName,
		}).Infof("creating new manifest")

		// Create a new image config.
		g := igen.New()
		createTime := time.Now()

		// Set all of the defaults we need.
		g.SetCreated(createTime)
		g.SetOS(runtime.GOOS)
		g.SetArchitecture(runtime.GOARCH)
		g.AddHistory(v1.History{
			CreatedBy:  fmt.Sprintf("umoci create [at '%s']", createTime),
			EmptyLayer: true,
		})

		// Make sure we have no diffids.
		g.SetRootfsType("layers")
		g.ClearRootfsDiffIDs()

		// Update config and create a new blob for it.
		config := g.Image()
		configDigest, configSize, err := engine.PutBlobJSON(context.TODO(), &config)
		if err != nil {
			return err
		}

		logrus.WithFields(logrus.Fields{
			"digest": configDigest,
			"size":   configSize,
		}).Debugf("umoci: added new config")

		// Create a new manifest that just points to the config and has an
		// empty layer set. FIXME: Implement ManifestList support.
		manifest := v1.Manifest{
			Versioned: ispec.Versioned{
				SchemaVersion: 2, // FIXME: This is hardcoded at the moment.
				MediaType:     v1.MediaTypeImageManifest,
			},
			Config: v1.Descriptor{
				MediaType: v1.MediaTypeImageConfig,
				Digest:    configDigest,
				Size:      configSize,
			},
			Layers: []v1.Descriptor{},
		}

		manifestDigest, manifestSize, err := engine.PutBlobJSON(context.TODO(), manifest)

		logrus.WithFields(logrus.Fields{
			"digest": manifestDigest,
			"size":   manifestSize,
		}).Debugf("umoci: added new manifest")

		// Now create a new reference, and either add it to the engine or spew it
		// to stdout.

		descriptor := v1.Descriptor{
			// FIXME: Support manifest lists.
			MediaType: v1.MediaTypeImageManifest,
			Digest:    manifestDigest,
			Size:      manifestSize,
		}

		logrus.WithFields(logrus.Fields{
			"mediatype": descriptor.MediaType,
			"digest":    descriptor.Digest,
			"size":      descriptor.Size,
		}).Infof("created new image")

		// We have to clobber the old reference.
		// XXX: Should we output some warning if we actually did remove an old
		//      reference?
		if err := engine.DeleteReference(context.TODO(), tagName); err != nil {
			return err
		}
		if err := engine.PutReference(context.TODO(), tagName, &descriptor); err != nil {
			return err
		}

		return nil
	}

	return nil
}
