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

package casext

import (
	"context"
	"errors"
	"fmt"
	"io"

	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/oci/casext/mediatype"
	"github.com/opencontainers/umoci/pkg/system"
)

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
		return nil, fmt.Errorf("get blob: %w", err)
	}

	blob := Blob{
		Descriptor: descriptor,
		Data:       reader,
	}

	if fn := mediatype.GetParser(descriptor.MediaType); fn != nil {
		defer func() {
			if _, err := system.Copy(io.Discard, reader); Err == nil && err != nil {
				Err = fmt.Errorf("discard trailing %q blob: %w", descriptor.MediaType, err)
			}
			if err := reader.Close(); Err == nil && err != nil {
				Err = fmt.Errorf("close %q blob: %w", descriptor.MediaType, err)
			}
		}()

		data, err := fn(reader)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", descriptor.MediaType, err)
		}
		blob.Data = data
	}
	if blob.Data == nil {
		return nil, errors.New("[internal error] b.Data was nil after parsing")
	}
	return &blob, nil
}
