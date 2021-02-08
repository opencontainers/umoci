/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016-2020 SUSE LLC
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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"

	"github.com/apex/log"
	"github.com/bloodorangeio/reggie"
	godigest "github.com/opencontainers/go-digest"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// a.k.a. "pull"
func copyRemoteToLocal(ctx *cli.Context, remote *parsedRemoteReference, local *parsedLocalReference) error {
	client, err := newRegistryClient(ctx, remote)
	if err != nil {
		return err
	}

	// Get a reference to the CAS.
	dir.Create(local.dir) // TODO: handle this error?
	engine, err := dir.Open(local.dir)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()
	engineContext := context.Background()

	log.Infof("Checking if manifest available in registry")
	req := client.NewRequest(reggie.HEAD, "/v2/<name>/manifests/<reference>",
		reggie.WithReference(remote.tag)).
		SetHeader("Accept", v1.MediaTypeImageManifest)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	expectedContentDigest := resp.Header().Get(contentDigestHeader)
	log.Infof("Registry reports manifest with digest %s", expectedContentDigest)
	parsedDigest, err := godigest.Parse(expectedContentDigest)
	if err != nil {
		return err
	}

	// download manifest if it doesnt already exist is local store
	var manifestBytes []byte
	manifestReader, err := engine.GetBlob(engineContext, parsedDigest)
	defer manifestReader.Close()
	if err != nil {
		// TODO: better than this error check
		if !strings.Contains(err.Error(), "no such file") {
			return err
		}
		// Fetch the raw manifest from registry and validate its digest
		log.Infof("Downloading manifest from registry")
		req = client.NewRequest(reggie.GET, "/v2/<name>/manifests/<reference>",
			reggie.WithReference(remote.tag)).
			SetHeader("Accept", v1.MediaTypeImageManifest)
		resp, err = client.Do(req)
		if err != nil {
			return err
		}
		if h := resp.Header().Get(contentDigestHeader); h != expectedContentDigest {
			return errors.New(
				fmt.Sprintf("Possible MITM attack: the %s header was %s on manifest HEAD, but %s on manifest GET",
					contentDigestHeader, expectedContentDigest, h))
		}
		actualContentDigest := godigest.FromBytes(resp.Body()).String()
		if actualContentDigest != expectedContentDigest {
			return errors.New(
				fmt.Sprintf("Possible MITM attack: the real digest of the downloaded manifest was %s",
					actualContentDigest))
		}
		log.Debugf("actual manifest digest matches expected digest (%s)", expectedContentDigest)

		// Note: only "application/vnd.oci.image.manifest.v1+json" supported for now
		mediaType := resp.Header().Get(manifestMediaTypeHeader)
		if mediaType != v1.MediaTypeImageManifest {
			return errors.New(
				fmt.Sprintf("Content-Type header for image manifest invalid: %s", mediaType))
		}

		d, s, err := engine.PutBlob(engineContext, bytes.NewReader(resp.Body()))
		if err != nil {
			return err
		}
		log.Debugf("blob saved to local store, digest: %s, size: %d", d, s)

		manifestBytes = resp.Body()
	} else {
		// load directly from CAS
		log.Infof("Manifest with digest %s already exists in local store", expectedContentDigest)
		manifestBytes, err = ioutil.ReadAll(manifestReader)
		if err != nil {
			return err
		}
	}

	// Parse into OCI Manifest
	var manifest v1.Manifest
	err = json.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return err
	}

	// Parse into OCI Descriptor
	var descriptor v1.Descriptor
	err = json.Unmarshal(manifestBytes, &descriptor)
	if err != nil {
		return err
	}
	descriptor.MediaType = v1.MediaTypeImageManifest
	descriptor.Digest = parsedDigest
	descriptor.Size = int64(len(manifestBytes))

	// Patiently and synchronously fetch layer blobs from registry, verify, and store them
	numLayers := len(manifest.Layers)
	log.Infof("Manifest layer list contains %d item(s)", numLayers)
	for i, layer := range manifest.Layers {
		layerDigest := layer.Digest
		layerReader, err := engine.GetBlob(engineContext, layerDigest)
		if err != nil {
			if !strings.Contains(err.Error(), "no such file") {
				return err
			}
			log.Infof("Copying layer %d/%d with digest %s", i+1, numLayers, layerDigest)
			req = client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>",
				reggie.WithDigest(layerDigest.String()))
			resp, err = client.Do(req)
			if err != nil {
				return err
			}

			if d := godigest.FromBytes(resp.Body()).String(); d != layerDigest.String() {
				return errors.New(
					fmt.Sprintf("Possible MITM attack: the real digest of the downloaded layer was %s", d))
			}
			log.Debugf("actual layer digest matches expected digest (%s)", layerDigest)

			d, s, err := engine.PutBlob(engineContext, bytes.NewReader(resp.Body()))
			if err != nil {
				return err
			}
			log.Debugf("blob saved to local store, digest: %s, size: %d", d, s)
			continue
		}
		layerReader.Close()
		log.Infof("Layer %d/%d with digest %s already exists in local store", i+1, numLayers, layerDigest)
	}

	// Fetch config blob if exists
	if manifest.Config.Size > 0 {
		configDigest := manifest.Config.Digest
		configReader, err := engine.GetBlob(engineContext, configDigest)
		defer configReader.Close()
		if err != nil {
			if !strings.Contains(err.Error(), "no such file") {
				return err
			}
			log.Infof("Copying config with digest %s", configDigest)
			req = client.NewRequest(reggie.GET, "/v2/<name>/blobs/<digest>",
				reggie.WithDigest(configDigest.String()))
			resp, err = client.Do(req)
			if err != nil {
				return err
			}

			if d := godigest.FromBytes(resp.Body()).String(); d != configDigest.String() {
				return errors.New(
					fmt.Sprintf("Possible MITM attack: the real digest of the downloaded config was %s", d))
			}
			log.Debugf("actual config digest matches expected digest (%s)", configDigest)

			d, s, err := engine.PutBlob(engineContext, bytes.NewReader(resp.Body()))
			if err != nil {
				return err
			}
			log.Debugf("blob saved to local store, digest: %s, size: %d", d, s)
		} else {
			log.Infof("Config with digest %s already exists in local store", configDigest)
		}
	}

	// Add reference to index
	log.Infof("Saving reference '%s' to index in %s", local.tag, local.dir)
	err = engineExt.UpdateReference(engineContext, local.tag, descriptor)
	if err != nil {
		return err
	}

	return nil
}
