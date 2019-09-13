/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017, 2018 SUSE LLC.
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

package casext

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"sync"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// BlobParseFunc is a callback that is registered for a given mediatype and
// called to parse a blob if it is encountered. If possible, the blob should be
// represented as a native Go object (with all Descriptors represented as
// ispec.Descriptor objects) -- this will allow umoci to recursively discover
// blob dependencies.
type BlobParseFunc func(io.Reader) (interface{}, error)

var registered = struct {
	lock      sync.RWMutex
	callbacks map[string]BlobParseFunc
}{
	callbacks: map[string]BlobParseFunc{
		ispec.MediaTypeDescriptor: func(reader io.Reader) (interface{}, error) {
			var ret ispec.Descriptor
			err := json.NewDecoder(reader).Decode(&ret)
			return ret, err
		},

		ispec.MediaTypeImageManifest: func(reader io.Reader) (interface{}, error) {
			var ret ispec.Manifest
			err := json.NewDecoder(reader).Decode(&ret)
			return ret, err
		},

		ispec.MediaTypeImageIndex: func(reader io.Reader) (interface{}, error) {
			var ret ispec.Index
			err := json.NewDecoder(reader).Decode(&ret)
			return ret, err
		},

		ispec.MediaTypeImageConfig: func(reader io.Reader) (interface{}, error) {
			var ret ispec.Image
			err := json.NewDecoder(reader).Decode(&ret)
			return ret, err
		},
	},
}

func getParser(mediaType string) BlobParseFunc {
	registered.lock.RLock()
	fn := registered.callbacks[mediaType]
	registered.lock.RUnlock()
	return fn
}

// RegisterBlobParser registers a new BlobParseFunc to be used when the given
// mediatype is encountered during parsing or recursive walks of blobs. See the
// documentation of BlobParseFunc for more detail.
func RegisterBlobParser(mediaType string, callback BlobParseFunc) {
	registered.lock.Lock()
	registered.callbacks[mediaType] = callback
	registered.lock.Unlock()
}

// Blob represents a "parsed" blob in an OCI image's blob store. MediaType
// offers a type-safe way of checking what the type of Data is.
type Blob struct {
	// Descriptor is the {mediatype,digest,length} 3-tuple. Note that this
	// isn't updated if the Data is modified.
	Descriptor ispec.Descriptor

	// Data is the "parsed" blob taken from the OCI image's blob store, and is
	// typed according to the media type. The default mappings from MIME =>
	// type is as follows (more can be registered using RegisterParser).
	//
	// ispec.MediaTypeDescriptor => ispec.Descriptor
	// ispec.MediaTypeImageManifest => ispec.Manifest
	// ispec.MediaTypeImageIndex => ispec.Index
	// ispec.MediaTypeImageLayer => io.ReadCloser
	// ispec.MediaTypeImageLayerGzip => io.ReadCloser
	// ispec.MediaTypeImageLayerNonDistributable => io.ReadCloser
	// ispec.MediaTypeImageLayerNonDistributableGzip => io.ReadCloser
	// ispec.MediaTypeImageConfig => ispec.Image
	// unknown => io.ReadCloser
	Data interface{}
}

// Close cleans up all of the resources for the opened blob.
func (b *Blob) Close() error {
	if closer, ok := b.Data.(io.Closer); ok {
		return closer.Close()
	}
	return nil
}

// FromDescriptor parses the blob referenced by the given descriptor.
func (e Engine) FromDescriptor(ctx context.Context, descriptor ispec.Descriptor) (_ *Blob, Err error) {
	reader, err := e.GetVerifiedBlob(ctx, descriptor)
	if err != nil {
		return nil, errors.Wrap(err, "get blob")
	}

	blob := Blob{
		Descriptor: descriptor,
		Data:       reader,
	}

	if fn := getParser(descriptor.MediaType); fn != nil {
		defer func() {
			if _, err := io.Copy(ioutil.Discard, reader); Err == nil {
				Err = errors.Wrapf(err, "discard trailing %q blob", descriptor.MediaType)
			}
			if err := reader.Close(); Err == nil {
				Err = errors.Wrapf(err, "close %q blob", descriptor.MediaType)
			}
		}()

		data, err := fn(reader)
		if err != nil {
			return nil, errors.Wrapf(err, "parse %s", descriptor.MediaType)
		}
		blob.Data = data
	}
	if blob.Data == nil {
		return nil, errors.Errorf("[internal error] b.Data was nil after parsing")
	}
	return &blob, nil
}
