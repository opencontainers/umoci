//go:build linux
// +build linux

package layer

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"github.com/opencontainers/umoci/pkg/fseval"
	"github.com/opencontainers/umoci/pkg/system"
	"github.com/stretchr/testify/assert"
	"github.com/vbatts/go-mtree"
	"golang.org/x/sys/unix"
)

func TestInsertLayerTranslateOverlayWhiteouts(t *testing.T) {
	assert := assert.New(t)
	dir, err := ioutil.TempDir("", "umoci-TestTranslateOverlayWhiteouts")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	mknodOk, err := canMknod(dir)
	if err != nil {
		t.Fatalf("couldn't mknod in dir: %v", err)
	}

	if !mknodOk {
		t.Skip("skipping overlayfs test on kernel < 5.8")
	}

	testNode := path.Join(dir, "test")
	err = system.Mknod(testNode, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	assert.NoError(err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}
	reader := GenerateInsertLayer(dir, "/", false, &packOptions)
	defer reader.Close()

	tr := tar.NewReader(reader)
	hdr, err := tr.Next()
	assert.NoError(err)
	assert.Equal(hdr.Name, "/")

	hdr, err = tr.Next()
	assert.NoError(err)

	assert.Equal(int32(hdr.Typeflag), int32(tar.TypeReg))
	assert.Equal(hdr.Name, whPrefix+"test")
	_, err = tr.Next()
	assert.Equal(err, io.EOF)
}

func TestGenerateLayerTranslateOverlayWhiteouts(t *testing.T) {
	assert := assert.New(t)
	dir, err := ioutil.TempDir("", "umoci-TestTranslateOverlayWhiteouts")
	assert.NoError(err)
	defer os.RemoveAll(dir)

	mknodOk, err := canMknod(dir)
	if err != nil {
		t.Fatalf("couldn't mknod in dir: %v", err)
	}

	if !mknodOk {
		t.Skip("skipping overlayfs test on kernel < 5.8")
	}

	testNode := path.Join(dir, "test")
	err = system.Mknod(testNode, unix.S_IFCHR|0666, unix.Mkdev(0, 0))
	assert.NoError(err)

	packOptions := RepackOptions{TranslateOverlayWhiteouts: true}
	// something reasonable
	mtreeKeywords := []mtree.Keyword{
		"size",
		"type",
		"uid",
		"gid",
		"mode",
	}
	deltas, err := mtree.Check(dir, nil, mtreeKeywords, fseval.Default)
	assert.NoError(err)

	reader, err := GenerateLayer(dir, deltas, &packOptions)
	assert.NoError(err)
	defer reader.Close()

	tr := tar.NewReader(reader)

	hdr, err := tr.Next()
	assert.NoError(err)

	assert.Equal(int32(hdr.Typeflag), int32(tar.TypeReg))
	assert.Equal(path.Base(hdr.Name), whPrefix+"test")
	_, err = tr.Next()
	assert.Equal(err, io.EOF)
}
