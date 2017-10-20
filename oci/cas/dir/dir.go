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

package dir

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/apex/log"
	"github.com/openSUSE/umoci/oci/cas"
	"github.com/opencontainers/go-digest"
	imeta "github.com/opencontainers/image-spec/specs-go"
	ispec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/pkg/errors"
	"github.com/wking/casengine"
	"github.com/wking/casengine/counter"
	"github.com/wking/casengine/dir"
	"github.com/xiekeyang/oci-discovery/tools/refenginediscovery"
	"golang.org/x/net/context"
	"golang.org/x/sys/unix"
)

const (
	// blobDirectory is the directory inside an OCI image that contains blobs.
	//
	// FIXME: if the URI Template currently hard-coded Open() changes,
	// then this variable will no longer be meaningful, and its consumers
	// will have to be updated to use other logic.
	blobDirectory = "blobs"

	// indexFile is the file inside an OCI image that contains the top-level
	// index.
	indexFile = "index.json"

	// layoutFile is the file in side an OCI image the indicates what version
	// of the OCI spec the image is.
	layoutFile = "oci-layout"
)

type dirEngine struct {
	cas                casengine.Engine
	digestListerEngine casengine.DigestListerEngine
	path               string
	temp               string
	tempFile           *os.File
}

func (e *dirEngine) ensureTempDir() error {
	if e.temp == "" {
		tempDir, err := ioutil.TempDir(e.path, ".umoci-")
		if err != nil {
			return errors.Wrap(err, "create tempdir")
		}

		// We get an advisory lock to ensure that GC() won't delete our
		// temporary directory here. Once we get the lock we know it won't do
		// anything until we unlock it or exit.

		e.tempFile, err = os.Open(tempDir)
		if err != nil {
			return errors.Wrap(err, "open tempdir for lock")
		}
		if err := unix.Flock(int(e.tempFile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			return errors.Wrap(err, "lock tempdir")
		}

		e.temp = tempDir
	}
	return nil
}

// CAS returns the casengine.Engine backing this engine.
func (e *dirEngine) CAS() (casEngine casengine.Engine) {
	return e.cas
}

// DigestListerEngine returns the casengine.DigestListerEngine backing
// this engine.
func (e *dirEngine) DigestListerEngine() (casEngine casengine.DigestListerEngine) {
	return e.digestListerEngine
}

// PutBlob adds a new blob to the image. This is idempotent; a nil error
// means that "the content is stored at DIGEST" without implying "because
// of this PutBlob() call".
//
// Deprecated: Use CAS().Put instead.
func (e *dirEngine) PutBlob(ctx context.Context, reader io.Reader) (digest.Digest, int64, error) {
	counter := &counter.Counter{}
	countedReader := io.TeeReader(reader, counter)
	dig, err := e.cas.Put(ctx, cas.BlobAlgorithm, countedReader)
	return dig, int64(counter.Count()), err
}

// GetBlob returns a reader for retrieving a blob from the image, which the
// caller must Close(). Returns os.ErrNotExist if the digest is not found.
//
// Deprecated: Use CAS().Get instead.
func (e *dirEngine) GetBlob(ctx context.Context, digest digest.Digest) (io.ReadCloser, error) {
	return e.cas.Get(ctx, digest)
}

// PutIndex sets the index of the OCI image to the given index, replacing the
// previously existing index. This operation is atomic; any readers attempting
// to access the OCI image while it is being modified will only ever see the
// new or old index.
func (e *dirEngine) PutIndex(ctx context.Context, index ispec.Index) error {
	if err := e.ensureTempDir(); err != nil {
		return errors.Wrap(err, "ensure tempdir")
	}

	// We copy this into a temporary index to ensure the atomicity of this
	// operation.
	fh, err := ioutil.TempFile(e.temp, "index-")
	if err != nil {
		return errors.Wrap(err, "create temporary index")
	}
	tempPath := fh.Name()
	defer fh.Close()

	// Encode the index.
	if err := json.NewEncoder(fh).Encode(index); err != nil {
		return errors.Wrap(err, "write temporary index")
	}
	fh.Close()

	// Move the blob to its correct path.
	path := filepath.Join(e.path, indexFile)
	if err := os.Rename(tempPath, path); err != nil {
		return errors.Wrap(err, "rename temporary index")
	}
	return nil
}

// GetIndex returns the index of the OCI image. Return ErrNotExist if the
// digest is not found. If the image doesn't have an index, ErrInvalid is
// returned (a valid OCI image MUST have an image index).
//
// It is not recommended that users of cas.Engine use this interface directly,
// due to the complication of properly handling references as well as correctly
// handling nested indexes. casext.Engine provides a wrapper for cas.Engine
// that implements various reference resolution functions that should work for
// most users.
func (e *dirEngine) GetIndex(ctx context.Context) (ispec.Index, error) {
	content, err := ioutil.ReadFile(filepath.Join(e.path, indexFile))
	if err != nil {
		if os.IsNotExist(err) {
			err = cas.ErrInvalid
		}
		return ispec.Index{}, errors.Wrap(err, "read index")
	}

	var index ispec.Index
	if err := json.Unmarshal(content, &index); err != nil {
		return ispec.Index{}, errors.Wrap(err, "parse index")
	}

	return index, nil
}

// DeleteBlob removes a blob from the image. This is idempotent; a nil
// error means "the content is not in the store" without implying "because
// of this DeleteBlob() call".
//
// Deprecated: Use CAS().Delete instead.
func (e *dirEngine) DeleteBlob(ctx context.Context, digest digest.Digest) error {
	return e.cas.Delete(ctx, digest)
}

// ListBlobs returns the set of blob digests stored in the image.
//
// Deprecated: Use DigestListerEngine().Digests instead.
func (e *dirEngine) ListBlobs(ctx context.Context) ([]digest.Digest, error) {
	if e.digestListerEngine == nil {
		return nil, fmt.Errorf("cannot list blobs without a DigestListerEngine")
	}

	digests := []digest.Digest{}
	err := e.digestListerEngine.Digests(ctx, "", "", -1, 0, func(ctx context.Context, digest digest.Digest) (err error) {
		digests = append(digests, digest)
		return nil
	})
	if err != nil {
		return nil, err
	}

	return digests, nil
}

// Clean executes a garbage collection of any non-blob garbage in the store
// (this includes temporary files and directories not reachable from the CAS
// interface). This MUST NOT remove any blobs or references in the store.
func (e *dirEngine) Clean(ctx context.Context) error {
	// Remove every .umoci directory that isn't flocked.
	matches, err := filepath.Glob(filepath.Join(e.path, ".umoci-*"))
	if err != nil {
		return errors.Wrap(err, "glob .umoci-*")
	}
	for _, path := range matches {
		err = e.cleanPath(ctx, path)
		if err != nil && err != filepath.SkipDir {
			return err
		}
	}

	return nil
}

func (e *dirEngine) cleanPath(ctx context.Context, path string) error {
	cfh, err := os.Open(path)
	if err != nil && !os.IsNotExist(err) {
		return errors.Wrap(err, "open for locking")
	}
	defer cfh.Close()

	if err := unix.Flock(int(cfh.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		// If we fail to get a flock(2) then it's probably already locked,
		// so we shouldn't touch it.
		return filepath.SkipDir
	}
	defer unix.Flock(int(cfh.Fd()), unix.LOCK_UN)

	if err := os.RemoveAll(path); os.IsNotExist(err) {
		return nil // somebody else beat us to it
	} else if err != nil {
		log.Warnf("failed to remove %s: %v", path, err)
		return filepath.SkipDir
	}
	log.Debugf("cleaned %s", path)

	return nil
}

// Close releases all references held by the e. Subsequent operations may
// fail.
func (e *dirEngine) Close() (err error) {
	ctx := context.Background()
	var err2 error
	if e.cas != nil {
		if err2 = e.cas.Close(ctx); err2 != nil {
			err = errors.Wrap(err, "close CAS")
		}
	}

	if e.temp != "" {
		if err2 := unix.Flock(int(e.tempFile.Fd()), unix.LOCK_UN); err2 != nil {
			err2 = errors.Wrap(err2, "unlock tempdir")
			if err == nil {
				err = err2
			}
		}
		if err2 := e.tempFile.Close(); err2 != nil {
			err2 = errors.Wrap(err2, "close tempdir")
			if err == nil {
				err = err2
			}
		}
		if err2 := os.RemoveAll(e.temp); err != nil {
			err2 = errors.Wrap(err2, "remove tempdir")
			if err == nil {
				err = err2
			}
		}
	}
	return err
}

// Open opens a new reference to the directory-backed OCI image
// referenced by the provided path.  If your image configures a custom
// blob URI, use OpenWithDigestLister instead.
func Open(path string) (engine cas.Engine, err error) {
	return OpenWithDigestLister(path, nil)
}

// OpenWithDigestLister opens a new reference to the directory-backed
// OCI image referenced by the provided path.  Use this function
// instead of Open when your image configures a custom blob URI.
func OpenWithDigestLister(path string, getDigest dir.GetDigest) (engine cas.Engine, err error) {
	ctx := context.Background()

	configBytes, err := ioutil.ReadFile(filepath.Join(path, layoutFile))
	if err != nil {
		if os.IsNotExist(err) {
			err = cas.ErrInvalid
		}
		return nil, errors.Wrap(err, "read oci-layout")
	}

	var ociLayout ispec.ImageLayout
	if err := json.Unmarshal(configBytes, &ociLayout); err != nil {
		return nil, errors.Wrap(err, "parse oci-layout")
	}

	uri := "blobs/{algorithm}/{encoded}"

	// XXX: Currently the meaning of this field is not adequately defined by
	//      the spec, nor is the "official" value determined by the spec.
	switch ociLayout.Version {
	case "1.0.0": // nothing to configure here
	case "1.1.0":
		var engines refenginediscovery.Engines
		if err := json.Unmarshal(configBytes, &engines); err != nil {
			return nil, errors.Wrap(err, "parse oci-layout")
		}
		for _, config := range engines.CASEngines {
			if config.Protocol == "oci-cas-template-v1" {
				uriInterface, ok := config.Data["uri"]
				if !ok {
					return nil, fmt.Errorf("CAS-template config missing required 'uri' property: %v", config.Data)
				}

				uri, ok = uriInterface.(string)
				if !ok {
					return nil, fmt.Errorf("CAS-template config 'uri' is not a string: %v", uriInterface)
				}

				break
			}
		}
	default:
		return nil, errors.Wrap(cas.ErrInvalid, fmt.Sprintf("layout version %s is not supported", ociLayout.Version))
	}

	if uri == "blobs/{algorithm}/{encoded}" {
		getDigest, err = defaultGetDigest()
		if err != nil {
			return nil, err
		}
	}

	var casEngine casengine.Engine
	var digestListerEngine casengine.DigestListerEngine
	if getDigest == nil {
		casEngine, err = dir.NewEngine(ctx, path, uri)
		if err != nil {
			return nil, errors.Wrap(err, "initialize CAS engine")
		}
	} else {
		digestListerEngine, err = dir.NewDigestListerEngine(ctx, path, uri, getDigest)
		if err != nil {
			return nil, errors.Wrap(err, "initialize CAS engine")
		}
		casEngine = digestListerEngine
	}
	defer func() {
		if err != nil {
			casEngine.Close(ctx)
		}
	}()

	// Check that "blobs" and "index.json" exist in the image.
	if fi, err := os.Stat(filepath.Join(path, blobDirectory)); err != nil {
		if os.IsNotExist(err) {
			err = cas.ErrInvalid
		}
		return nil, errors.Wrap(err, "check blobdir")
	} else if !fi.IsDir() {
		return nil, errors.Wrap(cas.ErrInvalid, "blobdir is not a directory")
	}

	if fi, err := os.Stat(filepath.Join(path, indexFile)); err != nil {
		if os.IsNotExist(err) {
			err = cas.ErrInvalid
		}
		return nil, errors.Wrap(err, "check index")
	} else if fi.IsDir() {
		return nil, errors.Wrap(cas.ErrInvalid, "index is a directory")
	}

	return &dirEngine{
		cas:                casEngine,
		digestListerEngine: digestListerEngine,
		path:               path,
		temp:               "",
	}, nil
}

func defaultGetDigest() (getDigest dir.GetDigest, err error) {
	pattern := `^blobs/(?P<algorithm>[a-z0-9+._-]+)/(?P<encoded>[a-zA-Z0-9=_-]{1,})$`
	if filepath.Separator != '/' {
		if filepath.Separator == '\\' {
			pattern = strings.Replace(pattern, "/", `\\`, -1)
		} else {
			return nil, fmt.Errorf("unknown path separator %q", string(filepath.Separator))
		}
	}

	getDigestRegexp, err := regexp.Compile(pattern)
	if err != nil {
		return nil, errors.Wrap(err, "get-digest regexp")
	}

	regexpGetDigest := &dir.RegexpGetDigest{
		Regexp: getDigestRegexp,
	}

	return regexpGetDigest.GetDigest, nil
}

// Create creates a new OCI image layout at the given path. If the path already
// exists, os.ErrExist is returned. However, all of the parent components of
// the path will be created if necessary.
func Create(path string, uri string) error {
	// We need to fail if path already exists, but we first create all of the
	// parent paths.
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return errors.Wrap(err, "mkdir parent")
		}
	}
	if err := os.Mkdir(path, 0755); err != nil {
		return errors.Wrap(err, "mkdir")
	}

	// Create the necessary directories and "oci-layout" file.
	if err := os.Mkdir(filepath.Join(path, blobDirectory), 0755); err != nil {
		return errors.Wrap(err, "mkdir blobdir")
	}
	if err := os.Mkdir(filepath.Join(path, blobDirectory, cas.BlobAlgorithm.String()), 0755); err != nil {
		return errors.Wrap(err, "mkdir algorithm")
	}

	indexFh, err := os.Create(filepath.Join(path, indexFile))
	if err != nil {
		return errors.Wrap(err, "create index.json")
	}
	defer indexFh.Close()

	defaultIndex := ispec.Index{
		Versioned: imeta.Versioned{
			SchemaVersion: 2, // FIXME: This is hardcoded at the moment.
		},
	}
	if err := json.NewEncoder(indexFh).Encode(defaultIndex); err != nil {
		return errors.Wrap(err, "encode index.json")
	}

	layoutFh, err := os.Create(filepath.Join(path, layoutFile))
	if err != nil {
		return errors.Wrap(err, "create oci-layout")
	}
	defer layoutFh.Close()

	var ociLayout interface{}
	switch uri {
	case "":
		ociLayout = ispec.ImageLayout{
			Version: "1.0.0",
		}
	default:
		ociLayout = map[string]interface{}{
			"imageLayoutVersion": "1.1.0",
			"casEngines": []map[string]interface{}{
				{
					"protocol": "oci-cas-template-v1",
					"uri":      uri,
				},
			},
		}
	}
	if err := json.NewEncoder(layoutFh).Encode(ociLayout); err != nil {
		return errors.Wrap(err, "encode oci-layout")
	}

	// Everything is now set up.
	return nil
}
