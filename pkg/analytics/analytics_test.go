package analytics

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/cloudnativedevelopment/cnd/pkg/config"
)

func Test_isEnabled(t *testing.T) {
	if !isEnabled() {
		t.Error("didn't enable analytics by default")
	}

	tmpdir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpdir)

	config.SetConfig(&config.Config{
		CNDHomePath: tmpdir,
	})

	analyticsFlag := filepath.Join(tmpdir, ".cnd", ".noanalytics")
	if analyticsFlag != getFlagPath() {
		t.Errorf("got %s expected %s", getFlagPath(), analyticsFlag)
	}

	if err := ioutil.WriteFile(analyticsFlag, nil, 0700); err != nil {
		t.Fatal(err)
	}

	if isEnabled() {
		t.Error("didn't detect that analytics was disabled by creating the file")
	}

	os.Remove(analyticsFlag)

	if !isEnabled() {
		t.Error("didn't detect that analytics was enabled by deleting the file")
	}
}
