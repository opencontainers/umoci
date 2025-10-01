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

package mediatype

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"sync"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/internal/assert"

	"github.com/opencontainers/umoci/internal"
)

// ErrNilReader is returned by the parsers in this package when they are called
// with a nil Reader. You may use this error for the same purpose if you wish,
// but it's not required.
var ErrNilReader = errors.New("")

// ParseFunc is a parser that is registered for a given mediatype and called
// to parse a blob if it is encountered. If possible, the blob should be
// represented as a native Go object (with all Descriptors represented as
// ispec.Descriptor objects) -- this will allow umoci to recursively discover
// blob dependencies.
//
// NOTE: Your ParseFunc must be able to accept a nil Reader (the error value is
// not relevant) and must return a struct. This is used during registration in
// order to determine the type of the struct (thus you must return the same
// struct you would return in a non-nil reader scenario). Go doesn't have a way
// for us to enforce this.
type ParseFunc func(io.Reader) (any, error)

var (
	lock sync.RWMutex

	// parsers is a mapping of media-type to parser function.
	parsers = map[string]ParseFunc{}

	// packages is the set of package paths which have been registered.
	packages = map[string]struct{}{}

	// targets is the set of media-types which are treated as "targets" for the
	// purposes of reference resolution (resolution terminates at these targets
	// as well as any un-parseable blob types).
	targets = map[string]struct{}{}
)

// IsRegisteredPackage returns whether a parser which returns a type from the
// given package path was registered. This is only useful to allow restricting
// reflection recursion (as a first-pass to limit how deep reflection goes).
func IsRegisteredPackage(pkgPath string) bool {
	lock.RLock()
	_, ok := packages[pkgPath]
	lock.RUnlock()
	return ok
}

// GetParser returns the ParseFunc that was previously registered for the given
// media-type with RegisterParser (or nil if the media-type is unknown).
func GetParser(mediaType string) ParseFunc {
	lock.RLock()
	fn := parsers[mediaType]
	lock.RUnlock()
	return fn
}

// RegisterParser registers a new ParseFunc to be used when the given
// media-type is encountered during parsing or recursive walks of blobs. See
// the documentation of ParseFunc for more detail. The returned ParseFunc must
// return a struct.
func RegisterParser(mediaType string, parser ParseFunc) {
	// Get the return type so we know what packages are white-listed for
	// recursion.
	v, _ := parser(nil)
	t := reflect.TypeOf(v)

	// Ensure the returned type is actually a struct. Ideally we would detect
	// this with generics but this seems to not be possible with Go generics
	// (circa Go 1.20).
	assert.Assert(t.Kind() == reflect.Struct, "parser given to RegisterParser doesn't return a struct{}") // programmer bug

	// Register the parser and package.
	lock.Lock()
	_, old := parsers[mediaType]
	parsers[mediaType] = parser
	packages[t.PkgPath()] = struct{}{}
	lock.Unlock()

	assert.Assertf(!old, "RegisterParser() called with already-registered media-type: %s", mediaType) // programmer bug
}

// IsTarget returns whether the given media-type should be treated as a "target
// media-type" for the purposes of reference resolution. This means that either
// the media-type has been registered as a target (using RegisterTarget) or has
// not been registered as parseable (using RegisterParser).
func IsTarget(mediaType string) bool {
	lock.RLock()
	_, isParseable := parsers[mediaType]
	_, isTarget := targets[mediaType]
	lock.RUnlock()
	return isTarget || !isParseable
}

// RegisterTarget registers that a given *parseable* media-type (meaning that
// there is a parser already registered using RegisterParser) should be treated
// as a "target" for the purposes of reference resolution. This means that if
// this media-type is encountered during a reference resolution walk, a
// DescriptorPath to *that* blob will be returned and resolution will not
// recurse any deeper. All un-parseable blobs are treated as targets, so this
// is only useful for blobs that have also been given parsers.
func RegisterTarget(mediaType string) {
	lock.Lock()
	targets[mediaType] = struct{}{}
	lock.Unlock()
}

// JSONParser is a minimal wrapper around
//
//	var v T
//	return json.NewDecoder(rdr).Decode(&v)
//
// But also handling the rdr == nil case which is needed for RegisterParser to
// correctly detect what type T is. T must be a struct (this is verified by
// RegisterParser).
func JSONParser[T any](rdr io.Reader) (_ any, err error) {
	var v T
	if rdr != nil {
		err = json.NewDecoder(rdr).Decode(&v)
	} else {
		// must not return a nil interface{}
		err = ErrNilReader
	}
	return v, err
}

func indexParser(rdr io.Reader) (any, error) {
	// Construct a fake struct which contains fields that shouldn't exist, to
	// detect images that have maliciously-inserted fields. CVE-2021-41190
	var index struct {
		ispec.Index
		Config json.RawMessage `json:"config,omitempty"`
		Layers json.RawMessage `json:"layers,omitempty"`
	}
	if rdr == nil {
		// must not return a nil interface{}
		return index, ErrNilReader
	}
	if err := json.NewDecoder(rdr).Decode(&index); err != nil {
		return nil, err
	}
	if index.MediaType != "" && index.MediaType != ispec.MediaTypeImageIndex {
		return nil, fmt.Errorf("malicious image detected: index contained incorrect mediaType: %s", index.MediaType)
	}
	if len(index.Config) != 0 {
		return nil, errors.New("malicious image detected: index contained forbidden 'config' field")
	}
	if len(index.Layers) != 0 {
		return nil, errors.New("malicious image detected: index contained forbidden 'layers' field")
	}
	return index.Index, nil
}

func manifestParser(rdr io.Reader) (any, error) {
	// Construct a fake struct which contains fields that shouldn't exist, to
	// detect images that have maliciously-inserted fields. CVE-2021-41190
	var manifest struct {
		ispec.Manifest
		Manifests json.RawMessage `json:"manifests,omitempty"`
	}
	if rdr == nil {
		// must not return a nil interface{}
		return manifest, ErrNilReader
	}
	if err := json.NewDecoder(rdr).Decode(&manifest); err != nil {
		return nil, err
	}
	if manifest.MediaType != "" && manifest.MediaType != ispec.MediaTypeImageManifest {
		return nil, fmt.Errorf("malicious manifest detected: manifest contained incorrect mediaType: %s", manifest.MediaType)
	}
	if len(manifest.Manifests) != 0 {
		return nil, errors.New("malicious manifest detected: manifest contained forbidden 'manifests' field")
	}
	return manifest.Manifest, nil
}

// emptyJSONParser only parses "application/vnd.oci.empty.v1+json" and
// validates that it is actually "{}".
func emptyJSONParser(rdr io.Reader) (any, error) {
	if rdr == nil {
		// must not return a nil interface{}
		return struct{}{}, ErrNilReader
	}

	// The only valid value for this blob.
	const emptyJSON = `{}`

	// Try to read at least one more byte than emptyJSON so if there is some
	// trailing data we will error out without needing to read any more.
	data, err := io.ReadAll(io.LimitReader(rdr, int64(len(emptyJSON))+1))
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(data, []byte(emptyJSON)) {
		return nil, internal.ErrInvalidEmptyJSON
	}
	return struct{}{}, nil
}

// Register the core image-spec types.
func init() {
	RegisterParser(ispec.MediaTypeDescriptor, JSONParser[ispec.Descriptor])
	RegisterParser(ispec.MediaTypeImageIndex, indexParser)
	RegisterParser(ispec.MediaTypeImageConfig, JSONParser[ispec.Image])
	RegisterParser(ispec.MediaTypeEmptyJSON, emptyJSONParser)

	RegisterTarget(ispec.MediaTypeImageManifest)
	RegisterParser(ispec.MediaTypeImageManifest, manifestParser)
}
