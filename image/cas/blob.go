/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016 SUSE LLC.
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

package cas

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context"
)

// Blob represents a "parsed" blob in an OCI image's blob store. MediaType
// offers a type-safe way of checking what the type of Data is.
type Blob struct {
	// MediaType is the OCI media type of Data.
	MediaType string

	// Digest is the digest of the parsed image. Note that this does not update
	// if Data is changed (it is the digest that this blob was parsed *from*).
	Digest string

	// Data is the "parsed" blob taken from the OCI image's blob store, and is
	// typed according to the media type. The mapping from MIME => type is as
	// follows.
	//
	// v1.MediaTypeDescriptor => *v1.Descriptor
	// v1.MediaTypeImageManifest => *v1.Manifest
	// v1.MediaTypeImageManifestList => *v1.ManifestList
	// v1.MediaTypeImageLayer => io.ReadCloser
	// v1.MediaTypeImageLayerNonDistributable => io.ReadCloser
	// v1.MediaTypeImageConfig => *v1.Image
	Data interface{}
}

func (b *Blob) load(ctx context.Context, engine Engine) error {
	reader, err := engine.GetBlob(ctx, b.Digest)
	if err != nil {
		return err
	}

	// The layer media types are special, we don't want to do any parsing (or
	// close the blob reference).
	switch b.MediaType {
	// v1.MediaTypeImageLayer => io.ReadCloser
	// v1.MediaTypeImageLayerNonDistributable => io.ReadCloser
	case v1.MediaTypeImageLayer, v1.MediaTypeImageLayerNonDistributable:
		// There isn't anything else we can practically do here.
		b.Data = reader
		return nil
	}

	defer reader.Close()

	var parsed interface{}
	switch b.MediaType {
	// v1.MediaTypeDescriptor => *v1.Descriptor
	case v1.MediaTypeDescriptor:
		parsed = &v1.Descriptor{}
	// v1.MediaTypeImageManifest => *v1.Manifest
	case v1.MediaTypeImageManifest:
		parsed = &v1.Manifest{}
	// v1.MediaTypeImageManifestList => *v1.ManifestList
	case v1.MediaTypeImageManifestList:
		parsed = &v1.ManifestList{}
	// v1.MediaTypeImageConfig => *v1.ImageConfig
	case v1.MediaTypeImageConfig:
		parsed = &v1.Image{}
	default:
		return fmt.Errorf("cas blob: unsupported mediatype: %s", b.MediaType)
	}

	if err := json.NewDecoder(reader).Decode(parsed); err != nil {
		return err
	}

	b.Data = parsed
	return nil
}

// Close cleans up all of the resources for the opened blob.
func (b *Blob) Close() {
	switch b.MediaType {
	case v1.MediaTypeImageLayer, v1.MediaTypeImageLayerNonDistributable:
		if b.Data != nil {
			b.Data.(io.Closer).Close()
		}
	}
}

// FromDescriptor parses the blob referenced by the given descriptor.
func FromDescriptor(ctx context.Context, engine Engine, descriptor *v1.Descriptor) (*Blob, error) {
	blob := &Blob{
		MediaType: descriptor.MediaType,
		Digest:    descriptor.Digest,
		Data:      nil,
	}

	if err := blob.load(ctx, engine); err != nil {
		return nil, err
	}

	return blob, nil
}
