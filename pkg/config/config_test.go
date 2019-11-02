package config

import (
	"io/ioutil"
	"os"
	"testing"
)

func TestGetUserHomeDir(t *testing.T) {

	home := GetUserHomeDir()
	if len(home) == 0 {
		t.Fatal("got an empty home value")
	}

	dir, err := ioutil.TempDir("", "")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	os.Setenv("OKTETO_HOME", dir)
	home = GetUserHomeDir()
	if home != dir {
		t.Fatalf("OKTETO_HOME override failed, got %s instead of %s", home, dir)
	}

}
