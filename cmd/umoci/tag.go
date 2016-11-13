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
	"io/ioutil"
	"os"
	"text/tabwriter"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

func isValidMediaType(mediaType string) bool {
	validTypes := map[string]struct{}{
		v1.MediaTypeImageManifest:              {},
		v1.MediaTypeImageManifestList:          {},
		v1.MediaTypeImageConfig:                {},
		v1.MediaTypeDescriptor:                 {},
		v1.MediaTypeImageLayer:                 {},
		v1.MediaTypeImageLayerNonDistributable: {},
	}
	_, ok := validTypes[mediaType]
	return ok
}

var tagCommand = cli.Command{
	Name:  "tag",
	Usage: "manipulate the references in an OCI image",
	ArgsUsage: `--image <image-path>

Where "<image-path>" is the path to the OCI image. Use the subcommands to
modify tags within the image specified.`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
	},

	Subcommands: []cli.Command{
		tagAddCommand,
		tagRemoveCommand,
		tagListCommand,
	},
}

var tagAddCommand = cli.Command{
	Name:  "add",
	Usage: "adds a reference to an object in an OCI image",
	ArgsUsage: `--tag <tag-name> --blob <digest> [--media-type <mediatype>]

Where "<tag-name>" is the name of the reference being created, and "<digest>"
is the digest of the blob being referenced. If specified, "<mediatype>" is the
hard-coded OCI image-spec media type to use for the tag (if unspecified it is
auto-detected).`,

	Flags: []cli.Flag{
		// FIXME: Add a --no-clobber option or something.
		cli.StringFlag{
			Name:  "tag",
			Usage: "name of tag to create",
		},
		cli.StringFlag{
			Name:  "blob",
			Usage: "digest of blob to reference",
		},
		cli.StringFlag{
			Name:  "media-type",
			Usage: "media type of blob being referenced",
		},
	},

	Action: tagAdd,
}

func tagAdd(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.GlobalString("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	tagName := ctx.String("tag")
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}
	blobDigest := ctx.String("blob")
	if blobDigest == "" {
		return fmt.Errorf("blob digest cannot be empty")
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	mediaType := ctx.String("media-type")
	if mediaType == "" {
		return fmt.Errorf("auto-detection of media-type not implemented")
	}
	if !isValidMediaType(mediaType) {
		return fmt.Errorf("unknown --media-type: %s", mediaType)
	}

	// This is a sanity check to make sure that the digest actually makes sense.
	// FIXME: We really should implement StatBlob and StatReference.
	r, err := engine.GetBlob(context.TODO(), blobDigest)
	if err != nil {
		return fmt.Errorf("--blob could not be found: %s", err)
	}
	defer r.Close()

	blobSize, err := io.Copy(ioutil.Discard, r)
	if err != nil {
		return err
	}

	// Create a new descriptor.
	descriptor := v1.Descriptor{
		MediaType: mediaType,
		Digest:    blobDigest,
		Size:      int64(blobSize),
	}

	// Add it.
	if err := engine.PutReference(context.TODO(), tagName, &descriptor); err != nil {
		return err
	}

	return nil
}

var tagRemoveCommand = cli.Command{
	Name:    "remove",
	Aliases: []string{"rm"},
	Usage:   "removes a reference to an object in an OCI image",
	ArgsUsage: `--tag <tag-name>

Where "<tag-name>" is the name of the reference to delete. An error will not be
emitted if the tag did not exist before running this command.`,

	Flags: []cli.Flag{
		// FIXME: Add a --no-clobber option or something.
		cli.StringFlag{
			Name:  "tag",
			Usage: "name of tag to create",
		},
	},

	Action: tagRemove,
}

func tagRemove(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.GlobalString("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	tagName := ctx.String("tag")
	if tagName == "" {
		return fmt.Errorf("tag name cannot be empty")
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	// Add it.
	if err := engine.DeleteReference(context.TODO(), tagName); err != nil {
		return err
	}

	return nil
}

var tagListCommand = cli.Command{
	Name:    "list",
	Aliases: []string{"ls"},
	Usage:   "lists the set of references in an OCI image",
	ArgsUsage: `

Gives the full list of references in an OCI image in the format:

    <tag-name> <media-type> <digest>

Where "<tag-name>" is the name of the tag, "<media-type>" is the OCI image spec
media type for the referenced blob, and "<digest>" is the digest of the
referenced blob.`,

	Flags: []cli.Flag{
	// FIXME: Add a --format or similar option to allow formatting to work properly.
	},

	Action: tagList,
}

func tagList(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.GlobalString("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	names, err := engine.ListReferences(context.TODO())
	if err != nil {
		return err
	}

	// FIXME: We should probably add more information.
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	//fmt.Fprintln(w, "TAG\tMEDIATYPE\tDIGEST")
	for _, name := range names {
		descriptor, err := engine.GetReference(context.TODO(), name)
		if err != nil {
			logrus.Warnf("could not get reference %s: %s", name, err)
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", name, descriptor.MediaType, descriptor.Digest)
	}

	return w.Flush()
}
