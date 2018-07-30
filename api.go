package umoci

import (
	"github.com/openSUSE/umoci/oci/cas/dir"
	"github.com/openSUSE/umoci/oci/casext"
	"github.com/pkg/errors"
)

// OpenLayout opens an existing OCI image layout, and fails if it does not
// exist.
func OpenLayout(imagePath string) (casext.Engine, error) {
	// Get a reference to the CAS.
	engine, err := dir.Open(imagePath)
	if err != nil {
		return casext.Engine{}, errors.Wrap(err, "open CAS")
	}

	return casext.NewEngine(engine), nil
}

// CreateLayout creates an existing OCI image layout, and fails if it already
// exists.
func CreateLayout(imagePath string) (casext.Engine, error) {
	err := dir.Create(imagePath)
	if err != nil {
		return casext.Engine{}, err
	}

	return OpenLayout(imagePath)
}
