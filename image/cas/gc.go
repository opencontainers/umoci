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
	"reflect"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/image-spec/specs-go/v1"
	"golang.org/x/net/context"
)

// Used by gcState.mark() to determine which struct members are descriptors to
// recurse into them. We aren't interested in struct members which are not
// either a slice of v1.Descriptor or v1.Descriptor themselves.
var descriptorType reflect.Type = reflect.TypeOf(v1.Descriptor{})

// isDescriptor returns whether the given T is a v1.Descriptor.
// XXX: Should we be using .PkgPath() + .Name() here?
func isDescriptor(T reflect.Type) bool {
	return T.AssignableTo(descriptorType) && descriptorType.AssignableTo(T)
}

// childDescriptors returns all child v1.Descriptors given a particular
// interface{}. This is recursively evaluated, so if you have some cyclic
// struct pointer stuff going on things won't end well.
// FIXME: Should we implement this in a way that avoids cycle issues?
func childDescriptors(i interface{}) []v1.Descriptor {
	// XXX: Is this correct?
	V := reflect.ValueOf(i)
	logrus.WithFields(logrus.Fields{
		"V": V,
	}).Debugf("childDescriptors")
	if !V.IsValid() {
		// nil value
		return []v1.Descriptor{}
	}

	// First check that V isn't actually a v1.Descriptor.
	if isDescriptor(V.Type()) {
		return []v1.Descriptor{V.Interface().(v1.Descriptor)}
	}

	// Recurse into all the types.
	switch V.Kind() {
	case reflect.Ptr:
		// Just deref the pointer.
		logrus.WithFields(logrus.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into ptr")
		if V.IsNil() {
			return []v1.Descriptor{}
		}
		return childDescriptors(V.Elem().Interface())

	case reflect.Array:
		// Convert to a slice.
		logrus.WithFields(logrus.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into array")
		return childDescriptors(V.Slice(0, V.Len()).Interface())

	case reflect.Slice:
		// Iterate over each element and append them to childDescriptors.
		children := []v1.Descriptor{}
		for idx := 0; idx < V.Len(); idx++ {
			logrus.WithFields(logrus.Fields{
				"name": V.Type().PkgPath() + "::" + V.Type().Name(),
				"idx":  idx,
			}).Debugf("recursing into slice")
			children = append(children, childDescriptors(V.Index(idx).Interface())...)
		}
		return children

	case reflect.Struct:
		// We are only ever going to be interested in v1.* types.
		if V.Type().PkgPath() != descriptorType.PkgPath() {
			logrus.WithFields(logrus.Fields{
				"name":   V.Type().PkgPath() + "::" + V.Type().Name(),
				"v1path": descriptorType.PkgPath(),
			}).Debugf("detected escape to outside v1.* namespace")
			return []v1.Descriptor{}
		}

		// We can now actually iterate through a struct to find all descriptors.
		children := []v1.Descriptor{}
		for idx := 0; idx < V.NumField(); idx++ {
			logrus.WithFields(logrus.Fields{
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
		return []v1.Descriptor{}
	}

	panic("should never be reached")
}

// gcState represents the state of the garbage collector at one point in time.
type gcState struct {
	// engine is the CAS engine we are operating on.
	engine Engine

	// black is the set of digests which are reachable by a descriptor path
	// from the root set. These are blobs which will *not* be deleted. The
	// white set of digests is not stored in the state (we only have to compute
	// it once anyway).
	black map[string]struct{}
}

func (gc *gcState) mark(ctx context.Context, descriptor v1.Descriptor) error {
	logrus.WithFields(logrus.Fields{
		"digest": descriptor.Digest,
	}).Debugf("gc.mark")

	// Technically we should never hit this because you can't have cycles in a
	// Merkle tree. But you can't be too careful.
	if _, ok := gc.black[descriptor.Digest]; ok {
		return nil
	}

	// Add the descriptor itself to the black list.
	gc.black[descriptor.Digest] = struct{}{}

	// Get the blob to recurse into.
	blob, err := FromDescriptor(ctx, gc.engine, &descriptor)
	if err != nil {
		return err
	}

	// Mark all children.
	for _, child := range childDescriptors(blob.Data) {
		logrus.WithFields(logrus.Fields{
			"digest": descriptor.Digest,
			"child":  child.Digest,
		}).Debugf("gc.mark recursing into child")

		if err := gc.mark(ctx, child); err != nil {
			return err
		}
	}

	return nil
}

// GC will perform a mark-and-sweep garbage collection of the OCI image
// referenced by the given CAS engine. The root set is taken to be the set of
// references stored in the image, and all blobs not reachable by following a
// descriptor path from the root set will be removed.
//
// GC will only call ListBlobs and ListReferences once, and assumes that there
// is no change in the set of references or blobs after calling those
// functions. In other words, it assumes it is the only user of the image that
// is making modifications. Things will not go well if this assumption is
// challenged.
func GC(engine Engine, ctx context.Context) error {
	// Generate the root set of descriptors.
	var root []v1.Descriptor

	names, err := engine.ListReferences(ctx)
	if err != nil {
		return err
	}

	for _, name := range names {
		descriptor, err := engine.GetReference(ctx, name)
		if err != nil {
			return err
		}
		logrus.WithFields(logrus.Fields{
			"name":   name,
			"digest": descriptor.Digest,
		}).Debugf("GC: got reference")
		root = append(root, *descriptor)
	}

	// Mark from the root set.
	gc := &gcState{
		engine: engine,
		black:  map[string]struct{}{},
	}

	for _, descriptor := range root {
		logrus.WithFields(logrus.Fields{
			"digest": descriptor.Digest,
		}).Debugf("GC: marking from root")
		if err := gc.mark(ctx, descriptor); err != nil {
			return err
		}
	}

	// Sweep all blobs in the white set.
	blobs, err := engine.ListBlobs(ctx)
	if err != nil {
		return err
	}

	n := 0
	for _, digest := range blobs {
		if _, ok := gc.black[digest]; ok {
			// Digest is in the black set.
			continue
		}
		logrus.WithFields(logrus.Fields{
			"digest": digest,
		}).Infof("GC: garbage collecting blob")
		if err := engine.DeleteBlob(ctx, digest); err != nil {
			return err
		}
		n++
	}

	logrus.Infof("GC: garbage collected %d blobs", n)
	return nil
}
