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

package main

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/apex/log"
	"github.com/urfave/cli"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/pkg/fseval"
)

func parseMtreeKeywordArg(arg string) []mtree.Keyword {
	names := strings.FieldsFunc(arg, func(ch rune) bool { return ch == ',' || ch == ' ' })
	keywords := make([]mtree.Keyword, 0, len(names))
	for _, name := range names {
		keywords = append(keywords, mtree.KeywordSynonym(name))
	}
	return keywords
}

func cmdParseMtreeKeywords(ctx *cli.Context, name string) {
	var keywords []mtree.Keyword
	if arg := ctx.String(name); ctx.IsSet(name) {
		keywords = parseMtreeKeywordArg(arg)
	}
	ctx.App.Metadata["--"+name] = keywords
}

func uxMtreeKeyword(cmd cli.Command) cli.Command {
	cmd.Flags = append(cmd.Flags, []cli.Flag{
		cli.StringFlag{
			Name:  "add-keywords,K", // for compatibility with gomtree
			Usage: "add the specified keywords to the set used for validation (delimited by comma or space)",
		},
		cli.StringFlag{
			Name:  "use-keywords,k", // for compatibility with gomtree
			Usage: "use only the specified keywords for validation (delimited by comma or space)",
		},
		cli.BoolFlag{
			Name:  "umoci-keywords",
			Usage: "use the default set of keywords used by umoci",
		},
	}...)

	oldBefore := cmd.Before
	cmd.Before = func(ctx *cli.Context) error {
		cmdParseMtreeKeywords(ctx, "add-keywords")
		cmdParseMtreeKeywords(ctx, "use-keywords")
		if oldBefore != nil {
			return oldBefore(ctx)
		}
		return nil
	}

	return cmd
}

var rawMtreeValidateCommand = uxMtreeKeyword(uxRootless(cli.Command{
	Name:  "mtree-validate",
	Usage: `validate an mtree manifest (akin to "go-mtree validate")`,
	ArgsUsage: `--manifest <manifest.mtree> --path <directory>

Validate "<directory>" against the mtree(8) manifest in "<manifest.mtree>".

This tool is primarily intended for umoci's integration tests (go-mtree is
missing rootless support in its CLI), and should not be relied upon by users.`,

	// This is only really used for our tests.
	Category: "devtools",
	Hidden:   true,

	Flags: []cli.Flag{
		cli.StringFlag{
			Name:     "manifest,f,file", // for compatibility with gomtree
			Required: true,
			Usage:    "mtree manifest to validate",
		},
		cli.StringFlag{
			Name:     "path,p", // for compatibility with gomtree
			Required: true,
			Usage:    "root path the the mtree manifest is relative to",
		},
		// TODO: Do we need --directory-only / -d ?
	},

	Action: rawMtreeValidate,

	Before: func(ctx *cli.Context) error {
		if ctx.String("manifest") == "" {
			return errors.New("--manifest must be a valid path")
		}
		if ctx.String("path") == "" {
			return errors.New("--path must be a valid path")
		}

		fsEval := fseval.Default
		if ctx.Bool("rootless") {
			fsEval = fseval.Rootless
		}
		ctx.App.Metadata["fseval"] = fsEval

		return nil
	},
}))

func rawMtreeValidate(ctx *cli.Context) (Err error) {
	manifestPath := ctx.String("manifest")
	rootPath := ctx.String("path")
	fsEval := mustFetchMeta[fseval.FsEval](ctx, "fseval")

	addKeywords := mustFetchMeta[[]mtree.Keyword](ctx, "--add-keywords")
	useKeywords := mustFetchMeta[[]mtree.Keyword](ctx, "--use-keywords")

	log.WithFields(log.Fields{
		"manifest": manifestPath,
		"path":     rootPath,
	}).Debugf("validating directory against mtree manifest")

	log.WithFields(log.Fields{
		"manifest": manifestPath,
	}).Info("parsing mtree manifest ...")
	manifestFile, err := os.Open(manifestPath)
	if err != nil {
		return fmt.Errorf("open mtree manifest: %w", err)
	}
	defer manifestFile.Close() //nolint:errcheck
	manifest, err := mtree.ParseSpec(manifestFile)
	if err != nil {
		return fmt.Errorf("parse mtree manifest: %w", err)
	}
	log.Info("... done")

	// Figure out the set of keywords to use for the comparison.
	var (
		compareKeywords  = useKeywords
		manifestKeywords = manifest.UsedKeywords()
	)
	if compareKeywords == nil {
		if ctx.Bool("umoci-keywords") {
			compareKeywords = umoci.MtreeKeywords[:]
		} else {
			// Just use the manifest keywords if none were specified.
			compareKeywords = manifestKeywords
		}
	}
	for _, kw := range addKeywords {
		if !mtree.InKeywordSlice(kw, compareKeywords) {
			compareKeywords = append(compareKeywords, kw)
		}
	}
	// "type" is necessary for any comparison to make sense.
	if !mtree.InKeywordSlice("type", compareKeywords) {
		compareKeywords = append([]mtree.Keyword{"type"}, compareKeywords...)
	}
	// NOTE: compareKeywords might contain keywords not in the manifest, but
	// this is usually okay.
	log.WithFields(log.Fields{
		"manifest_keywords": manifestKeywords,
		"keywords":          compareKeywords,
	}).Debug("computed set of keywords for mtree comparison")

	log.WithFields(log.Fields{
		"path": rootPath,
	}).Info("computing filesystem diff against manifest ...")
	diff, err := mtree.Check(rootPath, manifest, compareKeywords, fsEval)
	if err != nil {
		return fmt.Errorf("check mtree manifest %q against root %q: %w", manifestPath, rootPath, err)
	}
	log.Info("... done")

	for _, delta := range diff {
		fmt.Println(delta)
	}
	if n := len(diff); n > 0 {
		return fmt.Errorf("validation failed: %d differences found", n)
	}
	log.Infof("no errors found during mtree validation")
	return nil
}
