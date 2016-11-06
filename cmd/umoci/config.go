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
	"encoding/json"
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cyphar/umoci/image/cas"
	igen "github.com/cyphar/umoci/image/generator"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

var configFlags = []cli.Flag{
	cli.StringFlag{Name: "config.user"},
	cli.Int64Flag{Name: "config.memory.limit"},
	cli.Int64Flag{Name: "config.memory.swap"},
	cli.Int64Flag{Name: "config.cpu.shares"},
	cli.StringSliceFlag{Name: "config.exposedports"},
	cli.StringSliceFlag{Name: "config.env"},
	cli.StringSliceFlag{Name: "config.entrypoint"}, // FIXME: This interface is weird.
	cli.StringSliceFlag{Name: "config.cmd"},        // FIXME: This interface is weird.
	cli.StringSliceFlag{Name: "config.volume"},
	cli.StringFlag{Name: "config.workingdir"},
	// FIXME: These aren't really safe to expose.
	//cli.StringFlag{Name: "rootfs.type"},
	//cli.StringSliceFlag{Name: "rootfs.diffids"},
	cli.StringSliceFlag{Name: "history"}, // FIXME: Implement this is a way that isn't super dodgy.
	cli.StringFlag{Name: "created"},      // FIXME: Implement TimeFlag.
	cli.StringFlag{Name: "author"},
	cli.StringFlag{Name: "architecture"},
	cli.StringFlag{Name: "os"},
	cli.StringSliceFlag{Name: "clear"},
}

// FIXME: This is ugly.
func init() {
	configCommand.Flags = append(configCommand.Flags, configFlags...)
}

// FIXME: We should also implement a raw mode that just does modifications of
//        JSON blobs (allowing this all to be used outside of our build setup).
var configCommand = cli.Command{
	Name:  "config",
	Usage: "modifies the image configuration of an OCI image",
	ArgsUsage: `--image <image-path> --from <reference>

Where "<image-path>" is the path to the OCI image, and "<reference>" is the
name of the reference descriptor from which the config modifications will be
based.`,

	Flags: []cli.Flag{
		// FIXME: This really should be a global option.
		cli.StringFlag{
			Name:  "image",
			Usage: "path to OCI image bundle",
		},
		cli.StringFlag{
			Name:  "from",
			Usage: "reference descriptor name to modify",
		},
		cli.StringFlag{
			Name:  "tag",
			Usage: "tag name for repacked image",
		},
	},

	Action: config,
}

// TODO: This can be scripted by have a list of mappings to mutation methods.
func mutateConfig(g *igen.Generator, ctx *cli.Context) error {
	if ctx.IsSet("clear") {
		for _, key := range ctx.StringSlice("clear") {
			switch key {
			case "config.exposedports":
				g.ClearConfigExposedPorts()
			case "config.env":
				g.ClearConfigEnv()
			case "config.volume":
				g.ClearConfigVolumes()
			case "rootfs.diffids":
				//g.ClearRootfsDiffIDs()
				return fmt.Errorf("clear rootfs.diffids is not safe")
			case "history":
				g.ClearHistory()
			default:
				return fmt.Errorf("unknown set to clear: %s", key)
			}
		}
	}

	// FIXME: Implement TimeFlag.
	if ctx.IsSet("created") {
		// How do we handle other formats?
		created, err := time.Parse(igen.ISO8601, ctx.String("created"))
		if err != nil {
			return err
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
	// FIXME: These aren't really safe to expose.
	if ctx.IsSet("rootfs.type") {
		g.SetRootfsType(ctx.String("rootfs.type"))
	}
	if ctx.IsSet("rootfs.diffids") {
		for _, diffid := range ctx.StringSlice("rootfs.diffid") {
			g.AddRootfsDiffID(diffid)
		}
	}
	// FIXME: Also implement this is a way that isn't broken (using string is broken).
	if ctx.IsSet("history") {
		// This is a JSON-encoded version of v1.History. I'm sorry.
		for _, historyJSON := range ctx.StringSlice("history") {
			var history v1.History
			if err := json.Unmarshal([]byte(historyJSON), &history); err != nil {
				return fmt.Errorf("error reading --history argument: %s", err)
			}
			g.AddHistory(history)
		}
	}
	return nil
}

func config(ctx *cli.Context) error {
	// FIXME: Is there a nicer way of dealing with mandatory arguments?
	imagePath := ctx.String("image")
	if imagePath == "" {
		return fmt.Errorf("image path cannot be empty")
	}
	fromName := ctx.String("from")
	if fromName == "" {
		return fmt.Errorf("reference name cannot be empty")
	}

	// Get a reference to the CAS.
	engine, err := cas.Open(imagePath)
	if err != nil {
		return err
	}
	defer engine.Close()

	fromDescriptor, err := engine.GetReference(context.TODO(), fromName)
	if err != nil {
		return err
	}

	// FIXME: Implement support for manifest lists.
	if fromDescriptor.MediaType != v1.MediaTypeImageManifest {
		return fmt.Errorf("--from descriptor does not point to v1.MediaTypeImageManifest: not implemented: %s", fromDescriptor.MediaType)
	}

	// TODO TODO: Implement the configuration modification. The rest comes from
	//            repack, and should be mostly unchanged.

	// XXX: I get the feeling all of this should be moved to a separate package
	//      which abstracts this nicely.

	manifestBlob, err := cas.FromDescriptor(context.TODO(), engine, fromDescriptor)
	if err != nil {
		return err
	}
	defer manifestBlob.Close()

	logrus.WithFields(logrus.Fields{
		"digest": manifestBlob.Digest,
	}).Debugf("umoci: got original manifest")

	manifest, ok := manifestBlob.Data.(*v1.Manifest)
	if !ok {
		// Should never be reached.
		return fmt.Errorf("manifest blob type not implemented: %s", manifestBlob.MediaType)
	}

	// We also need to update the config. Fun.
	configBlob, err := cas.FromDescriptor(context.TODO(), engine, &manifest.Config)
	if err != nil {
		return err
	}
	defer configBlob.Close()

	logrus.WithFields(logrus.Fields{
		"digest": configBlob.Digest,
	}).Debugf("umoci: got original config")

	config, ok := configBlob.Data.(*v1.Image)
	if !ok {
		// Should not be reached.
		return fmt.Errorf("config blob type not implemented: %s", configBlob.MediaType)
	}

	g, err := igen.NewFromImage(*config)
	if err != nil {
		return err
	}

	// Now we mutate the config.
	if err := mutateConfig(g, ctx); err != nil {
		return err
	}

	// Update config and create a new blob for it.
	*config = g.Image()
	newConfigDigest, newConfigSize, err := engine.PutBlobJSON(context.TODO(), config)
	if err != nil {
		return err
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

	tagName := ctx.String("tag")
	if tagName == "" {
		return nil
	}

	// We have to clobber the old reference.
	// XXX: Should we output some warning if we actually did remove an old
	//      reference?
	if err := engine.DeleteReference(context.TODO(), tagName); err != nil {
		return err
	}
	if err := engine.PutReference(context.TODO(), tagName, newDescriptor); err != nil {
		return err
	}

	return nil
}
