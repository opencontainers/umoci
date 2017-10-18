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

package dir

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/opencontainers/go-digest"
	"github.com/sirupsen/logrus"
	"github.com/wking/casengine"
	"golang.org/x/net/context"
)

// GetDigest calculates the digest corresponding to a given relative
// path.  This is effectively the inverse of URI Template expansion,
// and is required to support Digests.
type GetDigest func(path string) (digest digest.Digest, err error)

// RegexpGetDigest is a helper structure for regular-expression based
// GetDigest implementations.
type RegexpGetDigest struct {
	Regexp *regexp.Regexp
}

// DigestListerEngine is a CAS engine based on the local filesystem.
type DigestListerEngine struct {
	*Engine

	getDigest GetDigest
}

// GetDigest implements GetDigest for RegexpGetDigest.
func (r *RegexpGetDigest) GetDigest(path string) (dig digest.Digest, err error) {
	matches := make(map[string]string)
	submatches := r.Regexp.FindStringSubmatch(path)
	for i, submatchName := range r.Regexp.SubexpNames() {
		if submatchName == "" {
			continue
		}
		if i > len(submatches) {
			return "", fmt.Errorf("%q does not match %q", path, r.Regexp.String())
		}
		matches[submatchName] = submatches[i]
	}

	algorithm, ok := matches["algorithm"]
	if !ok {
		return "", fmt.Errorf("no 'algorithm' capturing group in %q", r.Regexp.String())
	}

	encoded, ok := matches["encoded"]
	if !ok {
		return "", fmt.Errorf("no 'encoded' capturing group in %q", r.Regexp.String())
	}

	return digest.Parse(fmt.Sprintf("%s:%s", algorithm, encoded))
}

// NewDigestListerEngine creates a new CAS-engine instance that can
// list the digests it contains.  Arguments are the same as for
// NewEngine, with an additional getDigest used to translate paths to
// digests.
func NewDigestListerEngine(ctx context.Context, path string, uri string, getDigest GetDigest) (engine casengine.DigestListerEngine, err error) {
	base, err := NewEngine(ctx, path, uri)
	if err != nil {
		return nil, err
	}

	return &DigestListerEngine{
		Engine:    base.(*Engine),
		getDigest: getDigest,
	}, nil
}

// Digests implements DigestLister.Digests.
func (engine *DigestListerEngine) Digests(ctx context.Context, algorithm digest.Algorithm, prefix string, size int, from int, callback casengine.DigestCallback) (err error) {
	if size == 0 {
		return nil
	}
	globAlgorithm := algorithm.String()
	if globAlgorithm == "" {
		globAlgorithm = "*"
	}
	globDigest := digest.Digest(fmt.Sprintf("%s:*", globAlgorithm))
	glob, err := engine.Engine.getPath(globDigest)
	if err != nil {
		return err
	}

	matches, err := filepath.Glob(glob)
	if err != nil {
		return err
	}

	offset := 0
	count := 0
	for _, match := range matches {
		digest, err := engine.getDigest(match)
		if err != nil {
			logrus.Warnf("cannot compute digest for %q (%s)", match, err)
			continue
		}

		if algorithm.String() == "" || digest.Algorithm() == algorithm {
			if prefix == "" || strings.HasPrefix(digest.Encoded(), prefix) {
				if offset >= from {
					err = callback(ctx, digest)
					if err != nil {
						return err
					}
					count++
					if size != -1 && count >= size {
						return nil
					}
				}
				offset++
			}
		}
	}
	return nil
}
