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

package layerdiff

import (
	"compress/gzip"
	"io"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/vbatts/go-mtree"
)

// NOTE: This currently requires a version of go-mtree which has my Compare()
//       PR added. While we don't use this interface here, my work also
//       implemented the InodeDelta and supporting interfaces. Hopefully my PR
//       will be merged soon. https://github.com/vbatts/go-mtree/pull/48

// GenerateLayer creates a new OCI diff layer based on the mtree diff provided.
// All of the mtree.Modified and mtree.Extra blobs are read relative to the
// provided path (which should be the rootfs of the layer that was diffed).
func GenerateLayer(path string, deltas []mtree.InodeDelta) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	go func() {
		// We can't just dump all of the file contents into a tar file. We need
		// to emulate a proper tar generator. Luckily there aren't that many
		// things to emulate (and we can do them all in tar.go).
		// TODO: Should we doing the gzip in this layer?
		gzw := gzip.NewWriter(writer)
		tg := NewTarGenerator(gzw)

		// XXX: Do we need to sort the delta paths?

		for _, delta := range deltas {
			name := delta.Path()
			fullPath := filepath.Join(path, name)

			switch delta.Type() {
			case mtree.Modified, mtree.Extra:
				if err := tg.AddFile(name, fullPath); err != nil {
					logrus.Warnf("could not add file: %s", err)
					writer.CloseWithError(err)
					return
				}
			case mtree.Missing:
				if err := tg.AddWhiteout(name); err != nil {
					logrus.Warnf("could not add whiteout header: %s", err)
					writer.CloseWithError(err)
					return
				}
			}
		}

		if err := tg.tw.Close(); err != nil {
			logrus.Warnf("could not close layer: %s", err)
			writer.CloseWithError(err)
			return
		}

		if err := gzw.Close(); err != nil {
			logrus.Warnf("could not close gzip writer: %s", err)
			writer.CloseWithError(err)
			return
		}

		if err := writer.Close(); err != nil {
			logrus.Warnf("failed to close writer: %s", err)
			writer.CloseWithError(err)
			return
		}
	}()

	return reader, nil
}
