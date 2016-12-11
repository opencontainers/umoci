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
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	igen "github.com/cyphar/umoci/image/generator"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

// FIXME: We should also implement a raw mode that just does modifications of
//        JSON blobs (allowing this all to be used outside of our build setup).
var configCommand = uxHistory(uxTag(cli.Command{
	Name:  "config",
	Usage: "modifies the image configuration of an OCI image",
	ArgsUsage: `--image <image-path>[:<tag>] [--tag <new-tag>]

Where "<image-path>" is the path to the OCI image, and "<tag>" is the name of
the tagged image from which the config modifications will be based (if not
specified, it defaults to "latest"). "<new-tag>" is the new reference name to
save the new image as, if this is not specified then umoci will replace the old
image.`,

	// config modifies a particular image manifest.
	Category: "image",

	// Verify the metadata.
	Before: func(ctx *cli.Context) error {
		if _, ok := ctx.App.Metadata["--image-path"]; !ok {
			return errors.Errorf("missing mandatory argument: --image")
		}
		if _, ok := ctx.App.Metadata["--image-tag"]; !ok {
			return errors.Errorf("missing mandatory argument: --image")
		}
		return nil
	},

	Flags: []cli.Flag{
		cli.StringFlag{Name: "config.user"},
		cli.Int64Flag{Name: "config.memory.limit"},
		cli.Int64Flag{Name: "config.memory.swap"},
		cli.Int64Flag{Name: "config.cpu.shares"},
		cli.StringSliceFlag{Name: "config.exposedports"},
		cli.StringSliceFlag{Name: "config.env"},
		cli.StringSliceFlag{Name: "config.entrypoint"}, // FIXME: This interface is weird.
		cli.StringSliceFlag{Name: "config.cmd"},        // FIXME: This interface is weird.
		cli.StringSliceFlag{Name: "config.volume"},
		cli.StringSliceFlag{Name: "config.label"},
		cli.StringFlag{Name: "config.workingdir"},
		// FIXME: These aren't really safe to expose.
		//cli.StringFlag{Name: "rootfs.type"},
		//cli.StringSliceFlag{Name: "rootfs.diffids"},
		cli.StringFlag{Name: "created"}, // FIXME: Implement TimeFlag.
		cli.StringFlag{Name: "author"},
		cli.StringFlag{Name: "architecture"},
		cli.StringFlag{Name: "os"},
		cli.StringSliceFlag{Name: "manifest.annotation"},
		cli.StringSliceFlag{Name: "clear"},
	},

	Action: config,
}))

// TODO: This can be scripted by have a list of mappings to mutation methods.
func mutateConfig(g *igen.Generator, m *v1.Manifest, ctx *cli.Context) error {
	if ctx.IsSet("clear") {
		for _, key := range ctx.StringSlice("clear") {
			switch key {
			case "config.labels":
				g.ClearConfigLabels()
			case "manifest.annotations":
				m.Annotations = nil
			case "config.exposedports":
				g.ClearConfigExposedPorts()
			case "config.env":
				g.ClearConfigEnv()
			case "config.volume":
				g.ClearConfigVolumes()
			case "rootfs.diffids":
				//g.ClearRootfsDiffIDs()
				return errors.Errorf("--clear=rootfs.diffids is not safe")
			default:
				return errors.Errorf("unknown key to --clear: %s", key)
			}
		}
	}

	// FIXME: Implement TimeFlag.
	if ctx.IsSet("created") {
		// How do we handle other formats?
		created, err := time.Parse(igen.ISO8601, ctx.String("created"))
		if err != nil {
			return errors.Wrap(err, "parse --created")
		}
		g.SetCreated(created)
	}
	if ctx.IsSet("author") {
		g.SetAuthor(ctx.String("author"))
	}
	if ctx.IsSet("architecture") {
		g.SetArchitecture(ctx.String("architecture"))
	}
	if ctx.IsSet("os") {
		g.SetOS(ctx.String("os"))
	}
	if ctx.IsSet("config.user") {
		g.SetConfigUser(ctx.String("config.user"))
	}
	if ctx.IsSet("config.workingdir") {
		g.SetConfigWorkingDir(ctx.String("config.workingdir"))
	}
	if ctx.IsSet("config.memory.limit") {
		g.SetConfigMemory(ctx.Int64("config.memory.limit"))
	}
	if ctx.IsSet("config.memory.swap") {
		g.SetConfigMemorySwap(ctx.Int64("config.memory.swap"))
	}
	if ctx.IsSet("config.cpu.shares") {
		g.SetConfigCPUShares(ctx.Int64("config.cpu.shares"))
	}
	if ctx.IsSet("config.exposedports") {
		for _, port := range ctx.StringSlice("config.exposedports") {
			g.AddConfigExposedPort(port)
		}
	}
	if ctx.IsSet("config.env") {
		for _, env := range ctx.StringSlice("config.env") {
			g.AddConfigEnv(env)
		}
	}
	// FIXME: This interface is weird.
	if ctx.IsSet("config.entrypoint") {
		g.SetConfigEntrypoint(ctx.StringSlice("config.entrypoint"))
	}
	// FIXME: This interface is weird.
	if ctx.IsSet("config.cmd") {
		g.SetConfigCmd(ctx.StringSlice("config.cmd"))
	}
	if ctx.IsSet("config.volume") {
		for _, volume := range ctx.StringSlice("config.volume") {
			g.AddConfigVolume(volume)
		}
	}
	if ctx.IsSet("config.label") {
		for _, label := range ctx.StringSlice("config.label") {
			parts := strings.SplitN(label, "=", 2)
			g.AddConfigLabel(parts[0], parts[1])
		}
	}
	// FIXME: These aren't really safe to expose.
	if ctx.IsSet("rootfs.type") {
		g.SetRootfsType(ctx.String("rootfs.type"))
	}
	if ctx.IsSet("rootfs.diffids") {
		for _, diffid := range ctx.StringSlice("rootfs.diffid") {
			g.AddRootfsDiffID(diffid)
		}
	}
	if ctx.IsSet("manifest.annotation") {
		if m.Annotations == nil {
			m.Annotations = map[string]string{}
		}
		for _, label := range ctx.StringSlice("manifest.annotation") {
			parts := strings.SplitN(label, "=", 2)
			m.Annotations[parts[0]] = parts[1]
		}
	}

	return nil
}

func config(ctx *cli.Context) error {
	imagePath := ctx.App.Metadata["--image-path"].(string)
	fromName := ctx.App.Metadata["--image-tag"].(string)

	// By default we clobber the old tag.
	tagName := fromName
	if val, ok := ctx.App.Metadata["--tag"]; ok {
		tagName = val.(string)
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	defer engine.Close()

	fromDescriptor, err := engine.GetReference(context.TODO(), fromName)
	if err != nil {
		return errors.Wrap(err, "get from reference")
	}

	// FIXME: Implement support for manifest lists.
	if fromDescriptor.MediaType != v1.MediaTypeImageManifest {
		return errors.Wrap(fmt.Errorf("descriptor does not point to v1.MediaTypeImageManifest: not implemented: %s", fromDescriptor.MediaType), "invalid --image tag")
	}

	// TODO TODO: Implement the configuration modification. The rest comes from
	//            repack, and should be mostly unchanged.

	// XXX: I get the feeling all of this should be moved to a separate package
	//      which abstracts this nicely.

	manifestBlob, err := cas.FromDescriptor(context.TODO(), engine, fromDescriptor)
	if err != nil {
		return errors.Wrap(err, "get from manifest")
	}
	defer manifestBlob.Close()

	logrus.WithFields(logrus.Fields{
		"digest": manifestBlob.Digest,
	}).Debugf("umoci: got original manifest")

	manifest, ok := manifestBlob.Data.(*v1.Manifest)
	if !ok {
		// Should never be reached.
		return errors.Errorf("manifest blob type not implemented: %s", manifestBlob.MediaType)
	}

	// We also need to update the config. Fun.
	configBlob, err := cas.FromDescriptor(context.TODO(), engine, &manifest.Config)
	if err != nil {
		return errors.Wrap(err, "get from config")
	}
	defer configBlob.Close()

	logrus.WithFields(logrus.Fields{
		"digest": configBlob.Digest,
	}).Debugf("umoci: got original config")

	config, ok := configBlob.Data.(*v1.Image)
	if !ok {
		// Should not be reached.
		return errors.Errorf("config blob type not implemented: %s", configBlob.MediaType)
	}

	g, err := igen.NewFromImage(*config)
	if err != nil {
		return errors.Wrap(err, "create new generator")
	}

	// Now we mutate the config.
	if err := mutateConfig(g, manifest, ctx); err != nil {
		return errors.Wrap(err, "mutate config")
	}

	var (
		author    = g.Author()
		comment   = ""
		created   = time.Now().Format(igen.ISO8601)
		createdBy = "umoci config" // XXX: should we append argv to this?
	)

	if val, ok := ctx.App.Metadata["--history.author"]; ok {
		author = val.(string)
	}
	if val, ok := ctx.App.Metadata["--history.comment"]; ok {
		comment = val.(string)
	}
	if val, ok := ctx.App.Metadata["--history.created"]; ok {
		created = val.(string)
	}
	if val, ok := ctx.App.Metadata["--history.created_by"]; ok {
		createdBy = val.(string)
	}

	// Add a history entry about the fact we just changed the config.
	// FIXME: It should be possible to disable this.
	g.AddHistory(v1.History{
		Created:    created,
		CreatedBy:  createdBy,
		Author:     author,
		Comment:    comment,
		EmptyLayer: true,
	})

	// Update config and create a new blob for it.
	*config = g.Image()
	newConfigDigest, newConfigSize, err := engine.PutBlobJSON(context.TODO(), config)
	if err != nil {
		return errors.Wrap(err, "put config blob")
	}

	logrus.WithFields(logrus.Fields{
		"digest": newConfigDigest,
		"size":   newConfigSize,
	}).Debugf("umoci: added new config")

	// Update the manifest to point at the new config, then create a new blob
	// for it.
	manifest.Config.Digest = newConfigDigest
	manifest.Config.Size = newConfigSize
	newManifestDigest, newManifestSize, err := engine.PutBlobJSON(context.TODO(), manifest)
	if err != nil {
		return errors.Wrap(err, "put manifest blob")
	}

	logrus.WithFields(logrus.Fields{
		"digest": newManifestDigest,
		"size":   newManifestSize,
	}).Debugf("umoci: added new manifest")

	// Now create a new reference, and either add it to the engine or spew it
	// to stdout.

	newDescriptor := &v1.Descriptor{
		// FIXME: Support manifest lists.
		MediaType: v1.MediaTypeImageManifest,
		Digest:    newManifestDigest,
		Size:      newManifestSize,
	}

	logrus.WithFields(logrus.Fields{
		"mediatype": newDescriptor.MediaType,
		"digest":    newDescriptor.Digest,
		"size":      newDescriptor.Size,
	}).Infof("created new image")

	// We have to clobber the old reference.
	// XXX: Should we output some warning if we actually did remove an old
	//      reference?
	if err := engine.DeleteReference(context.TODO(), tagName); err != nil {
		return errors.Wrap(err, "delete old tag")
	}
	if err := engine.PutReference(context.TODO(), tagName, newDescriptor); err != nil {
		return errors.Wrap(err, "add new tag")
	}

	return nil
}
