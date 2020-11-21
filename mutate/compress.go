package mutate

import (
	"io"
	"io/ioutil"
	"runtime"

	zstd "github.com/klauspost/compress/zstd"
	gzip "github.com/klauspost/pgzip"
	"github.com/pkg/errors"
)

// Compressor is an interface which users can use to implement different
// compression types.
type Compressor interface {
	// Compress sets up the streaming compressor for this compression type.
	Compress(io.Reader) (io.ReadCloser, error)

	// MediaTypeSuffix returns the suffix to be added to the layer to
	// indicate what compression type is used, e.g. "gzip", or "" for no
	// compression.
	MediaTypeSuffix() string
}

type noopCompressor struct{}

func (nc noopCompressor) Compress(r io.Reader) (io.ReadCloser, error) {
	return ioutil.NopCloser(r), nil
}

func (nc noopCompressor) MediaTypeSuffix() string {
	return ""
}

// NoopCompressor provides no compression.
var NoopCompressor Compressor = noopCompressor{}

// GzipCompressor provides gzip compression.
var GzipCompressor Compressor = gzipCompressor{}

type gzipCompressor struct{}

func (gz gzipCompressor) Compress(reader io.Reader) (io.ReadCloser, error) {
	pipeReader, pipeWriter := io.Pipe()

	gzw := gzip.NewWriter(pipeWriter)
	if err := gzw.SetConcurrency(256<<10, 2*runtime.NumCPU()); err != nil {
		return nil, errors.Wrapf(err, "set concurrency level to %v blocks", 2*runtime.NumCPU())
	}
	go func() {
		if _, err := io.Copy(gzw, reader); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "compressing layer"))
		}
		if err := gzw.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close gzip writer"))
		}
		if err := pipeWriter.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close pipe writer"))
		}
	}()

	return pipeReader, nil
}

func (gz gzipCompressor) MediaTypeSuffix() string {
	return "gzip"
}

// ZstdCompressor provides zstd compression.
var ZstdCompressor Compressor = zstdCompressor{}

type zstdCompressor struct{}

func (zs zstdCompressor) Compress(reader io.Reader) (io.ReadCloser, error) {

	pipeReader, pipeWriter := io.Pipe()
	zenc, err := zstd.NewWriter(pipeWriter)
	if err != nil {
		return nil, err
	}
	go func() {
		if _, err := io.Copy(zenc, reader); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "compressing layer"))
		}
		if err := zenc.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close gzip writer"))
		}
		if err := pipeWriter.Close(); err != nil {
			// #nosec G104
			_ = pipeWriter.CloseWithError(errors.Wrap(err, "close pipe writer"))
		}
	}()

	return pipeReader, nil
}

func (zs zstdCompressor) MediaTypeSuffix() string {
	return "zstd"
}
