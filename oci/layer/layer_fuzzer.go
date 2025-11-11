//go:build gofuzz

// SPDX-License-Identifier: Apache-2.0
/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2025 SUSE LLC
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

package layer

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"unicode"

	fuzz "github.com/AdaLogics/go-fuzz-headers"
	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	"github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
)

func createRandomFile(dirpath string, filename, filecontents []byte) error {
	fileP := filepath.Join(dirpath, string(filename))
	if err := os.WriteFile(fileP, filecontents, 0o644); err != nil {
		return err
	}
	return nil
}

func createRandomDir(basedir string, dirname []byte, dirArray []string) ([]string, error) {
	dirPath := filepath.Join(basedir, string(dirname))
	if err := os.MkdirAll(dirPath, 0o755); err != nil {
		return dirArray, err
	}
	dirArray = append(dirArray, string(dirname))
	return dirArray, nil
}

func isLetter(input []byte) bool {
	s := string(input)
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// FuzzGenerateLayer implements a fuzzer that targets layer.GenerateLayer().
func FuzzGenerateLayer(data []byte) int {
	if len(data) < 5 {
		return -1
	}
	if !fuzz.IsDivisibleBy(len(data), 2) {
		return -1
	}
	half := len(data) / 2
	firstHalf := data[:half]
	f1 := fuzz.NewConsumer(firstHalf)
	err := f1.Split(3, 30)
	if err != nil {
		return -1
	}

	secondHalf := data[half:]
	f2 := fuzz.NewConsumer(secondHalf)
	err = f2.Split(3, 30)
	if err != nil {
		return -1
	}
	baseDir := "fuzz-base-dir"
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return -1
	}
	defer os.RemoveAll(baseDir) //nolint:errcheck

	var dirArray []string //nolint:prealloc
	iteration := 0
	chunkSize := len(f1.RestOfArray) / f1.NumberOfCalls
	for i := 0; i < len(f1.RestOfArray); i = i + chunkSize {
		from := i           // lower
		to := i + chunkSize // upper
		inputData := firstHalf[from:to]
		if len(inputData) > 6 && isLetter(inputData[:5]) {
			dirArray, err = createRandomDir(baseDir, inputData[:5], dirArray)
			if err != nil {
				continue
			}
		} else {
			if len(dirArray) == 0 {
				continue
			}
			dirp := int(inputData[0]) % len(dirArray)
			fp := filepath.Join(baseDir, dirArray[dirp])
			if len(inputData) > 10 {
				filename := inputData[5:8]
				err = createRandomFile(fp, filename, inputData[8:])
				if err != nil {
					continue
				}
			}
		}
		iteration++
	}

	// Get initial.
	initDh, err := mtree.Walk(baseDir, nil, append(mtree.DefaultKeywords, "sha256digest"), nil)
	if err != nil {
		return 0
	}
	iteration = 0
	chunkSize = len(f2.RestOfArray) / f2.NumberOfCalls
	for i := 0; i < len(f2.RestOfArray); i = i + chunkSize {
		from := i           // lower
		to := i + chunkSize // upper
		inputData := secondHalf[from:to]
		if len(inputData) > 6 && isLetter(inputData[:5]) {
			dirArray, err = createRandomDir(baseDir, inputData[:5], dirArray)
			if err != nil {
				continue
			}
		} else {
			if len(dirArray) == 0 {
				continue
			}
			dirp := int(inputData[0]) % len(dirArray)
			fp := filepath.Join(baseDir, dirArray[dirp])
			if len(inputData) > 10 {
				filename := inputData[5:8]
				err = createRandomFile(fp, filename, inputData[8:])
				if err != nil {
					continue
				}
			}
		}
		iteration++
	}

	// Get post.
	postDh, err := mtree.Walk(baseDir, nil, initDh.UsedKeywords(), nil)
	if err != nil {
		return 0
	}

	diffs, err := mtree.Compare(initDh, postDh, initDh.UsedKeywords())
	if err != nil {
		return -1
	}
	reader, err := GenerateLayer(baseDir, diffs, &RepackOptions{})
	if err != nil {
		return -1
	}
	defer reader.Close() //nolint:errcheck

	tr := tar.NewReader(reader)
	for {
		_, err = tr.Next()
		if err != nil {
			break
		}
	}
	return 1
}

func makeFuzzImage(base641, base642 string) (string, ispec.Manifest, casext.Engine, error) {
	ctx := context.Background()

	layers := []struct {
		base64 string
		digest digest.Digest
	}{
		{
			base64: base641,
			digest: digest.NewDigestFromHex(digest.SHA256.String(), "e489a16a8ca0d682394867ad8a8183f0a47cbad80b3134a83412a6796ad9242a"),
		},
		{
			base64: base642,
			digest: digest.NewDigestFromHex(digest.SHA256.String(), "39f100ed000b187ba74b3132cc207c63ad1765adaeb783aa7f242f1f7b6f5ea2"),
		},
	}

	root, err := os.MkdirTemp("", "umoci-TestUnpackManifestCustomLayer")
	if err != nil {
		return "nil", ispec.Manifest{}, casext.Engine{}, err
	}

	// Create our image.
	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		return "nil", ispec.Manifest{}, casext.Engine{}, err
	}
	engine, err := dir.Open(image)
	if err != nil {
		return "nil", ispec.Manifest{}, casext.Engine{}, err
	}
	engineExt := casext.NewEngine(engine)

	// Set up the CAS and an image from the above layers.
	layerDigests := make([]digest.Digest, 0, len(layers))
	layerDescriptors := make([]ispec.Descriptor, 0, len(layers))
	for _, layer := range layers {
		layerData, _ := base64.StdEncoding.DecodeString(layer.base64)
		layerReader := bytes.NewBuffer(layerData)
		layerDigest, layerSize, err := engineExt.PutBlob(ctx, layerReader)
		if err != nil {
			return "nil", ispec.Manifest{}, casext.Engine{}, err
		}

		layerDigests = append(layerDigests, layer.digest)
		layerDescriptors = append(layerDescriptors, ispec.Descriptor{
			MediaType: ispec.MediaTypeImageLayerGzip,
			Digest:    layerDigest,
			Size:      layerSize,
		})
	}

	// Create the config.
	config := ispec.Image{
		Platform: ispec.Platform{
			OS: "linux",
		},
		RootFS: ispec.RootFS{
			Type:    "layers",
			DiffIDs: layerDigests,
		},
	}
	configDigest, configSize, err := engineExt.PutBlobJSON(ctx, config)
	if err != nil {
		return "nil", ispec.Manifest{}, casext.Engine{}, err
	}
	configDescriptor := ispec.Descriptor{
		MediaType: ispec.MediaTypeImageConfig,
		Digest:    configDigest,
		Size:      configSize,
	}

	// Create the manifest.
	manifest := ispec.Manifest{
		Versioned: specs.Versioned{
			SchemaVersion: 2,
		},
		MediaType: ispec.MediaTypeImageManifest,
		Config:    configDescriptor,
		Layers:    layerDescriptors,
	}

	return root, manifest, engineExt, nil
}

// FuzzUnpack implements a fuzzer that targets UnpackManifest().
func FuzzUnpack(data []byte) int {
	// We would like as little log output as possible:
	level, err := log.ParseLevel("fatal")
	if err != nil {
		return -1
	}
	log.SetLevel(level)
	ctx := context.Background()
	c := fuzz.NewConsumer(data)
	base641, err := c.GetString()
	if err != nil {
		return -1
	}

	base642, err := c.GetString()
	if err != nil {
		return -1
	}
	root, manifest, engineExt, err := makeFuzzImage(base641, base642)
	if err != nil {
		return 0
	}
	defer os.RemoveAll(root) //nolint:errcheck

	bundle, err := os.MkdirTemp("", "umoci-TestUnpackManifestCustomLayer_bundle")
	if err != nil {
		return 0
	}
	defer os.RemoveAll(bundle) //nolint:errcheck

	unpackOptions := &UnpackOptions{OnDiskFormat: DirRootfs{
		MapOptions: MapOptions{
			UIDMappings: []rspec.LinuxIDMapping{
				{HostID: uint32(os.Geteuid()), ContainerID: 0, Size: 1},
				{HostID: uint32(os.Geteuid()), ContainerID: 1000, Size: 1},
			},
			GIDMappings: []rspec.LinuxIDMapping{
				{HostID: uint32(os.Getegid()), ContainerID: 0, Size: 1},
				{HostID: uint32(os.Getegid()), ContainerID: 100, Size: 1},
			},
			Rootless: os.Geteuid() != 0,
		},
	}}

	unpackOptions.AfterLayerUnpack = func(ispec.Manifest, ispec.Descriptor) error {
		return nil
	}

	_ = UnpackManifest(ctx, engineExt, bundle, manifest, unpackOptions)

	return 1
}
