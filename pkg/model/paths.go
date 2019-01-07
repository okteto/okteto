package model

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// GetCNDHome returns the base path for CND config files
func GetCNDHome() string {
	home := path.Join(os.Getenv("HOME"), ".cnd")
	if err := os.MkdirAll(home, 0700); err != nil {
		log.Errorf("failed to create the home directory: %s", err)
	}

	return home
}
