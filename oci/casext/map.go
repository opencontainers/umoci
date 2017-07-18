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
	"reflect"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
)

// Used by walkState.mark() to determine which struct members are descriptors to
// recurse into them. We aren't interested in struct members which are not
// either a slice of ispec.Descriptor or ispec.Descriptor themselves.
var descriptorType = reflect.TypeOf(ispec.Descriptor{})

// DescriptorMapFunc is a function that is used to provide a mapping between
// different descriptor values with MapDescriptors. It will not be called
// concurrently, and will only be called once for each recursively resolved
// element.
type DescriptorMapFunc func(ispec.Descriptor) ispec.Descriptor

// isDescriptor returns whether the given T is a ispec.Descriptor.
func isDescriptor(T reflect.Type) bool {
	return T.AssignableTo(descriptorType) && descriptorType.AssignableTo(T)
}

// MapDescriptors applies the given function once for every instance of
// ispec.Descriptor found in the given type, and replaces it with the returned
// value (which may be the same). This is done through the reflection API in
// Go, which means that hidden attributes may be inaccessible.
// DescriptorMapFunc will only be executed once for every ispec.Descriptor
// found.
func MapDescriptors(i interface{}, mapFunc DescriptorMapFunc) error {
	V := reflect.ValueOf(i)
	log.WithFields(log.Fields{
		"V": V,
	}).Debugf("MapDescriptors")
	if !V.IsValid() {
		// nil value
		return nil
	}

	// First check that V isn't actually a ispec.Descriptor, if it is then
	// we're done.
	if isDescriptor(V.Type()) {
		old := V.Interface().(ispec.Descriptor)
		new := mapFunc(old)

		if !V.CanSet() {
			// No need to return an error if they're equal.
			if reflect.DeepEqual(old, new) {
				return nil
			}
			return errors.Errorf("cannot apply map function to %v: IsSet != true", V)
		}

		V.Set(reflect.ValueOf(new))
		return nil
	}

	// Recurse into all the types.
	switch V.Kind() {
	case reflect.Ptr:
		// Just deref the pointer.
		log.WithFields(log.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into ptr")
		if V.IsNil() {
			return nil
		}
		return MapDescriptors(V.Elem().Interface(), mapFunc)

	case reflect.Array:
		// Convert to a slice.
		log.WithFields(log.Fields{
			"name": V.Type().PkgPath() + "::" + V.Type().Name(),
		}).Debugf("recursing into array")
		return MapDescriptors(V.Slice(0, V.Len()).Interface(), mapFunc)

	case reflect.Slice:
		// Iterate over each element.
		for idx := 0; idx < V.Len(); idx++ {
			log.WithFields(log.Fields{
				"name": V.Type().PkgPath() + "::" + V.Type().Name(),
				"idx":  idx,
			}).Debugf("recursing into slice")

			if err := MapDescriptors(V.Index(idx).Interface(), mapFunc); err != nil {
				return err
			}
		}
		return nil

	case reflect.Struct:
		// We are only ever going to be interested in ispec.* types.
		// XXX: This is something we might want to revisit in the future.
		if V.Type().PkgPath() != descriptorType.PkgPath() {
			log.WithFields(log.Fields{
				"name":   V.Type().PkgPath() + "::" + V.Type().Name(),
				"v1path": descriptorType.PkgPath(),
			}).Debugf("detected escape to outside ispec.* namespace")
			return nil
		}

		// We can now actually iterate through a struct to find all descriptors.
		for idx := 0; idx < V.NumField(); idx++ {
			log.WithFields(log.Fields{
				"name":  V.Type().PkgPath() + "::" + V.Type().Name(),
				"field": V.Type().Field(idx).Name,
			}).Debugf("recursing into struct")

			if err := MapDescriptors(V.Field(idx).Interface(), mapFunc); err != nil {
				return err
			}
		}
		return nil

	default:
		// FIXME: Should we log something here? While this will be hit normally
		//        (namely when we hit an io.ReadCloser) this seems a bit
		//        careless.
		return nil
	}

	// Unreachable.
}
