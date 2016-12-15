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

package symlink

import "os"

// FsEval is a mock-friendly (and unpriv.*) friendly way of wrapping
// filesystem-related functions. Note that this code (and all code referencing
// it) comes from this fork and is not present in the upstream code.
type FsEval interface {
	Lstat(path string) (fi os.FileInfo, err error)
	Readlink(path string) (linkname string, err error)
}

var _ FsEval = defaultFsEval

// defaultFsEval is just a wrapper around os.*.
const defaultFsEval = osFsEval(0)

type osFsEval int

func (fs osFsEval) Lstat(path string) (fi os.FileInfo, err error) {
	return os.Lstat(path)
}

func (fs osFsEval) Readlink(path string) (linkname string, err error) {
	return os.Readlink(path)
}
