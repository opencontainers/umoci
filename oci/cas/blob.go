/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2016, 2017 SUSE LLC.
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

	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
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
	// ispec.MediaTypeDescriptor => *ispec.Descriptor
	// ispec.MediaTypeImageManifest => *ispec.Manifest
	// ispec.MediaTypeImageManifestList => *ispec.ManifestList
	// ispec.MediaTypeImageLayer => io.ReadCloser
	// ispec.MediaTypeImageLayerNonDistributable => io.ReadCloser
	// ispec.MediaTypeImageConfig => *ispec.Image
	Data interface{}
}

func (b *Blob) load(ctx context.Context, engine Engine) error {
	reader, err := engine.GetBlob(ctx, b.Digest)
	if err != nil {
		return errors.Wrap(err, "get blob")
	}

	// The layer media types are special, we don't want to do any parsing (or
	// close the blob reference).
	switch b.MediaType {
	// ispec.MediaTypeImageLayer => io.ReadCloser
	// ispec.MediaTypeImageLayerNonDistributable => io.ReadCloser
	case ispec.MediaTypeImageLayer, ispec.MediaTypeImageLayerNonDistributable:
		// There isn't anything else we can practically do here.
		b.Data = reader
		return nil
	}

	defer reader.Close()

	var parsed interface{}
	switch b.MediaType {
	// ispec.MediaTypeDescriptor => *ispec.Descriptor
	case ispec.MediaTypeDescriptor:
		parsed = &ispec.Descriptor{}
	// ispec.MediaTypeImageManifest => *ispec.Manifest
	case ispec.MediaTypeImageManifest:
		parsed = &ispec.Manifest{}
	// ispec.MediaTypeImageManifestList => *ispec.ManifestList
	case ispec.MediaTypeImageManifestList:
		parsed = &ispec.ManifestList{}
	// ispec.MediaTypeImageConfig => *ispec.ImageConfig
	case ispec.MediaTypeImageConfig:
		parsed = &ispec.Image{}
	default:
		return fmt.Errorf("cas blob: unsupported mediatype: %s", b.MediaType)
	}

	if err := json.NewDecoder(reader).Decode(parsed); err != nil {
		return errors.Wrap(err, "parse blob")
	}

	// TODO: We should really check that parsed.MediaType == b.MediaType.

	b.Data = parsed
	return nil
}

// Close cleans up all of the resources for the opened blob.
func (b *Blob) Close() {
	switch b.MediaType {
	case ispec.MediaTypeImageLayer, ispec.MediaTypeImageLayerNonDistributable:
		if b.Data != nil {
			b.Data.(io.Closer).Close()
		}
	}
}

// FromDescriptor parses the blob referenced by the given descriptor.
func FromDescriptor(ctx context.Context, engine Engine, descriptor *ispec.Descriptor) (*Blob, error) {
	blob := &Blob{
		MediaType: descriptor.MediaType,
		Digest:    descriptor.Digest,
		Data:      nil,
	}

	if err := blob.load(ctx, engine); err != nil {
		return nil, errors.Wrap(err, "load")
	}

	return blob, nil
}
