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

package system

import (
	"testing"
)

func TestMakedev(t *testing.T) {
	for _, test := range []struct {
		major, minor uint64
	}{
		{0, 0},
		{1, 13},
		{52, 12},
		{2, 252},
	} {
		devt := Makedev(test.major, test.minor)
		gotMajor := Majordev(devt)
		gotMinor := Minordev(devt)

		if gotMajor != test.major || gotMinor != test.minor {
			t.Errorf("got wrong {major:minor} for test: expected={%d:%d} got={%d:%d}", test.major, test.minor, gotMajor, gotMinor)
		}
	}
}
