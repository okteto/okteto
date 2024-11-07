// Copyright 2023 The Okteto Authors
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
	"regexp"

	"github.com/okteto/okteto/pkg/log/io"
)

const (
	// versionPattern is the pattern to match a version string
	versionPattern = `\d+\.\d+\.\d+`

	// oktetoDeployRemoteImageEnvVar defines okteto cli image used for deploy an environment remotely
	oktetoDeployRemoteImageEnvVar = "OKTETO_REMOTE_CLI_IMAGE"

	// oktetoCLIImageForRemoteTemplate defines okteto CLI image template to use for remote deployments
	oktetoCLIImageForRemoteTemplate = "okteto/okteto:%s"
)

var (
	// versionRegex is the regex to match a version string
	versionRegex = regexp.MustCompile(versionPattern)
)

type ImageConfig struct {
	ioCtrl *io.Controller
	getEnv func(string) string
}

func NewImageConfig(ioCtrl *io.Controller) *ImageConfig {
	return &ImageConfig{
		ioCtrl: ioCtrl,
		getEnv: os.Getenv,
	}
}

func (c *ImageConfig) GetBinImage() string {
	return ""
}

func (c *ImageConfig) GetRemoteImage(versionString string) string {
	if versionRegex.MatchString(versionString) {
		return fmt.Sprintf(oktetoCLIImageForRemoteTemplate, versionString)
	}
	c.ioCtrl.Infof("invalid version string: %s, using latest", versionString)

	remoteOktetoImage := c.getEnv(oktetoDeployRemoteImageEnvVar)
	if remoteOktetoImage != "" {
		return remoteOktetoImage
	}
	return fmt.Sprintf(oktetoCLIImageForRemoteTemplate, "latest")
}
