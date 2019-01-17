package analytics

import (
	"io/ioutil"
	"log"
	"os"
	"testing"

	"github.com/okteto/cnd/pkg/config"
)

func Test_isEnabled(t *testing.T) {
	tmpfile, err := ioutil.TempFile("", "")
	if err != nil {
		log.Fatal(err)
	}

	config.SetConfig(&config.Config{
		CNDHomePath: tmpfile.Name(),
	})

	if isEnabled() {
		t.Error("didn't detect that analytics was disabled")
	}

	os.Remove(tmpfile.Name())

	if !isEnabled() {
		t.Error("didn't detect that analytics was enabled")
	}
}
