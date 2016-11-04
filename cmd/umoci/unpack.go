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
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/image-tools/image"
	"github.com/urfave/cli"
	"github.com/vbatts/go-mtree"
)

const rootfsPath = "rootfs"

var unpackCommand = cli.Command{
	Name:  "unpack",
	Usage: "unpacks a reference into an OCI runtime bundle",
	ArgsUsage: `--image <image-path> --from <reference> --bundle <bundle-path>

Where "<image-path>" is the path to the OCI image, "<reference>" is the name of
the reference descriptor to unpacka and "<bundle-path>" is the destination to
unpack the image to.

It should be noted that this is not the same as oci-create-runtime-bundle,
because this command also will create an mtree specification to allow for layer
creation with umoci-repack(1).`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
		cli.StringFlag{
			Name:  "from",
			Usage: "reference descriptor name to unpack",
		},
		cli.StringFlag{
			Name:  "bundle",
			Usage: "destination bundle path",
		},
	},

	Action: unpack,
}

func unpack(ctx *cli.Context) error {
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

	logrus.WithFields(logrus.Fields{
		"image":  imagePath,
		"bundle": bundlePath,
		"ref":    refName,
		"rootfs": rootfsPath,
	}).Debugf("umoci: unpacking OCI image")

	// Unpack the runtime bundle.
	if err := os.MkdirAll(bundlePath, 0755); err != nil {
		return fmt.Errorf("failed to create bundle path: %q", err)
	}
	// XXX: We should probably defer os.RemoveAll(bundlePath).
	// FIXME: Currently we only support OCI layouts, not tar archives. This
	//        should be fixed once the CAS engine PR is merged into
	//        image-tools. https://github.com/opencontainers/image-tools/pull/5
	// FIXME: This also currently requires root privileges in order to extract
	//        something owned by root, which is a real shame. There are some
	//        PRs to fix this though. https://github.com/opencontainers/image-tools/pull/3
	if err := image.CreateRuntimeBundleLayout(imagePath, bundlePath, refName, rootfsPath); err != nil {
		return fmt.Errorf("failed to create runtime bundle: %q", err)
	}

	// Create the mtree manifest.
	keywords := append(mtree.DefaultKeywords[:], "sha256digest")

	logrus.WithFields(logrus.Fields{
		"keywords": keywords,
		"mtree":    mtreePath,
	}).Debugf("umoci: generating mtree manifest")

	fullRootfsPath := filepath.Join(bundlePath, rootfsPath)
	dh, err := mtree.Walk(fullRootfsPath, nil, keywords)
	if err != nil {
		return fmt.Errorf("failed to generate mtree spec: %q", err)
	}

	fh, err := os.OpenFile(mtreePath, os.O_EXCL|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open mtree spec for writing: %q", err)
	}
	defer fh.Close()

	logrus.Debugf("umoci: saving mtree manifest")

	if _, err := dh.WriteTo(fh); err != nil {
		return fmt.Errorf("failed to write mtree spec: %q", err)
	}

	logrus.Debugf("umoci: unpacking complete")
	return nil
}
