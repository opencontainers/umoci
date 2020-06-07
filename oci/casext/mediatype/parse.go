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

package mediatype

import (
	"encoding/json"
	"io"
	"reflect"
	"sync"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// ParseFunc is a parser that is registered for a given mediatype and called
// to parse a blob if it is encountered. If possible, the blob should be
// represented as a native Go object (with all Descriptors represented as
// ispec.Descriptor objects) -- this will allow umoci to recursively discover
// blob dependencies.
//
// Currently, we require the returned interface{} to be a raw struct
// (unexpected behaviour may occur otherwise).
//
// NOTE: Your ParseFunc must be able to accept a nil Reader (the error
//       value is not relevant). This is used during registration in order to
//       determine the type of the struct (thus you must return a struct that
//       you would return in a non-nil reader scenario). Go doesn't have a way
//       for us to enforce this.
type ParseFunc func(io.Reader) (interface{}, error)

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
// the documentation of ParseFunc for more detail.
func RegisterParser(mediaType string, parser ParseFunc) {
	// Get the return type so we know what packages are white-listed for
	// recursion. #nosec G104
	v, _ := parser(nil)
	t := reflect.TypeOf(v)

	// Register the parser and package.
	lock.Lock()
	_, old := parsers[mediaType]
	parsers[mediaType] = parser
	packages[t.PkgPath()] = struct{}{}
	lock.Unlock()

	// This should never happen, and is a programmer bug.
	if old {
		panic("RegisterParser() called with already-registered media-type: " + mediaType)
	}
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

// CustomJSONParser creates a custom ParseFunc which JSON-decodes blob data
// into the type of the given value (which *must* be a struct, otherwise
// CustomJSONParser will panic). This is intended to make ergonomic use of
// RegisterParser much simpler.
func CustomJSONParser(v interface{}) ParseFunc {
	t := reflect.TypeOf(v)
	// These should never happen and are programmer bugs.
	if t == nil {
		panic("CustomJSONParser() called with nil interface!")
	}
	if t.Kind() != reflect.Struct {
		panic("CustomJSONParser() called with non-struct kind!")
	}
	return func(reader io.Reader) (_ interface{}, err error) {
		ptr := reflect.New(t)
		if reader != nil {
			err = json.NewDecoder(reader).Decode(ptr.Interface())
		}
		ret := reflect.Indirect(ptr)
		return ret.Interface(), err
	}
}

// Register the core image-spec types.
func init() {
	RegisterParser(ispec.MediaTypeDescriptor, CustomJSONParser(ispec.Descriptor{}))
	RegisterParser(ispec.MediaTypeImageIndex, CustomJSONParser(ispec.Index{}))
	RegisterParser(ispec.MediaTypeImageConfig, CustomJSONParser(ispec.Image{}))

	RegisterTarget(ispec.MediaTypeImageManifest)
	RegisterParser(ispec.MediaTypeImageManifest, CustomJSONParser(ispec.Manifest{}))
}
