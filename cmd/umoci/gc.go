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
	"regexp"

	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	casDir "github.com/wking/casengine/dir"
	"golang.org/x/net/context"
)

var gcCommand = cli.Command{
	Name:  "gc",
	Usage: "garbage-collects an OCI image's blobs",
	ArgsUsage: `--layout <image-path>

Where "<image-path>" is the path to the OCI image.

This command will do a mark-and-sweep garbage collection of the provided OCI
image, only retaining blobs which can be reached by a descriptor path from the
root set of references. All other blobs will be removed.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "digest-regexp",
			Usage: "regular expression for calculating the digest from a filesystem path.  This is required if your oci-layout declares an oci-cas-template-v1 CAS engine (e.g. via 'umoci init --blob-uri ...')",
		},
	},

	// create modifies an image layout.
	Category: "layout",

	Before: func(ctx *cli.Context) error {
		if _, ok := ctx.App.Metadata["--image-path"]; !ok {
			return errors.Errorf("missing mandatory argument: --layout")
		}
		return nil
	},

	Action: gc,
}

func gc(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)

	var getDigest casDir.GetDigest
	if ctx.IsSet("digest-regexp") {
		getDigestRegexp, err := regexp.Compile(ctx.String("digest-regexp"))
		if err != nil {
			return errors.Wrap(err, "compile digest-regexp")
		}

		regexpGetDigest := &casDir.RegexpGetDigest{
			Regexp: getDigestRegexp,
		}

		getDigest = regexpGetDigest.GetDigest
	}

	// Get a reference to the CAS.
	engine, err := dir.OpenWithDigestLister(imagePath, getDigest)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()

	// Run the GC.
	return errors.Wrap(engineExt.GC(context.Background()), "gc")
}
