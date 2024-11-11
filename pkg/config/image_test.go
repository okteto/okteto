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
	"github.com/stretchr/testify/require"
)

func Test_GetRemoteImage(t *testing.T) {
	var tests = []struct {
		name                                 string
		versionString, expected, cliImageEnv string
	}{
		{
			name:          "no version string and no env return latest",
			versionString: "",
			expected:      "okteto/okteto:latest",
		},
		{
			name:          "no version string return env value",
			versionString: "",
			cliImageEnv:   "okteto/remote:test",
			expected:      "okteto/remote:test",
		},
		{
			name:          "found version string",
			versionString: "2.2.2",
			expected:      "okteto/okteto:2.2.2",
		},
		{
			name:          "found incorrect version string return latest ",
			versionString: "2.a.2",
			expected:      "okteto/okteto:latest",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			if tt.cliImageEnv != "" {
				t.Setenv(oktetoDeployRemoteImageEnvVar, tt.cliImageEnv)
			}

			version := NewImageConfig(io.NewIOController()).GetRemoteImage(tt.versionString)
			require.Equal(t, version, tt.expected)
		})
	}
}

func TestGetBinImage(t *testing.T) {
	testCases := []struct {
		name          string
		envVars       map[string]string
		versionString string
		expectedImage string
		expectedLogs  []string
	}{
		{
			name: "Environment variable OKTETO_BIN is set",
			envVars: map[string]string{
				oktetoBinEnvVar: "mycustomimage:tag",
			},
			versionString: "1.2.3",
			expectedImage: "mycustomimage:tag",
			expectedLogs:  []string{"using okteto bin image (from env var): mycustomimage:tag"},
		},
		{
			name: "Environment variable OKTETO_BIN is set to beta",
			envVars: map[string]string{
				oktetoBinEnvVar: "mycustomimage:tag",
			},
			versionString: "1.2.3-beta.1",
			expectedImage: "mycustomimage:tag",
			expectedLogs:  []string{"using okteto bin image (from env var): mycustomimage:tag"},
		},
		{
			name:          "OKTETO_BIN not set, valid VersionString",
			envVars:       map[string]string{},
			versionString: "1.2.3",
			expectedImage: "okteto/okteto:1.2.3",
			expectedLogs:  []string{"using okteto bin image (from cli version): 1.2.3"},
		},
		{
			name:          "OKTETO_BIN not set, invalid VersionString",
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
			}

			image := c.GetBinImage()
			assert.Equal(t, tc.expectedImage, image)

		})
	}
}

func TestGetOktetoImage(t *testing.T) {
	testCases := []struct {
		name          string
		versionString string
		expectedImage string
		expectedLogs  []string
	}{
		{
			name:          "VersionString valid",
			versionString: "1.2.3",
			expectedImage: "okteto/okteto:1.2.3",
			expectedLogs:  []string{"using okteto bin image (from cli version): 1.2.3"},
		},
		{
			name:          "VersionString invalid",
			versionString: "invalidversion",
			expectedImage: "okteto/okteto:stable",
			expectedLogs:  []string{"invalid version string: invalidversion, using latest stable"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			VersionString = tc.versionString

			// Create ImageConfig with mocked getEnv
			c := &ImageConfig{
				ioCtrl: io.NewIOController(),
				getEnv: func(s string) string {
					return ""
				},
			}

			image := c.GetOktetoImage()
			assert.Equal(t, tc.expectedImage, image)

		})
	}
}
