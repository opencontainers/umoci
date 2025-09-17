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

package umoci

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/apex/log"
	"github.com/docker/go-units"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/urfave/cli"
	"github.com/vbatts/go-mtree"

	"github.com/opencontainers/umoci/internal/funchelpers"
	"github.com/opencontainers/umoci/internal/idtools"
	"github.com/opencontainers/umoci/oci/casext"
	igen "github.com/opencontainers/umoci/oci/config/generate"
	"github.com/opencontainers/umoci/oci/layer"
)

// FIXME: This should be moved to a library. Too much of this code is in the
//        cmd/... code, but should really be refactored to the point where it
//        can be useful to other people. This is _particularly_ true for the
//        code which repacks images (the changes to the config, manifest and
//        CAS should be made into a library).

// MtreeKeywords is the set of keywords used by umoci for verification and diff
// generation of a bundle. This is based on mtree.DefaultKeywords, but is
// hardcoded here to ensure that vendor changes don't mess things up.
var MtreeKeywords = []mtree.Keyword{
	"size",
	"type",
	"uid",
	"gid",
	"mode",
	"link",
	"nlink",
	"tar_time",
	"sha256digest",
	"xattr",
}

// MetaName is the name of umoci's metadata file that is stored in all
// bundles extracted by umoci.
const MetaName = "umoci.json"

// MetaVersion is the version of Meta supported by this code. The
// value is only bumped for updates which are not backwards compatible.
const MetaVersion = "2"

// Meta represents metadata about how umoci unpacked an image to a bundle
// and other similar information. It is used to keep track of information that
// is required when repacking an image and other similar bundle information.
type Meta struct {
	// Version is the version of umoci used to unpack the bundle. This is used
	// to future-proof the umoci.json information.
	Version string `json:"umoci_version"`

	// From is a copy of the descriptor pointing to the image manifest that was
	// used to unpack the bundle. Essentially it's a resolved form of the
	// --image argument to umoci-unpack(1).
	From casext.DescriptorPath `json:"from_descriptor_path"`

	// MapOptions is the parsed version of --uid-map, --gid-map and --rootless
	// arguments to umoci-unpack(1). While all of these options technically do
	// not need to be the same for corresponding umoci-unpack(1) and
	// umoci-repack(1) calls, changing them is not recommended and so the
	// default should be that they are the same.
	MapOptions layer.MapOptions `json:"map_options"`

	// WhiteoutMode indicates what style of whiteout was written to disk
	// when this filesystem was extracted.
	//
	// Deprecated: This feature was completely broken. See
	// <https://github.com/opencontainers/umoci/issues/574> for more details.
	WhiteoutMode int `json:"whiteout_mode,omitempty"`
}

// WriteTo writes a JSON-serialised version of Meta to the given io.Writer.
func (m Meta) WriteTo(w io.Writer) (int64, error) {
	buf := new(bytes.Buffer)
	err := json.NewEncoder(io.MultiWriter(buf, w)).Encode(m)
	return int64(buf.Len()), err
}

// WriteBundleMeta writes an umoci.json file to the given bundle path.
func WriteBundleMeta(bundle string, meta Meta) (Err error) {
	fh, err := os.Create(filepath.Join(bundle, MetaName))
	if err != nil {
		return fmt.Errorf("create metadata: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, fh)

	if _, err := meta.WriteTo(fh); err != nil {
		return fmt.Errorf("write metadata: %w", err)
	}
	return nil
}

// ReadBundleMeta reads and parses the umoci.json file from a given bundle path.
func ReadBundleMeta(bundle string) (_ Meta, Err error) {
	var meta Meta

	fh, err := os.Open(filepath.Join(bundle, MetaName))
	if err != nil {
		return meta, fmt.Errorf("open metadata: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, fh)

	err = json.NewDecoder(fh).Decode(&meta)
	if err != nil {
		return meta, fmt.Errorf("decode metadata: %w", err)
	}
	if meta.Version != MetaVersion {
		return meta, fmt.Errorf("decode metadata: unsupported umoci.json version %q", meta.Version)
	}
	// NOTE: This field has been deprecated, as the feature was completely
	// broken. See <https://github.com/opencontainers/umoci/issues/574> for
	// more details.
	if meta.WhiteoutMode != 0 {
		return meta, fmt.Errorf("decode metadata: deprecated (broken) whiteout_mode field set (%d)", meta.WhiteoutMode)
	}
	return meta, nil
}

// ManifestStat has information about a given OCI manifest.
// TODO: Implement support for manifest lists, this should also be able to
// contain stat information for a list of manifests.
type ManifestStat struct {
	Manifest manifestStat `json:"manifest"`

	// Config stores information about the configuration of a manifest.
	Config configStat `json:"config"`

	// History stores the history information for the manifest.
	History historyStatList `json:"history"`
}

func quote(s string, quoteEmpty bool) string {
	quoted := strconv.Quote(s)
	// Only return quoted string if it actually required escaping or is empty.
	if quoted != `"`+s+`"` || (quoteEmpty && s == "") {
		return quoted
	}
	return s
}

func pprint(w io.Writer, prefix, key string, values ...string) (err error) {
	if len(values) > 0 {
		for idx, value := range values {
			if strings.Contains(value, ",") {
				// Make sure "," leads to quoting.
				values[idx] = strconv.Quote(value)
			} else {
				values[idx] = quote(values[idx], true)
			}
		}
		_, err = fmt.Fprintf(w, "%s%s: %s\n", prefix, key, strings.Join(values, ", "))
	} else {
		_, err = fmt.Fprintf(w, "%s%s:\n", prefix, key)
	}
	return err
}

func pprintSlice(w io.Writer, prefix, name string, data []string) error {
	if err := pprint(w, prefix, name); err != nil {
		return err
	}
	prefix += "\t"
	for _, line := range data {
		if _, err := fmt.Fprintf(w, "%s%s\n", prefix, quote(line, true)); err != nil {
			return err
		}
	}
	return nil
}

func pprintMap(w io.Writer, prefix, name string, data map[string]string) error {
	if err := pprint(w, prefix, name); err != nil {
		return err
	}
	prefix += "\t"
	for _, key := range slices.Sorted(maps.Keys(data)) {
		if err := pprint(w, prefix, key, data[key]); err != nil {
			return err
		}
	}
	return nil
}

func pprintSet(w io.Writer, prefix, name string, data map[string]struct{}) error {
	keys := slices.Sorted(maps.Keys(data))
	return pprint(w, prefix, name, keys...)
}

func pprintPlatform(w io.Writer, prefix string, platform ispec.Platform) error {
	if err := pprint(w, prefix, "Platform"); err != nil {
		return err
	}
	prefix += "\t"

	if err := pprint(w, prefix, "OS", platform.OS); err != nil {
		return err
	}
	if platform.OSVersion != "" {
		if err := pprint(w, prefix, "OS Version", platform.OSVersion); err != nil {
			return err
		}
	}
	if len(platform.OSFeatures) > 0 {
		if err := pprint(w, prefix, "OS Features", platform.OSFeatures...); err != nil {
			return err
		}
	}
	arch := platform.Architecture
	if platform.Variant != "" {
		arch = fmt.Sprintf("%s (%s)", platform.Architecture, platform.Variant)
	}
	if err := pprint(w, prefix, "Architecture", arch); err != nil {
		return err
	}
	return nil
}

// pprintDescriptor pretty-prints an ispec.Descriptor.
func pprintDescriptor(w io.Writer, prefix string, descriptor ispec.Descriptor) error {
	if err := pprint(w, prefix, "Descriptor"); err != nil {
		return err
	}
	prefix += "\t"
	if err := pprint(w, prefix, "Media Type", descriptor.MediaType); err != nil {
		return err
	}
	if err := pprint(w, prefix, "Digest", descriptor.Digest.String()); err != nil {
		return err
	}
	size := units.HumanSize(float64(descriptor.Size))
	if err := pprint(w, prefix, "Size", size); err != nil {
		return err
	}
	if descriptor.Platform != nil {
		if err := pprintPlatform(w, "", *descriptor.Platform); err != nil {
			return err
		}
	}
	if len(descriptor.URLs) > 0 {
		if err := pprintSlice(w, prefix, "URLs", descriptor.URLs); err != nil {
			return err
		}
	}
	if len(descriptor.Annotations) > 0 {
		if err := pprintMap(w, prefix, "Annotations", descriptor.Annotations); err != nil {
			return err
		}
	}
	// TODO(image-spec v1.1): descriptor.Data
	// TODO(image-spec v1.1): descriptor.ArtifactType
	return nil
}

// Format formats a ManifestStat using the default formatting, and writes the
// result to the given writer.
//
// TODO: This should really be implemented in a way that allows for users to
// define their own custom templates for different blocks (meaning that this
// should use text/template rather than using tabwriters manually.
func (ms ManifestStat) Format(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "== MANIFEST =="); err != nil {
		return err
	}
	if err := ms.Manifest.pprint(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, "\n== CONFIG =="); err != nil {
		return err
	}
	if err := ms.Config.pprint(w); err != nil {
		return err
	}

	// Output history information.
	if _, err := fmt.Fprintln(w, "\n== HISTORY =="); err != nil {
		return err
	}
	if err := ms.History.pprint(w); err != nil {
		return err
	}
	return nil
}

// manifestStat contains information about the image manifest.
type manifestStat struct {
	// Descriptor is the descriptor for the configuration JSON.
	Descriptor ispec.Descriptor `json:"descriptor"`

	// Manifest is the contents of the image manifest.
	Manifest ispec.Manifest `json:"-"`

	// RawData is the raw data stream of the blob, which is output when we
	// provide JSON output (to make sure no information is lost in --json
	// mode).
	RawData json.RawMessage `json:"blob"`
}

func (m manifestStat) pprint(w io.Writer) error {
	manifest := m.Manifest
	if err := pprint(w, "", "Schema Version", strconv.Itoa(manifest.SchemaVersion)); err != nil {
		return err
	}
	if err := pprint(w, "", "Media Type", manifest.MediaType); err != nil {
		return err
	}
	// TODO(image-spec v1.1): manifest.ArtifactType
	// TODO(image-spec v1.1): manifest.Subject
	if err := pprint(w, "", "Config"); err != nil {
		return err
	}
	if err := pprintDescriptor(w, "\t", manifest.Config); err != nil {
		return err
	}
	if len(manifest.Layers) > 0 {
		if err := pprint(w, "", "Layers"); err != nil {
			return err
		}
		for _, layer := range manifest.Layers {
			if err := pprintDescriptor(w, "\t", layer); err != nil {
				return err
			}
		}
	}
	if len(manifest.Annotations) > 0 {
		if err := pprintMap(w, "", "Annotations", manifest.Annotations); err != nil {
			return err
		}
	}
	if err := pprintDescriptor(w, "", m.Descriptor); err != nil {
		return err
	}
	return nil
}

// configStat contains information about the image configuration of this
// manifest.
type configStat struct {
	// Descriptor is the descriptor for the configuration JSON.
	Descriptor ispec.Descriptor `json:"descriptor"`

	// Image is the contents of the configuration.
	Image ispec.Image `json:"-"`

	// RawData is the raw data stream of the blob, which is output when we
	// provide JSON output (to make sure no information is lost in --json
	// mode).
	RawData json.RawMessage `json:"blob"`
}

func pprintImageConfig(w io.Writer, prefix string, config ispec.ImageConfig) error {
	if err := pprint(w, prefix, "Image Config"); err != nil {
		return err
	}
	prefix += "\t"
	if err := pprint(w, prefix, "User", config.User); err != nil {
		return err
	}

	if len(config.Entrypoint) > 0 {
		if err := pprintSlice(w, prefix, "Entrypoint", config.Entrypoint); err != nil {
			return err
		}
	}
	if err := pprintSlice(w, prefix, "Command", config.Cmd); err != nil {
		return err
	}
	if config.WorkingDir != "" {
		if err := pprint(w, prefix, "Working Directory", config.WorkingDir); err != nil {
			return err
		}
	}
	if len(config.Env) > 0 {
		if err := pprintSlice(w, prefix, "Environment", config.Env); err != nil {
			return err
		}
	}
	if config.StopSignal != "" {
		if err := pprint(w, prefix, "Stop Signal", config.StopSignal); err != nil {
			return err
		}
	}
	if len(config.ExposedPorts) > 0 {
		if err := pprintSet(w, prefix, "Exposed Ports", config.ExposedPorts); err != nil {
			return err
		}
	}
	if len(config.Volumes) > 0 {
		if err := pprintSet(w, prefix, "Volumes", config.Volumes); err != nil {
			return err
		}
	}
	if len(config.Labels) > 0 {
		if err := pprintMap(w, prefix, "Labels", config.Labels); err != nil {
			return err
		}
	}
	return nil
}

func (c configStat) pprint(w io.Writer) error {
	image := c.Image
	if image.Created != nil {
		date := image.Created.Format(igen.ISO8601)
		if err := pprint(w, "", "Created", date); err != nil {
			return err
		}
	}
	if err := pprint(w, "", "Author", image.Author); err != nil {
		return err
	}
	// TODO(image-spec v1.1): Use embedded Platform.
	platform := ispec.Platform{
		OS:           image.OS,
		Architecture: image.Architecture,
	}
	if err := pprintPlatform(w, "", platform); err != nil {
		return err
	}
	if err := pprintImageConfig(w, "", image.Config); err != nil {
		return err
	}
	if err := pprintDescriptor(w, "", c.Descriptor); err != nil {
		return err
	}
	return nil
}

// historyStat contains information about a single entry in the history of a
// manifest. This is essentially equivalent to a single record from
// docker-history(1).
type historyStat struct {
	// Layer is the descriptor referencing where the layer is stored. If it is
	// nil, then this entry is an empty_layer (and thus doesn't have a backing
	// diff layer).
	Layer *ispec.Descriptor `json:"layer"`

	// DiffID is an additional piece of information to Layer. It stores the
	// DiffID of the given layer corresponding to the history entry. If DiffID
	// is "", then this entry is an empty_layer.
	DiffID string `json:"diff_id"`

	// History is embedded in the stat information.
	ispec.History
}

type historyStatList []historyStat

func (hsl historyStatList) pprint(w io.Writer) error {
	tw := tabwriter.NewWriter(w, 4, 2, 1, ' ', 0)
	if _, err := fmt.Fprintf(tw, "LAYER\tCREATED\tCREATED BY\tSIZE\tCOMMENT\n"); err != nil {
		return err
	}
	for _, histEntry := range hsl {
		var (
			created   = quote(histEntry.Created.Format(igen.ISO8601), false)
			createdBy = quote(histEntry.CreatedBy, false)
			comment   = quote(histEntry.Comment, false)
			layerID   = "<none>"
			size      = "<none>"
		)

		if !histEntry.EmptyLayer {
			layerID = histEntry.Layer.Digest.String()
			size = units.HumanSize(float64(histEntry.Layer.Size))
		}

		// TODO: We need to truncate some of the fields.
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", layerID, created, createdBy, size, comment); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// Stat computes the ManifestStat for a given manifest blob. The provided
// descriptor must refer to an OCI Manifest.
func Stat(ctx context.Context, engine casext.Engine, manifestDescriptor ispec.Descriptor) (ManifestStat, error) {
	var stat ManifestStat

	if manifestDescriptor.MediaType != ispec.MediaTypeImageManifest {
		return stat, fmt.Errorf("stat: cannot stat a non-manifest descriptor: invalid media type %q", manifestDescriptor.MediaType)
	}

	// We have to get the actual manifest.
	manifestBlob, err := engine.FromDescriptor(ctx, manifestDescriptor)
	if err != nil {
		return stat, err
	}
	manifest, ok := manifestBlob.Data.(ispec.Manifest)
	if !ok {
		// Should _never_ be reached.
		return stat, fmt.Errorf("[internal error] unknown manifest blob type: %s", manifestBlob.Descriptor.MediaType)
	}
	stat.Manifest = manifestStat{
		Descriptor: manifestDescriptor,
		Manifest:   manifest,
		RawData:    manifestBlob.RawData,
	}

	// Now get the config.
	configBlob, err := engine.FromDescriptor(ctx, manifest.Config)
	if err != nil {
		return stat, fmt.Errorf("stat: %w", err)
	}
	config, ok := configBlob.Data.(ispec.Image)
	if !ok {
		// Should _never_ be reached.
		return stat, fmt.Errorf("[internal error] unknown config blob type: %s", configBlob.Descriptor.MediaType)
	}
	stat.Config = configStat{
		Descriptor: manifest.Config,
		Image:      config,
		RawData:    configBlob.RawData,
	}

	// TODO: This should probably be moved into separate functions.

	// Generate the history of the image. Because the config.History entries
	// are in the same order as the manifest.Layer entries this is fairly
	// simple. However, we only increment the layer index if a layer was
	// actually generated by a history entry.
	layerIdx := 0
	for _, histEntry := range config.History {
		info := historyStat{
			History: histEntry,
			DiffID:  "",
			Layer:   nil,
		}

		// Only fill the other information and increment layerIdx if it's a
		// non-empty layer.
		if !histEntry.EmptyLayer {
			info.DiffID = config.RootFS.DiffIDs[layerIdx].String()
			info.Layer = &manifest.Layers[layerIdx]
			layerIdx++
		}

		stat.History = append(stat.History, info)
	}

	return stat, nil
}

// GenerateBundleManifest creates and writes an mtree of the rootfs in the given
// bundle path, using the supplied fsEval method.
func GenerateBundleManifest(mtreeName string, bundlePath string, fsEval mtree.FsEval) (Err error) {
	mtreePath := filepath.Join(bundlePath, mtreeName+".mtree")
	fullRootfsPath := filepath.Join(bundlePath, layer.RootfsName)

	log.WithFields(log.Fields{
		"keywords": MtreeKeywords,
		"mtree":    mtreePath,
	}).Debugf("umoci: generating mtree manifest")

	log.Info("computing filesystem manifest ...")
	dh, err := mtree.Walk(fullRootfsPath, nil, MtreeKeywords, fsEval)
	if err != nil {
		return fmt.Errorf("generate mtree spec: %w", err)
	}
	log.Info("... done")

	flags := os.O_CREATE | os.O_WRONLY | os.O_EXCL
	fh, err := os.OpenFile(mtreePath, flags, 0o644)
	if err != nil {
		return fmt.Errorf("open mtree: %w", err)
	}
	defer funchelpers.VerifyClose(&Err, fh)

	log.Debugf("umoci: saving mtree manifest")

	if _, err := dh.WriteTo(fh); err != nil {
		return fmt.Errorf("write mtree: %w", err)
	}

	return nil
}

// ParseIdmapOptions sets up the mapping options for Meta, using
// the arguments specified on the command line.
func ParseIdmapOptions(meta *Meta, ctx *cli.Context) error {
	// We need to set mappings if we're in rootless mode.
	meta.MapOptions.Rootless = ctx.Bool("rootless")
	if meta.MapOptions.Rootless {
		if !ctx.IsSet("uid-map") {
			if err := ctx.Set("uid-map", fmt.Sprintf("0:%d:1", os.Geteuid())); err != nil {
				// Should _never_ be reached.
				return fmt.Errorf("[internal error] failure auto-setting rootless --uid-map: %w", err)
			}
		}
		if !ctx.IsSet("gid-map") {
			if err := ctx.Set("gid-map", fmt.Sprintf("0:%d:1", os.Getegid())); err != nil {
				// Should _never_ be reached.
				return fmt.Errorf("[internal error] failure auto-setting rootless --gid-map: %w", err)
			}
		}
	}

	for _, uidmap := range ctx.StringSlice("uid-map") {
		idMap, err := idtools.ParseMapping(uidmap)
		if err != nil {
			return fmt.Errorf("failure parsing --uid-map %s: %w", uidmap, err)
		}
		meta.MapOptions.UIDMappings = append(meta.MapOptions.UIDMappings, idMap)
	}
	for _, gidmap := range ctx.StringSlice("gid-map") {
		idMap, err := idtools.ParseMapping(gidmap)
		if err != nil {
			return fmt.Errorf("failure parsing --gid-map %s: %w", gidmap, err)
		}
		meta.MapOptions.GIDMappings = append(meta.MapOptions.GIDMappings, idMap)
	}

	log.WithFields(log.Fields{
		"map.uid": meta.MapOptions.UIDMappings,
		"map.gid": meta.MapOptions.GIDMappings,
	}).Debugf("parsed mappings")

	return nil
}
