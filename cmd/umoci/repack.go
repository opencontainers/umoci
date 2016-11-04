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
	"io"
	"os"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/layerdiff"
	"github.com/urfave/cli"
	"github.com/vbatts/go-mtree"
)

var repackCommand = cli.Command{
	Name:  "repack",
	Usage: "repacks an OCI runtime bundle into a reference",
	ArgsUsage: `--image <image-path> --from <reference> --bundle <bundle-path>

Where "<image-path>" is the path to the OCI image, "<reference>" is the name of
the reference descriptor which was used to generate the original runtime bundle
and "<bundle-path>" is the destination to repack the image to.

It should be noted that this is not the same as oci-create-layer because it
uses go-mtree to create diff layers from runtime bundles unpacked with
umoci-unpack(1). In addition, it modifies the image so that all of the relevant
manifest and configuration information uses the new diff atop the old manifest.`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
		cli.StringFlag{
			Name:  "from",
			Usage: "reference descriptor name to repack",
		},
		cli.StringFlag{
			Name:  "bundle",
			Usage: "destination bundle path",
		},
	},

	Action: repack,
}

func repack(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.String("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	bundlePath := ctx.String("bundle")
	if bundlePath == "" {
		return fmt.Errorf("bundle path cannot be empty")
	}
	refName := ctx.String("from")
	if refName == "" {
		return fmt.Errorf("reference name cannot be empty")
	}

	// FIXME: This *should* be named after the digest referenced by the ref.
	mtreePath := filepath.Join(bundlePath, refName+".mtree")
	fullRootfsPath := filepath.Join(bundlePath, rootfsPath)

	logrus.WithFields(logrus.Fields{
		"image":  imagePath,
		"bundle": bundlePath,
		"ref":    refName,
		"rootfs": rootfsPath,
		"mtree":  mtreePath,
	}).Debugf("umoci: repacking OCI image")

	mfh, err := os.Open(mtreePath)
	if err != nil {
		return err
	}
	defer mfh.Close()

	spec, err := mtree.ParseSpec(mfh)
	if err != nil {
		return err
	}

	keywords := mtree.CollectUsedKeywords(spec)

	diffs, err := mtree.Check(fullRootfsPath, spec, keywords)
	if err != nil {
		return err
	}

	reader, err := layerdiff.GenerateLayer(fullRootfsPath, diffs)
	if err != nil {
		return err
	}

	io.Copy(os.Stdout, reader)

	// TODO: Modify the configuration and manifest to use the new layer. -- requires CAS
	// TODO: Add all of those blobs. -- requires CAS
	// TODO: Add the refs. -- requires CAS

	return fmt.Errorf("not implemented")
}
