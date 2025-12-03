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
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
)

func TestGetCliImage(t *testing.T) {
	testCases := []struct {
		name          string
		envVars       map[string]string
		versionString string
		expectedImage string
		expectedLogs  []string
	}{
		{
			name: "Environment variable OKTETO_CLI_IMAGE is set",
			envVars: map[string]string{
				oktetoCLIImageEnvVar: "mycustomimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "mycustomimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_CLI_IMAGE): mycustomimage:tag"},
		},
		{
			name: "Environment variable OKTETO_BIN is set (backward compatibility)",
			envVars: map[string]string{
				oktetoBinEnvVar: "mycustomimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "mycustomimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_BIN): mycustomimage:tag"},
		},
		{
			name: "Environment variable OKTETO_REMOTE_CLI_IMAGE is set (backward compatibility)",
			envVars: map[string]string{
				oktetoDeployRemoteImageEnvVar: "mycustomimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "mycustomimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_REMOTE_CLI_IMAGE): mycustomimage:tag"},
		},
		{
			name: "OKTETO_CLI_IMAGE takes precedence over OKTETO_BIN",
			envVars: map[string]string{
				oktetoCLIImageEnvVar: "newimage:tag",
				oktetoBinEnvVar:      "oldimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "newimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_CLI_IMAGE): newimage:tag"},
		},
		{
			name: "OKTETO_CLI_IMAGE takes precedence over OKTETO_REMOTE_CLI_IMAGE",
			envVars: map[string]string{
				oktetoCLIImageEnvVar:          "newimage:tag",
				oktetoDeployRemoteImageEnvVar: "oldimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "newimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_CLI_IMAGE): newimage:tag"},
		},
		{
			name: "OKTETO_BIN takes precedence over OKTETO_REMOTE_CLI_IMAGE",
			envVars: map[string]string{
				oktetoBinEnvVar:               "binimage:tag",
				oktetoDeployRemoteImageEnvVar: "remoteimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "binimage:tag",
			expectedLogs:  []string{"using okteto cli image (from OKTETO_BIN): binimage:tag"},
		},
		{
			name:          "No env vars set, valid VersionString",
			envVars:       map[string]string{},
			versionString: "1.2.3",
			expectedImage: "okteto/okteto:1.2.3",
			expectedLogs:  []string{"using okteto cli image (from cli version): 1.2.3"},
		},
		{
			name:          "No env vars set, invalid VersionString",
			envVars:       map[string]string{},
			versionString: "invalidversion",
			expectedImage: "okteto/okteto:master",
			expectedLogs:  []string{"invalid version string: invalidversion, using latest"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			VersionString = tc.versionString

			// Create ImageConfig with mocked getEnv
			c := &ImageConfig{
				ioCtrl: io.NewIOController(),
				getEnv: func(s string) string {
					if v, ok := tc.envVars[s]; ok {
						return v
					}
					return ""
				},
				cliRepository: "okteto/okteto",
			}

			image := c.GetCliImage()
			assert.Equal(t, tc.expectedImage, image)

		})
	}
}

func TestClusterCliRepositoryOverridesDefault(t *testing.T) {
	// Save original value
	originalClusterRepo := ClusterCliRepository
	defer func() {
		ClusterCliRepository = originalClusterRepo
	}()

	testCases := []struct {
		name          string
		clusterRepo   string
		versionString string
		expectedImage string
	}{
		{
			name:          "ClusterCliRepository from API overrides default",
			clusterRepo:   "custom.registry.io/okteto/cli",
			versionString: "1.2.3",
			expectedImage: "custom.registry.io/okteto/cli:1.2.3",
		},
		{
			name:          "Empty ClusterCliRepository uses default",
			clusterRepo:   "",
			versionString: "1.2.3",
			expectedImage: "okteto/okteto:1.2.3",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			VersionString = tc.versionString
			ClusterCliRepository = tc.clusterRepo

			imageConfig := NewImageConfig(io.NewIOController())
			image := imageConfig.GetCliImage()
			assert.Equal(t, tc.expectedImage, image)
		})
	}
}
