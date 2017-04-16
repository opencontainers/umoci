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

package casext

import (
	"errors"
	"reflect"

	"github.com/apex/log"
	"github.com/opencontainers/go-digest"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context"
)

// Used by walkState.mark() to determine which struct members are descriptors to
// recurse into them. We aren't interested in struct members which are not
// either a slice of ispec.Descriptor or ispec.Descriptor themselves.
var descriptorType = reflect.TypeOf(ispec.Descriptor{})

// isDescriptor returns whether the given T is a ispec.Descriptor.
func isDescriptor(T reflect.Type) bool {
	return T.AssignableTo(descriptorType) && descriptorType.AssignableTo(T)
}

// childDescriptors returns all child ispec.Descriptors given a particular
// interface{}. This is recursively evaluated, so if you have some cyclic
// struct pointer stuff going on things won't end well.
// FIXME: Should we implement this in a way that avoids cycle issues?
func childDescriptors(i interface{}) []ispec.Descriptor {
	V := reflect.ValueOf(i)
	log.WithFields(log.Fields{
		"V": V,
	}).Debugf("childDescriptors")
	if !V.IsValid() {
		// nil value
		return []ispec.Descriptor{}
	}

	// First check that V isn't actually a ispec.Descriptor.
	if isDescriptor(V.Type()) {
		return []ispec.Descriptor{V.Interface().(ispec.Descriptor)}
	}

	// Recurse into all the types.
	switch V.Kind() {
	case reflect.Ptr:
		// Just deref the pointer.
		log.WithFields(log.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into ptr")
		if V.IsNil() {
			return []ispec.Descriptor{}
		}
		return childDescriptors(V.Elem().Interface())

	case reflect.Array:
		// Convert to a slice.
		log.WithFields(log.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into array")
		return childDescriptors(V.Slice(0, V.Len()).Interface())

	case reflect.Slice:
		// Iterate over each element and append them to childDescriptors.
		children := []ispec.Descriptor{}
		for idx := 0; idx < V.Len(); idx++ {
			log.WithFields(log.Fields{
				"name": V.Type().PkgPath() + "::" + V.Type().Name(),
				"idx":  idx,
			}).Debugf("recursing into slice")
			children = append(children, childDescriptors(V.Index(idx).Interface())...)
		}
		return children

	case reflect.Struct:
		// We are only ever going to be interested in ispec.* types.
		if V.Type().PkgPath() != descriptorType.PkgPath() {
			log.WithFields(log.Fields{
				"name":   V.Type().PkgPath() + "::" + V.Type().Name(),
				"v1path": descriptorType.PkgPath(),
			}).Debugf("detected escape to outside ispec.* namespace")
			return []ispec.Descriptor{}
		}

		// We can now actually iterate through a struct to find all descriptors.
		children := []ispec.Descriptor{}
		for idx := 0; idx < V.NumField(); idx++ {
			log.WithFields(log.Fields{
				"name":  V.Type().PkgPath() + "::" + V.Type().Name(),
				"field": V.Type().Field(idx).Name,
			}).Debugf("recursing into struct")

			children = append(children, childDescriptors(V.Field(idx).Interface())...)
		}
		return children

	default:
		// FIXME: Should we log something here? While this will be hit normally
		//        (namely when we hit an io.ReadCloser) this seems a bit
		//        careless.
		return []ispec.Descriptor{}
	}

	// Unreachable.
}

// walkState stores state information about the recursion into a given
// descriptor tree.
type walkState struct {
	// engine is the CAS engine we are operating on.
	engine Engine

	// walkFunc is the WalkFunc provided by the user.
	walkFunc WalkFunc
}

// TODO: Also provide Blob to WalkFunc so that callers don't need to load blobs
//       more than once. This is quite important for remote CAS implementations.

// TODO: Move this and blob.go to a separate package.

// ErrSkipDescriptor is a special error returned by WalkFunc which will cause
// Walk to not recurse into the descriptor currently being evaluated by
// WalkFunc.  This interface is roughly equivalent to filepath.SkipDir.
var ErrSkipDescriptor = errors.New("[internal] do not recurse into descriptor")

// WalkFunc is the type of function passed to Walk. It will be a called on each
// descriptor encountered, recursively -- which may involve the function being
// called on the same descriptor multiple times (though because an OCI image is
// a Merkle tree there will never be any loops). If an error is returned by
// WalkFunc, the recursion will halt and the error will bubble up to the
// caller.
type WalkFunc func(descriptor ispec.Descriptor) error

func (ws *walkState) recurse(ctx context.Context, descriptor ispec.Descriptor) error {
	log.WithFields(log.Fields{
		"digest": descriptor.Digest,
	}).Debugf("-> ws.recurse")
	defer log.WithFields(log.Fields{
		"digest": descriptor.Digest,
	}).Debugf("<- ws.recurse")

	// Run walkFunc.
	if err := ws.walkFunc(descriptor); err != nil {
		if err == ErrSkipDescriptor {
			return nil
		}
		return err
	}

	// Get blob to recurse into.
	blob, err := ws.engine.FromDescriptor(ctx, descriptor)
	if err != nil {
		return err
	}
	defer blob.Close()

	// Recurse into children.
	for _, child := range childDescriptors(blob.Data) {
		if err := ws.recurse(ctx, child); err != nil {
			return err
		}
	}

	return nil
}

// Walk preforms a depth-first walk from a given root descriptor, using the
// provided CAS engine to fetch all other necessary descriptors. If an error is
// returned by the provided WalkFunc, walking is terminated and the error is
// returned to the caller.
func (e Engine) Walk(ctx context.Context, root ispec.Descriptor, walkFunc WalkFunc) error {
	ws := &walkState{
		engine:   e,
		walkFunc: walkFunc,
	}
	return ws.recurse(ctx, root)
}

// Paths returns the set of descriptors that can be traversed from the provided
// root descriptor. It is effectively shorthand for Walk(). Note that there may
// be repeated descriptors in the returned slice, due to different blobs
// containing the same (or a similar) descriptor.
func (e Engine) Paths(ctx context.Context, root ispec.Descriptor) ([]ispec.Descriptor, error) {
	var reachable []ispec.Descriptor

	err := e.Walk(ctx, root, func(descriptor ispec.Descriptor) error {
		reachable = append(reachable, descriptor)
		return nil
	})
	return reachable, err
}

// Reachable returns the set of digests which can be reached using a descriptor
// path from the provided root descriptor. It is effectively a shorthand for
// Walk(). The returned slice will *not* contain any duplicate digest.Digest
// entries. Note that without descriptors, a digest is not particularly
// meaninful (OCI blobs are not self-descriptive).
func (e Engine) Reachable(ctx context.Context, root ispec.Descriptor) ([]digest.Digest, error) {
	seen := map[digest.Digest]struct{}{}

	if err := e.Walk(ctx, root, func(descriptor ispec.Descriptor) error {
		seen[descriptor.Digest] = struct{}{}
		return nil
	}); err != nil {
		return nil, err
	}

	var reachable []digest.Digest
	for node := range seen {
		reachable = append(reachable, node)
	}
	return reachable, nil
}
