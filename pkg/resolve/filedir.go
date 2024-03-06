package resolve

import (
	"os"
	"path"

	"github.com/mitchellh/go-homedir"
)

// versionedBinDir returns the directory where versioned bins
// are stored
func versionedBinDir() string {
	return path.Join(oktetoHome(), "bin")
}

// oktetoContextFilename returns the okteto context config file location
func oktetoContextFilename() string {
	return path.Join(oktetoHome(), "context", "config.json")
}

// oktetoHome return the okteto home directory
func oktetoHome() string {
	if okHome, ok := os.LookupEnv("OKTETO_HOME"); ok {
		return okHome
	}
	home, _ := homedir.Dir()
	return path.Join(home, ".okteto")
}
