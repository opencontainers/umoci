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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateRefname(t *testing.T) {
	for _, test := range []struct {
		refname string
		valid   bool
	}{
		// No characters.
		{"", false},
		// Component "/" without next component.
		{"somepath392/", false},
		// Duplicate "/".
		{"some//test/hello", false},
		{"some/oth3r//teST", false},
		// Separator without additional alphanum.
		{"deadb33fc4f3+123888+", false},
		// More than one separator.
		{"anot++her", false},
		{"dead-.meme/cafe", false},
		// Leading separator or "/".
		{"/a/b/c123", false},
		{"--21js8CAS", false},
		{".AZ94n18s", false},
		{"@2318s88", false},
		{":29a.158/2131--91ab", false},
		// Plain components.
		{"a", true},
		{"latest", true},
		{"42.03.19", true},
		{"v1.3.1+dev", true},
		{"aBC1958NaK284IT9Q0kX82jnMnis8201j", true},
		{"Aa0Bb1Cc2-Dd3Ee4Ff5.Gg6Hh7Ii8:Jj9KkLl@MmNnOo+QqRrSs--TtUuVv.WwXxYy_Zz", true},
		{"A--2.C+9@e_3", true},
		// Multiple components.
		{"Aa0-Bb1/Cc2-Dd3.Ee4Ff5/Gg6Hh7Ii8:Jj/9KkLl@Mm/NnOo+QqRrS/s--TtUu/Vv.WwXxYy_Z/z", true},
		{"A/1--2.C+9@e_4/3", true},
		{"etc/passwd/123", true},
	} {
		valid := IsValidReferenceName(test.refname)
		assert.Equalf(t, test.valid, valid, "incorrectly determined validity of refname %q", test.refname)
	}
}
