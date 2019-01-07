package analytics

import (
	"io/ioutil"
	"log"
	"os"
	"testing"
)

func Test_isEnabled(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}

	flagPath = tmpfile.Name()

	if isEnabled() {
		t.Error("didn't detect that analytics was disabled")
	}

	os.Remove(tmpfile.Name())

	if !isEnabled() {
		t.Error("didn't detect that analytics was enabled")
	}
}
