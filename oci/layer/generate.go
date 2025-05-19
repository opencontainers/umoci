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

package layer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/apex/log"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci/pkg/fseval"
)

// inodeDeltas is a wrapper around []mtree.InodeDelta that allows for sorting
// the set of deltas by the pathname.
type inodeDeltas []mtree.InodeDelta

func (ids inodeDeltas) Len() int           { return len(ids) }
func (ids inodeDeltas) Less(i, j int) bool { return ids[i].Path() < ids[j].Path() }
func (ids inodeDeltas) Swap(i, j int)      { ids[i], ids[j] = ids[j], ids[i] }

// GenerateLayer creates a new OCI diff layer based on the mtree diff provided.
// All of the mtree.Modified and mtree.Extra blobs are read relative to the
// provided path (which should be the rootfs of the layer that was diffed). The
// returned reader is for the *raw* tar data, it is the caller's responsibility
// to gzip it.
func GenerateLayer(path string, deltas []mtree.InodeDelta, opt *RepackOptions) (io.ReadCloser, error) {
	opt = opt.fill()

	fsEval := fseval.Default
	if opt.MapOptions().Rootless {
		fsEval = fseval.Rootless
	}

	reader, writer := io.Pipe()

	go func() (Err error) {
		// Close with the returned error.
		defer func() {
			var closeErr error
			if Err != nil {
				log.Warnf("could not generate layer: %v", Err)
				closeErr = fmt.Errorf("generate layer: %w", Err)
			}
			_ = writer.CloseWithError(closeErr)
		}()

		// We can't just dump all of the file contents into a tar file. We need
		// to emulate a proper tar generator. Luckily there aren't that many
		// things to emulate (and we can do them all in tar.go).
		tg := newTarGenerator(writer, opt)

		// Sort the delta paths.
		// FIXME: We need to add whiteouts first, otherwise we might end up
		//        doing something silly like deleting a file which we actually
		//        meant to modify.
		sort.Sort(inodeDeltas(deltas))

		for _, delta := range deltas {
			name := delta.Path()
			fullPath := filepath.Join(path, name)

			// XXX: It's possible that if we unlink a hardlink, we're going to
			//      AddFile() for no reason. Maybe we should drop nlink= from
			//      the set of keywords we care about?

			switch delta.Type() {
			case mtree.Modified, mtree.Extra:
				if onDiskFmt, isOverlayfs := opt.OnDiskFormat.(OverlayfsRootfs); isOverlayfs {
					woType, isWo, err := isOverlayWhiteout(onDiskFmt, fullPath, fsEval)
					if err != nil {
						return fmt.Errorf("check if %q is a whiteout: %w", fullPath, err)
					}
					if isWo {
						log.Debugf("generate layer: converting overlayfs whiteout %s %q to OCI whiteout", woType, name)

						var err error
						switch woType {
						case overlayWhiteoutPlain:
							err = tg.AddWhiteout(name)
						case overlayWhiteoutOpaque:
							// For opaque whiteout directories we need to
							// output an entry for the directory itself so that
							// the ownership and modes set on the directory are
							// included in the archive.
							if err := tg.AddFile(name, fullPath); err != nil {
								log.Warnf("generate layer: could not add directory entry for opaque from overlayfs for file %q: %s", name, err)
								return fmt.Errorf("generate directory entry for opaque whiteout from overlayfs: %w", err)
							}
							err = tg.AddOpaqueWhiteout(name)
						default:
							return fmt.Errorf("[internal error] unknown overlayfs whiteout type %q", woType)
						}
						if err != nil {
							log.Warnf("generate layer: could not add whiteout %s from overlayfs for file %q: %s", woType, name, err)
							return fmt.Errorf("generate whiteout %s from overlayfs: %w", woType, err)
						}
						continue
					}
				}
				if err := tg.AddFile(name, fullPath); err != nil {
					log.Warnf("generate layer: could not add file %q: %s", name, err)
					return fmt.Errorf("generate layer file: %w", err)
				}
			case mtree.Missing:
				if err := tg.AddWhiteout(name); err != nil {
					log.Warnf("generate layer: could not add whiteout %q: %s", name, err)
					return fmt.Errorf("generate whiteout layer file: %w", err)
				}

			case mtree.Same, mtree.ErrorDifference:
				fallthrough
			default:
				// We should never see these delta types because they are not
				// generated for regular mtree.Compare.
				return fmt.Errorf("generate layer: unsupported mtree delta type %v for path %q", delta.Type(), name)
			}
		}

		if err := tg.tw.Close(); err != nil {
			log.Warnf("generate layer: could not close tar.Writer: %s", err)
			return fmt.Errorf("close tar writer: %w", err)
		}

		return nil
	}() //nolint:errcheck // errors are handled in defer func

	return reader, nil
}

// GenerateInsertLayer generates a completely new layer from root to be
// inserted into the image at target. If root is an empty string then the
// target will be removed via a whiteout. If opaque is true then the target
// directory will also have an opaque whiteout applied (clearing any files
// inside the directory), followed by the contents of the root.
func GenerateInsertLayer(root, target string, opaque bool, opt *RepackOptions) io.ReadCloser {
	root = CleanPath(root)
	opt = opt.fill()

	fsEval := fseval.Default
	if opt.MapOptions().Rootless {
		fsEval = fseval.Rootless
	}

	reader, writer := io.Pipe()

	go func() (Err error) {
		defer func() {
			var closeErr error
			if Err != nil {
				log.Warnf("could not generate insert layer: %v", Err)
				closeErr = fmt.Errorf("generate insert layer: %w", Err)
			}
			_ = writer.CloseWithError(closeErr)
		}()

		tg := newTarGenerator(writer, opt)

		defer func() {
			if err := tg.tw.Close(); err != nil {
				log.Warnf("generate insert layer: could not close tar.Writer: %s", err)
			}
		}()

		if root == "" {
			return tg.AddWhiteout(target)
		}
		if opaque {
			if err := tg.AddOpaqueWhiteout(target); err != nil {
				return err
			}
			// Continue on to add the new root contents...
		}
		return fsEval.Walk(root, func(fullPath string, _ os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			relName, err := filepath.Rel(root, fullPath)
			if err != nil {
				return err
			}
			name := filepath.Join(target, relName)

			if onDiskFmt, isOverlayfs := opt.OnDiskFormat.(OverlayfsRootfs); isOverlayfs {
				woType, isWo, err := isOverlayWhiteout(onDiskFmt, fullPath, fsEval)
				if err != nil {
					return fmt.Errorf("check if %q is a whiteout: %w", fullPath, err)
				}
				if isWo {
					log.Debugf("generate insert layer: converting overlayfs %s %q to OCI whiteout", woType, name)
					switch woType {
					case overlayWhiteoutPlain:
						return tg.AddWhiteout(name)
					case overlayWhiteoutOpaque:
						// For opaque whiteout directories we need to
						// output an entry for the directory itself so that
						// the ownership and modes set on the directory are
						// included in the archive.
						if err := tg.AddFile(name, fullPath); err != nil {
							log.Warnf("generate insert layer: could not add directory entry for opaque from overlayfs for file %q: %s", name, err)
							return fmt.Errorf("generate directory entry for opaque whiteout from overlayfs: %w", err)
						}
						return tg.AddOpaqueWhiteout(name)
					default:
						return fmt.Errorf("[internal error] unknown overlayfs whiteout type %q", woType)
					}
				}
			}

			return tg.AddFile(name, fullPath)
		})
	}() //nolint:errcheck // errors are handled in defer func
	return reader
}
