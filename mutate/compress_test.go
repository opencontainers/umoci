package mutate

import (
	"bytes"
	"io/ioutil"
	"testing"

	gzip "github.com/klauspost/pgzip"
	"github.com/stretchr/testify/assert"
)

const (
	fact = "meshuggah rocks!!!"
)

func TestNoopCompressor(t *testing.T) {
	assert := assert.New(t)
	buf := bytes.NewBufferString(fact)

	r, err := NoopCompressor.Compress(buf)
	assert.NoError(err)
	assert.Equal(NoopCompressor.MediaTypeSuffix(), "")

	content, err := ioutil.ReadAll(r)
	assert.NoError(err)

	assert.Equal(string(content), fact)
}

func TestGzipCompressor(t *testing.T) {
	assert := assert.New(t)

	buf := bytes.NewBufferString(fact)
	c := GzipCompressor

	r, err := c.Compress(buf)
	assert.NoError(err)
	assert.Equal(c.MediaTypeSuffix(), "gzip")

	r, err = gzip.NewReader(r)
	assert.NoError(err)

	content, err := ioutil.ReadAll(r)
	assert.NoError(err)

	assert.Equal(string(content), fact)
}
