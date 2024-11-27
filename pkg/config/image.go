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
)

const (
	// Image env vars
	// oktetoBinEnvVar defines the okteto bin image to use
	// This variable is used for the okteto up start script. It runs syncthing and a supervisor
	oktetoBinEnvVar = "OKTETO_BIN"

	// oktetoDeployRemoteImageEnvVar defines okteto cli image used to deploy an environment remotely
	oktetoDeployRemoteImageEnvVar = "OKTETO_REMOTE_CLI_IMAGE"

	// oktetoCLIImageTemplate defines okteto CLI image template to use for remote deployments
	oktetoCLIImageTemplate = "%s:%s"

	// oktetoCliRepository defines the okteto cli repository
	oktetoCliRepository = "okteto/okteto"
)

type ImageConfig struct {
	ioCtrl        Logger
	getEnv        func(string) string
	cliRepository string
}

// Logger is the interface used to log messages
type Logger interface {
	Infof(format string, args ...interface{})
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

// GetBinImage returns the okteto bin image to use
// Bin image is used to run start script in okteto up
func (c *ImageConfig) GetBinImage() string {
	binImage := c.getEnv(oktetoBinEnvVar)
	if binImage != "" {
		c.ioCtrl.Infof("using okteto bin image (from env var): %s", binImage)
		return binImage
	}

	if _, err := semver.StrictNewVersion(VersionString); err == nil {
		c.ioCtrl.Infof("using okteto bin image (from cli version): %s", VersionString)
		return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, VersionString)
	}

	c.ioCtrl.Infof("invalid version string: %s, using latest", VersionString)
	return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, "master")
}

// GetRemoteImage returns the okteto cli image to use for remote deployments
// Remote image is used to run okteto deploy/destroy/test remotely
func (c *ImageConfig) GetRemoteImage(versionString string) string {
	if _, err := semver.StrictNewVersion(versionString); err == nil {
		return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, versionString)
	}
	c.ioCtrl.Infof("invalid version string: %s, using latest stable", versionString)

	remoteOktetoImage := c.getEnv(oktetoDeployRemoteImageEnvVar)
	if remoteOktetoImage != "" {
		return remoteOktetoImage
	}

	return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, "stable")
}

// GetOktetoImage returns the okteto cli image to use for hybrid development environments
func (c *ImageConfig) GetOktetoImage() string {
	if _, err := semver.StrictNewVersion(VersionString); err == nil {
		c.ioCtrl.Infof("using okteto bin image (from cli version): %s", VersionString)
		return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, VersionString)
	}

	c.ioCtrl.Infof("invalid version string: %s, using latest stable", VersionString)
	return fmt.Sprintf(oktetoCLIImageTemplate, c.cliRepository, "stable")
}
