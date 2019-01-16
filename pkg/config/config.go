package config

import (
	"os"
	"path"

	log "github.com/sirupsen/logrus"
)

// These values will be stamped at build time
var (
	// VersionString is the canonical version string
	VersionString string
)

// Config holds all the configuration values.
// This is meant to be changed by implementors of CND.
type Config struct {
	// AnalyticsEndpoint is the endpoint to use for analytics
	AnalyticsEndpoint string

	//
	// CNDFolderName is the name of the base folder that stores the state on the client machine
	CNDFolderName string

	// CNDManifestFileName is the name of the manifest file e.g. cnd.yml
	CNDManifestFileName string

	// BinaryName is the name of the CLI binary
	BinaryName string
}

var (
	defaultConfig = &Config{
		AnalyticsEndpoint:   "https://us-central1-okteto-prod.cloudfunctions.net/cnd-analytics",
		CNDFolderName:       ".cnd",
		CNDManifestFileName: "cnd.yml",
		BinaryName:          "cnd",
	}

	overrideConfig = &Config{}
)

//SetConfig sets the configuration object to use
func SetConfig(newConfig *Config) {
	overrideConfig = newConfig
}

// GetAnalyticsEndpoint returns the endpoint to use for analytics
func GetAnalyticsEndpoint() string {
	if overrideConfig.AnalyticsEndpoint == "" {
		return defaultConfig.AnalyticsEndpoint
	}

	return overrideConfig.AnalyticsEndpoint
}

// CNDFolderName returns the name of the cnd folder
func CNDFolderName() string {
	if overrideConfig.CNDFolderName == "" {
		return defaultConfig.CNDFolderName
	}

	return overrideConfig.CNDFolderName
}

// CNDManifestFileName returns the name of the manifest file
func CNDManifestFileName() string {
	if overrideConfig.CNDManifestFileName == "" {
		return defaultConfig.CNDManifestFileName
	}

	return overrideConfig.CNDManifestFileName
}

//GetBinaryName returns the name of the binary
func GetBinaryName() string {
	if overrideConfig.BinaryName == "" {
		return defaultConfig.BinaryName
	}

	return overrideConfig.BinaryName
}

// GetCNDHome returns the path of the folder
func GetCNDHome() string {
	var cndFolder = defaultConfig.CNDFolderName
	if overrideConfig.CNDFolderName != "" {
		cndFolder = overrideConfig.CNDFolderName
	}

	home := path.Join(os.Getenv("HOME"), cndFolder)

	if err := os.MkdirAll(home, 0700); err != nil {
		log.Errorf("failed to create the home directory: %s", err)
	}

	return home
}
