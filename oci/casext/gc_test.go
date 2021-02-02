package casext

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas/dir"
)

func TestGCWithEmptyIndex(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReference")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)
	defer engine.Close()

	// creates an empty index.json and several orphan blobs which should be pruned
	descMap, err := fakeSetupEngine(t, engineExt)
	if err != nil {
		t.Fatalf("unexpected error doing fakeSetupEngine: %+v", err)
	}
	if descMap == nil {
		t.Fatalf("empty descMap")
	}

	b, err := engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty blob list before GC")
	}

	err = engineExt.GC(ctx)
	if err != nil {
		t.Fatalf("GC failed: %+v", err)
	}

	b, err = engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) != 0 {
		t.Fatalf("expected empty blob list after GC")
	}
}

func TestGCWithNonEmptyIndex(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReference")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)
	defer engine.Close()

	// creates an empty index.json and several orphan blobs which should be pruned
	descMap, err := fakeSetupEngine(t, engineExt)
	if err != nil {
		t.Fatalf("unexpected error doing fakeSetupEngine: %+v", err)
	}
	if descMap == nil {
		t.Fatalf("empty descMap")
	}

	b, err := engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty blob list before GC")
	}

	// build a blob, manifest, index that will survive GC
	content := "this is a test blob"
	br := strings.NewReader(content)
	digest, size, err := engine.PutBlob(ctx, br)
	if err != nil {
		t.Fatalf("error writing blob: %+v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("partially written blob")
	}

	m := ispec.Manifest{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		Config: ispec.Descriptor{
			MediaType: ispec.MediaTypeImageIndex,
			Digest:    digest,
			Size:      size,
		},
		Layers: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageIndex,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	data, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("error marshaling json: %+v", err)
	}
	mr := bytes.NewReader(data)
	digest, size, err = engine.PutBlob(ctx, mr)
	if err != nil {
		t.Fatalf("error writing blob: %+v", err)
	}
	if size != int64(len(data)) {
		t.Fatalf("partially written blob")
	}

	idx := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		Manifests: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageIndex,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	if err := engine.PutIndex(ctx, idx); err != nil {
		t.Fatalf("error writing index: %+v", err)
	}

	err = engineExt.GC(ctx)
	if err != nil {
		t.Fatalf("GC failed: %+v", err)
	}

	b, err = engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) != 1 {
		t.Fatalf("expected single-entry blob list after GC")
	}
}

func gcOkFunc(t *testing.T, expectedDigest digest.Digest, unexpectedDigest digest.Digest) GCPolicy {
	return func(ctx context.Context, digest digest.Digest) (bool, error) {
		if digest == "" || digest == unexpectedDigest {
			t.Errorf("got incorrect digest to gc policy callback: unexpected %v", digest)
		}
		if digest != expectedDigest {
			t.Errorf("got incorrect digest to gc policy callback: expected %v, got %v", expectedDigest, digest)
		}
		return true, nil
	}
}

func gcSkipFunc(t *testing.T, expectedDigest digest.Digest) GCPolicy {
	return func(ctx context.Context, digest digest.Digest) (bool, error) {
		if digest != expectedDigest {
			t.Errorf("got incorrect digest to gc policy callback: expected %v, got %v", expectedDigest, digest)
		}
		return false, nil
	}
}

func errFunc(ctx context.Context, digest digest.Digest) (bool, error) {
	return false, fmt.Errorf("err policy")
}

func TestGCWithPolicy(t *testing.T) {
	ctx := context.Background()

	root, err := ioutil.TempDir("", "umoci-TestEngineReference")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(root)

	image := filepath.Join(root, "image")
	if err := dir.Create(image); err != nil {
		t.Fatalf("unexpected error creating image: %+v", err)
	}

	engine, err := dir.Open(image)
	if err != nil {
		t.Fatalf("unexpected error opening image: %+v", err)
	}
	engineExt := NewEngine(engine)
	defer engine.Close()

	// build a orphan blob that should be GC'ed
	content := "this is a orphan blob"
	br := strings.NewReader(content)
	odigest, size, err := engine.PutBlob(ctx, br)
	if err != nil {
		t.Fatalf("error writing blob: %+v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("partially written blob")
	}

	// build a blob, manifest, index that will survive GC
	content = "this is a test blob"
	br = strings.NewReader(content)
	digest, size, err := engine.PutBlob(ctx, br)
	if err != nil {
		t.Fatalf("error writing blob: %+v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("partially written blob")
	}

	digest, size, err = engineExt.PutBlobJSON(ctx,
		ispec.Manifest{
			Versioned: imeta.Versioned{
				SchemaVersion: 2,
			},
			Config: ispec.Descriptor{
				MediaType: ispec.MediaTypeImageLayer,
				Digest:    digest,
				Size:      size,
			},
			Layers: []ispec.Descriptor{
				{
					MediaType: ispec.MediaTypeImageLayer,
					Digest:    digest,
					Size:      size,
				},
			},
		})
	if err != nil {
		t.Fatalf("error writing blob: %+v", err)
	}

	idx := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2,
		},
		Manifests: []ispec.Descriptor{
			{
				MediaType: ispec.MediaTypeImageManifest,
				Digest:    digest,
				Size:      size,
			},
		},
	}
	if err := engine.PutIndex(ctx, idx); err != nil {
		t.Fatalf("error writing index: %+v", err)
	}

	err = engineExt.GC(ctx, errFunc)
	// expect this to fail
	if err == nil {
		t.Fatalf("GC failed: %+v", err)
	}

	err = engineExt.GC(ctx, gcSkipFunc(t, odigest))
	// expect this to succeed but not perform GC
	if err != nil {
		t.Fatalf("GC failed: %+v", err)
	}
	b, err := engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) != 3 {
		t.Fatalf("expected all entries in blob list after skip GC policy")
	}

	err = engineExt.GC(ctx, gcOkFunc(t, odigest, digest))
	// expect this to succeed
	if err != nil {
		t.Fatalf("GC failed: %+v", err)
	}

	b, err = engine.ListBlobs(ctx)
	if err != nil {
		t.Fatalf("unable to list blobs: %+v", err)
	}
	if len(b) != 2 {
		t.Fatalf("expected blob list with two entries after GC")
	}
}
