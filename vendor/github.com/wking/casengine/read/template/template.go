// Copyright 2017 casengine contributors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package template implements the OCI CAS Template Protocol v1.
// https://github.com/xiekeyang/oci-discovery/blob/0be7eae246ae9a975a76ca209c045043f0793572/cas-template.md
package template

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/jtacoma/uritemplates"
	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/wking/casengine"
	"github.com/wking/casengine/read"
	"golang.org/x/net/context"
)

// Engine implements the OCI CAS Template Protocol v1.
type Engine struct {
	uri  *uritemplates.UriTemplate
	base *url.URL

	// Client allows callers to configure the HTTP client.  Get will use
	// http.DefaultClient if Client is not set.  You can set this
	// property with:
	//
	//   engine, err := New(ctx, nil, config)
	//   // handle err and possibly engine.Close(ctx)
	//   engine.(*Engine).Client = yourCustomClient
	Client *http.Client
}

// New creates a new CAS-engine instance.
func New(ctx context.Context, baseURI *url.URL, config interface{}) (engine casengine.ReadCloser, err error) {
	configMap, ok := config.(map[string]string)
	if !ok {
		configMap2, ok := config.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("CAS-template config is not a map[string]string: %v", config)
		}
		uriInterface, ok := configMap2["uri"]
		if !ok {
			return nil, fmt.Errorf("CAS-template config missing required 'uri' property: %v", configMap)
		}
		configMap = make(map[string]string)
		configMap["uri"], ok = uriInterface.(string)
		if !ok {
			return nil, fmt.Errorf("CAS-template config 'uri' is not a string: %v", uriInterface)
		}
	}

	uriString, ok := configMap["uri"]
	if !ok {
		return nil, fmt.Errorf("CAS-template config missing required 'uri' property: %v", configMap)
	}

	uriTemplate, err := uritemplates.Parse(uriString)
	if err != nil {
		return nil, err
	}

	return &Engine{
		uri:  uriTemplate,
		base: baseURI,
	}, nil
}

// Get returns a reader for retrieving a blob from the store.
func (engine *Engine) Get(ctx context.Context, digest digest.Digest) (reader io.ReadCloser, err error) {
	request, err := engine.getPreFetch(digest)
	if err != nil {
		return nil, err
	}
	request = request.WithContext(ctx)

	client := engine.Client
	if client == nil {
		client = http.DefaultClient
	}
	logrus.Debugf("requesting %s from %s", digest, request.URL)
	response, err := client.Do(request)
	if err != nil {
		return nil, err
	}

	return engine.getPostFetch(response, digest)
}

// Close releases resources held by the engine.
func (engine *Engine) Close(ctx context.Context) (err error) {
	return nil
}

// URI returns the expanded, resolved URI for digest.
func (engine *Engine) URI(digest digest.Digest) (uri *url.URL, err error) {
	values := map[string]interface{}{
		"digest":    string(digest),
		"algorithm": string(digest.Algorithm()),
		"encoded":   digest.Encoded(),
	}

	referenceURI, err := engine.uri.Expand(values)
	if err != nil {
		return nil, err
	}

	parsedReference, err := url.Parse(referenceURI)
	if err != nil {
		return nil, err
	}

	if !parsedReference.IsAbs() && engine.base == nil {
		return nil, fmt.Errorf("cannot resolve relative %s without a base engine URI", parsedReference)
	}

	return engine.base.ResolveReference(parsedReference), nil
}

func (engine *Engine) getPreFetch(digest digest.Digest) (request *http.Request, err error) {
	uri, err := engine.URI(digest)
	if err != nil {
		return nil, err
	}

	return &http.Request{
		Method: "GET",
		URL:    uri,
	}, nil
}

func (engine *Engine) getPostFetch(response *http.Response, digest digest.Digest) (reader io.ReadCloser, err error) {
	defer func() {
		if err != nil {
			response.Body.Close()
		}
	}()

	if response.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}

	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("requested %s but got %s", response.Request.URL, response.Status)
	}

	return response.Body, nil
}

func init() {
	read.Constructors["oci-cas-template-v1"] = New
}
