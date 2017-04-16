/*
 * umoci: Umoci Modifies Open Containers' Images
 * Copyright (C) 2017 SUSE LLC.
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
	"github.com/apex/log"
	"github.com/openSUSE/umoci/oci/cas"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"golang.org/x/net/context"
)

// XXX: This is a temporary implementation that will be replaced with the move
//      to 1.0.0-rc5. You really shouldn't hit any bugs on this commit.

// ResolveReference is a wrapper around GetReference.
func (e Engine) ResolveReference(ctx context.Context, refname string) ([]ispec.Descriptor, error) {
	descriptor, err := e.GetReference(ctx, refname)
	if err != nil {
		return nil, err
	}
	// XXX: This is wrong for ImageManifests. However,
	return []ispec.Descriptor{descriptor}, nil
}

// UpdateReference is just a wrapper around PutReference.
func (e Engine) UpdateReference(ctx context.Context, refname string, descriptor ispec.Descriptor) error {
	err := e.PutReference(ctx, refname, descriptor)
	// TODO: This won't ever be hit with v1.0.0-rc5.
	if err == cas.ErrClobber {
		// We have to clobber a tag.
		log.Warnf("clobbering existing tag: %s", refname)

		// Delete the old tag.
		if err := e.DeleteReference(ctx, refname); err != nil {
			return errors.Wrap(err, "delete old tag")
		}
		err = e.PutReference(ctx, refname, descriptor)
	}
	return err
}

// AddReferences is just a wrapper around PutReference.
func (e Engine) AddReferences(ctx context.Context, refname string, descriptors ...ispec.Descriptor) error {
	if len(descriptors) != 1 {
		// XXX: This is not supported by v1.0.0-rc4 of the image-spec.
		return errors.Errorf("can only have a single descriptor for each refname")
	}
	return e.UpdateReference(ctx, refname, descriptors[0])
}

// DeleteReference is just a wrapper around DeleteReference.
func (e Engine) DeleteReference(ctx context.Context, refname string) error {
	return e.Engine.DeleteReference(ctx, refname)
}

// ListReferences is just a wrapper around ListReferences.
func (e Engine) ListReferences(ctx context.Context) ([]string, error) {
	return e.Engine.ListReferences(ctx)
}
