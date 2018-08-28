package umoci

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestCreateExistingFails(t *testing.T) {
	dir, err := ioutil.TempDir("", "umoci_testCreateExistingFails")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// opening a bad layout should fail
	_, err = OpenLayout(dir)
	if err == nil {
		t.Fatal("opening non-existent layout succeeded?")
	}

	// remove directory so that create can create it
	os.RemoveAll(dir)

	// create should work
	_, err = CreateLayout(dir)
	if err != nil {
		t.Fatal(err)
	}

	// but not twice
	_, err = CreateLayout(dir)
	if err == nil {
		t.Fatal("create worked twice?")
	}

	// but open should work now
	_, err = OpenLayout(dir)
	if err != nil {
		t.Fatal(err)
	}
}
