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
	"fmt"
	"net/url"
	"strings"

	"github.com/apex/log"
	"github.com/bloodorangeio/reggie"
	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/pkg/errors"
	"github.com/urfave/cli"
)

var copyCommand = cli.Command{
	Name:    "copy",
	Aliases: []string{"cp"},
	Usage:   "copy an image into OCI layout",
	ArgsUsage: `--layout <image-path>

Where "<image-path>" is the path to the OCI layout.`,

	Before: func(ctx *cli.Context) error {
		if ctx.NArg() != 2 {
			return errors.Errorf("invalid number of positional arguments: expected <src> <dest>")
		}
		if ctx.Args().First() == "" {
			return errors.Errorf("src cannot be empty")
		}
		ctx.App.Metadata["src"] = ctx.Args().Get(0)
		if ctx.Args().First() == "" {
			return errors.Errorf("dest cannot be empty")
		}
		ctx.App.Metadata["dest"] = ctx.Args().Get(1)
		return nil
	},

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "username",
			Usage:    "authentication username",
			Required: false,
		},
		cli.StringFlag{
			Name:     "password",
			Usage:    "authentication password",
			Required: false,
		},
		cli.BoolFlag{
			Name:  "plain-http",
			Usage: "use plain HTTP for registry connection",
		},
		cli.BoolFlag{
			Name:  "trace-requests",
			Usage: "print detailed HTTP(s) logs from registry requests",
		},
	},

	Action: copy,
}

func copy(ctx *cli.Context) error {
	src := ctx.App.Metadata["src"].(string)
	dest := ctx.App.Metadata["dest"].(string)
	remote, err := parseRemoteReference(src)
	if err != nil {
		// Assume the args are flipped (remote to local vs. local to remote)
		if err == badRemoteURLError {
			remote, err = parseRemoteReference(dest)
			if err != nil {
				return err
			}
			local, err := parseLocalReference(src)
			if err != nil {
				return err
			}
			return copyLocalToRemote(ctx, local, remote)
		}
		return err
	}
	local, err := parseLocalReference(dest)
	if err != nil {
		return err
	}
	return copyRemoteToLocal(ctx, remote, local)
}

const (
	manifestMediaTypeHeader = "Content-Type"
	contentDigestHeader     = "Docker-Content-Digest"
)

var (
	badRemoteURLError = errors.New("remote URLs must be prefixed with oci://")
)

type parsedRemoteReference struct {
	host      string
	namespace string
	tag       string
}

type parsedLocalReference struct {
	dir string
	tag string
}

// parse something in the form of "oci://localhost:5000/opensuse:42.2"
func parseRemoteReference(raw string) (*parsedRemoteReference, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return nil, err
	}

	if u.Scheme != "oci" {
		return nil, badRemoteURLError
	}

	host := u.Host
	parts := strings.Split(u.Path, ":")
	namespace := strings.Trim(strings.Join(parts[0:len(parts)-1], ":"), "/")
	tag := parts[len(parts)-1]

	ref := parsedRemoteReference{
		host:      host,
		namespace: namespace,
		tag:       tag,
	}
	return &ref, nil
}

// parse something in the form "opensuse:42.2"
// TODO: copied/modified from utils_ux.go
func parseLocalReference(raw string) (*parsedLocalReference, error) {
	var dir, tag string
	sep := strings.Index(raw, ":")
	if sep == -1 {
		dir = raw
		tag = "latest"
	} else {
		dir = raw[:sep]
		tag = raw[sep+1:]
	}

	// Verify directory value.
	if dir == "" {
		return nil, errors.New("path is empty")
	}

	// Verify tag value.
	if !casext.IsValidReferenceName(tag) {
		return nil, errors.New(fmt.Sprintf("tag contains invalid characters: '%s'", tag))
	}
	if tag == "" {
		return nil, errors.New("tag is empty")
	}

	ref := parsedLocalReference{
		dir: dir,
		tag: tag,
	}
	return &ref, nil
}

// construct a registry client from context
func newRegistryClient(ctx *cli.Context, remote *parsedRemoteReference) (*reggie.Client, error) {
	scheme := "https"
	if ctx.Bool("plain-http") {
		scheme = "http"
	}
	registryAddress := scheme + "://" + remote.host
	log.Debugf("Registry address: %s, namespace: %s, tag: %s", registryAddress, remote.namespace, remote.tag)
	userAgent := fmt.Sprintf("umoci %s", umoci.FullVersion())
	username := ctx.String("username")
	password := ctx.String("password")
	traceRequests := ctx.Bool("trace-requests")
	return reggie.NewClient(registryAddress,
		reggie.WithDefaultName(remote.namespace),
		reggie.WithUserAgent(userAgent),
		reggie.WithUsernamePassword(username, password),
		reggie.WithDebug(traceRequests))
}
