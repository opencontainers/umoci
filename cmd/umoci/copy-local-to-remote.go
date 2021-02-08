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
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/apex/log"
	"github.com/bloodorangeio/reggie"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

// a.k.a. "push"
func copyLocalToRemote(ctx *cli.Context, local *parsedLocalReference, remote *parsedRemoteReference) error {
	client, err := newRegistryClient(ctx, remote)
	if err != nil {
		return err
	}

	engine, err := dir.Open(local.dir)
	if err != nil {
		return errors.Wrap(err, "open CAS")
	}
	engineExt := casext.NewEngine(engine)
	defer engine.Close()
	engineContext := context.Background()

	descriptorPaths, err := engineExt.ResolveReference(engineContext, local.tag)
	if err != nil {
		return err
	}

	// TODO: in what scenario would this length be greater than 1??
	numDescriptorPaths := len(descriptorPaths)
	if numDescriptorPaths == 0 {
		return errors.New(fmt.Sprintf("Reference '%s' not found in index", local.tag))
	} else if numDescriptorPaths > 1 {
		return errors.New(fmt.Sprintf("More than one entry for reference '%s' in index", local.tag))
	}

	manifestDescriptor := descriptorPaths[0].Descriptor()
	manifestDigest := manifestDescriptor.Digest
	log.Infof("Reference '%s' found in index, points to manifest %s", local.tag, manifestDigest)

	manifestReader, err := engine.GetBlob(engineContext, manifestDigest)
	defer manifestReader.Close()
	if err != nil {
		return err
	}

	manifestBytes, err := ioutil.ReadAll(manifestReader)
	if err != nil {
		return err
	}

	// Parse into OCI Manifest
	var manifest v1.Manifest
	err = json.Unmarshal(manifestBytes, &manifest)
	if err != nil {
		return err
	}
	log.Infof("Manifest successfully loaded from local store")

	// Upload layers
	numLayers := len(manifest.Layers)
	log.Infof("Manifest layer list contains %d item(s)", numLayers)
	for i, layer := range manifest.Layers {
		layerDigest := layer.Digest
		layerReader, err := engine.GetBlob(engineContext, layerDigest)
		defer layerReader.Close()
		if err != nil {
			return err
		}
		log.Infof("Uploading layer %d/%d with digest %s from local store", i+1, numLayers, layerDigest)

		// Create upload session
		req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		layerBytes, err := ioutil.ReadAll(layerReader)
		if err != nil {
			return err
		}

		// Monolithic upload
		// TODO: support chunked uploading
		req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
			SetQueryParam("digest", layerDigest.String()).
			SetHeader("Content-Type", "application/octet-stream").
			SetHeader("Content-Length", fmt.Sprint(layer.Size)).
			SetBody(layerBytes)
		resp, err = client.Do(req)
		if err != nil {
			return err
		}

		statusCode := resp.StatusCode()
		if statusCode != http.StatusCreated && statusCode != http.StatusAccepted {
			return errors.New("Registry did not return 201 or 202 on layer upload")
		}
	}

	// Upload config if present
	if manifest.Config.Size > 0 {
		configDigest := manifest.Config.Digest
		configReader, err := engine.GetBlob(engineContext, configDigest)
		defer configReader.Close()
		if err != nil {
			return err
		}
		log.Infof("Uploading config %s", configDigest)

		// Create upload session
		req := client.NewRequest(reggie.POST, "/v2/<name>/blobs/uploads/")
		resp, err := client.Do(req)
		if err != nil {
			return err
		}

		layerBytes, err := ioutil.ReadAll(configReader)
		if err != nil {
			return err
		}

		// Monolithic upload
		req = client.NewRequest(reggie.PUT, resp.GetRelativeLocation()).
			SetQueryParam("digest", configDigest.String()).
			SetHeader("Content-Type", "application/octet-stream").
			SetHeader("Content-Length", fmt.Sprint(manifest.Config.Size)).
			SetBody(layerBytes)
		resp, err = client.Do(req)
		if err != nil {
			return err
		}

		statusCode := resp.StatusCode()
		if statusCode != http.StatusCreated && statusCode != http.StatusAccepted {
			return errors.New("Registry did not return 201 or 202 on config upload")
		}
	}

	// upload manifest
	req := client.NewRequest(reggie.PUT, "/v2/<name>/manifests/<reference>",
		reggie.WithReference(remote.tag)).
		SetHeader("Content-Type", v1.MediaTypeImageManifest).
		SetBody(manifestBytes)
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	statusCode := resp.StatusCode()
	if statusCode != http.StatusCreated && statusCode != http.StatusAccepted {
		return errors.New("Registry did not return 201 or 202 on manifest upload")
	}

	log.Infof("Successfully copied to remote %s", remote.host)
	return nil
}
