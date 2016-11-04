// Copyright 2016 The Linux Foundation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package image

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
)

type reader interface {
	Get(desc descriptor) (io.ReadCloser, error)
}

type tarReader struct {
	name string
}

func (r *tarReader) Get(desc descriptor) (io.ReadCloser, error) {
	f, err := os.Open(r.name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	tr := tar.NewReader(f)
loop:
	for {
		hdr, err := tr.Next()
		switch err {
		case io.EOF:
			break loop
		case nil:
		// success, continue below
		default:
			return nil, err
		}
		if hdr.Name == filepath.Join("blobs", desc.algo(), desc.hash()) &&
			!hdr.FileInfo().IsDir() {
			buf, err := ioutil.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			return ioutil.NopCloser(bytes.NewReader(buf)), nil
		}
	}

	return nil, fmt.Errorf("object not found")
}

type layoutReader struct {
	root string
}

func (r *layoutReader) Get(desc descriptor) (io.ReadCloser, error) {
	name := filepath.Join(r.root, "blobs", desc.algo(), desc.hash())

	info, err := os.Stat(name)
	if err != nil {
		return nil, err
	}

	if info.IsDir() {
		return nil, fmt.Errorf("object is dir")
	}

	return os.Open(name)
}
