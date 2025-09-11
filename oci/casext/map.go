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
	"fmt"
	"reflect"

	"github.com/apex/log"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/opencontainers/umoci/oci/casext/mediatype"
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
	return T == descriptorType
}

func mapDescriptors(V reflect.Value, mapFunc DescriptorMapFunc) error {
	// We can ignore this value.
	if !V.IsValid() {
		return nil
	}

	// First check that V isn't actually a ispec.Descriptor, if it is then
	// we're done.
	if isDescriptor(V.Type()) {
		oldDesc := V.Interface().(ispec.Descriptor) //nolint:forcetypeassert // already checked with reflection in isDescriptor
		newDesc := mapFunc(oldDesc)

		// We only need to do any assignment if the two are not equal.
		if !reflect.DeepEqual(newDesc, oldDesc) {
			// P is a ptr to V (or just V if it's already a pointer).
			P := V
			if !P.CanSet() {
				// This is a programmer error.
				return fmt.Errorf("[internal error] cannot apply map function to %v: %v is not settable", P, P.Type())
			}
			P.Set(reflect.ValueOf(newDesc))
		}
		return nil
	}

	// Recurse into all the types.
	switch V.Kind() { //nolint:exhaustive // no need to handle other types explicitly
	case reflect.Ptr, reflect.Interface:
		// Just deref the pointer/interface.
		if V.IsNil() {
			return nil
		}
		if err := mapDescriptors(V.Elem(), mapFunc); err != nil {
			return fmt.Errorf("%v: %w", V.Type(), err)
		}
		return nil

	case reflect.Slice, reflect.Array:
		// Iterate over each element.
		for idx := 0; idx < V.Len(); idx++ {
			err := mapDescriptors(V.Index(idx), mapFunc)
			if err != nil {
				return fmt.Errorf("%v[%d]->%v: %w", V.Type(), idx, V.Index(idx).Type(), err)
			}
		}
		return nil

	case reflect.Struct:
		// We are only ever going to be interested in registered types.
		if !mediatype.IsRegisteredPackage(V.Type().PkgPath()) {
			log.WithFields(log.Fields{
				"name":   V.Type().PkgPath() + "::" + V.Type().Name(),
				"v1path": descriptorType.PkgPath(),
			}).Debugf("detected jump outside permitted packages")
			return nil
		}

		// We can now actually iterate through a struct to find all descriptors.
		for idx := 0; idx < V.NumField(); idx++ {
			err := mapDescriptors(V.Field(idx), mapFunc)
			if err != nil {
				return fmt.Errorf("%v[%d=%s]->%v: %w", V.Type(), idx, V.Type().Field(idx).Name, V.Field(idx).Type(), err)
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

// MapDescriptors applies the given function once for every instance of
// ispec.Descriptor found in the given type, and replaces it with the returned
// value (which may be the same). This is done through the reflection API in
// Go, which means that hidden attributes may be inaccessible.
// DescriptorMapFunc will only be executed once for every ispec.Descriptor
// found.
func MapDescriptors(i any, mapFunc DescriptorMapFunc) error {
	return mapDescriptors(reflect.ValueOf(i), mapFunc)
}
