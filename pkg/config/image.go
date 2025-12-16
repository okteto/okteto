// Copyright 2024 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package config

import (
	"fmt"
	"os"

	"github.com/Masterminds/semver/v3"
)

var (
	// ClusterCliRepository defines the okteto cli repository for all the operations.
	// This env var is set when executing the okteto context at the beginning of any command
	ClusterCliRepository = ""

	// cachedCliImage stores the computed CLI image to avoid re-evaluation and duplicate warnings
	cachedCliImage = ""
)

const (
	// Image env vars
	// oktetoCLIImageEnvVar defines the okteto cli image to use for all operations
	// This is the primary environment variable that replaces OKTETO_BIN and OKTETO_REMOTE_CLI_IMAGE
	oktetoCLIImageEnvVar = "OKTETO_CLI_IMAGE"

	// oktetoBinEnvVar defines the okteto bin image to use (deprecated, use OKTETO_CLI_IMAGE instead)
	// This variable is used for the okteto up start script. It runs syncthing and a supervisor
	// Kept for backward compatibility
	oktetoBinEnvVar = "OKTETO_BIN"

	// oktetoDeployRemoteImageEnvVar defines okteto cli image used to deploy an environment remotely (deprecated, use OKTETO_CLI_IMAGE instead)
	// Kept for backward compatibility
	oktetoDeployRemoteImageEnvVar = "OKTETO_REMOTE_CLI_IMAGE"

	// oktetoCLIImageTemplate defines okteto CLI image template to use for remote deployments
	oktetoCLIImageTemplate = "%s:%s"

	// oktetoCliRepository defines the okteto cli repository
	oktetoCliRepository = "ghcr.io/okteto/okteto"
)

type ImageConfig struct {
	ioCtrl        Logger
	getEnv        func(string) string
	cliRepository string
}

// Logger is the interface used to log messages
type Logger interface {
	Infof(format string, args ...interface{})
	Warning(format string, args ...interface{})
}

// NewImageConfig creates a new ImageConfig instance
// ImageConfig is used to get the correct image during the code generation
func NewImageConfig(ioCtrl Logger) *ImageConfig {
	cliImage := oktetoCliRepository
	if ClusterCliRepository != "" {
		cliImage = ClusterCliRepository
	}
	return &ImageConfig{
		ioCtrl:        ioCtrl,
		getEnv:        os.Getenv,
		cliRepository: cliImage,
	}
}

// GetCliImage returns the okteto cli image to use
// This is used for all okteto operations including bin image for okteto up start script
func (c *ImageConfig) GetCliImage() string {
	// Return cached value if already computed
	if cachedCliImage != "" {
		return cachedCliImage
	}

	// Check new unified env var first
	cliImage := c.getEnv(oktetoCLIImageEnvVar)
	if cliImage != "" {
		c.ioCtrl.Infof("using okteto cli image (from OKTETO_CLI_IMAGE): %s", cliImage)
		cachedCliImage = cliImage
		return cachedCliImage
	}

	// Fall back to legacy OKTETO_BIN for backward compatibility
	binImage := c.getEnv(oktetoBinEnvVar)
	if binImage != "" {
		c.ioCtrl.Warning("Using Okteto CLI image '%s' from the OKTETO_BIN environment variable\n    OKTETO_BIN is deprecated, please use OKTETO_CLI_IMAGE instead", binImage)
		cachedCliImage = binImage
		return cachedCliImage
	}

	// Fall back to legacy OKTETO_REMOTE_CLI_IMAGE for backward compatibility
	remoteImage := c.getEnv(oktetoDeployRemoteImageEnvVar)
	if remoteImage != "" {
		c.ioCtrl.Warning("Using Okteto CLI image '%s' from the OKTETO_REMOTE_CLI_IMAGE environment variable\n    OKTETO_REMOTE_CLI_IMAGE is deprecated, please use OKTETO_CLI_IMAGE instead", remoteImage)
		cachedCliImage = remoteImage
		return cachedCliImage
	}

	if _, err := semver.StrictNewVersion(VersionString); err == nil {
		c.ioCtrl.Infof("using okteto cli image (from cli version): %s", VersionString)
		cachedCliImage = fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, VersionString)
		return cachedCliImage
	}

	c.ioCtrl.Infof("invalid version string: %s, using latest", VersionString)
	cachedCliImage = fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, "master")
	return cachedCliImage
}
