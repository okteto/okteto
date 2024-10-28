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

package buildkit

import (
	"testing"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

func TestGetFrontend(t *testing.T) {
	// Define test cases
	tests := []struct {
		expectedError    error
		buildOptions     *types.BuildOptions
		expectedFrontend *Frontend
		name             string
		envValue         string
	}{
		{
			name:             "Nil BuildOptions",
			buildOptions:     nil,
			envValue:         "",
			expectedFrontend: nil,
			expectedError:    errOptionsIsNil,
		},
		{
			name: "No ExtraHosts, No Custom Env",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{},
			},
			envValue: "",
			expectedFrontend: &Frontend{
				Frontend: defaultFrontend,
				Image:    "",
			},
			expectedError: nil,
		},
		{
			name: "ExtraHosts Present, No Custom Env",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{
					{Hostname: "host1", IP: "192.168.1.1"},
				},
			},
			envValue: "",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    dockerFrontendImage,
			},
			expectedError: nil,
		},
		{
			name: "Custom Frontend Image via Env, No ExtraHosts",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{},
			},
			envValue: "custom/image:latest",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    "custom/image:latest",
			},
			expectedError: nil,
		},
		{
			name: "ExtraHosts and Custom Frontend Image via Env",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{
					{Hostname: "host1", IP: "192.168.1.1"},
				},
			},
			envValue: "custom/image:latest",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    "custom/image:latest",
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			getEnv := func(key string) string {
				if key == buildkitFrontendImageEnvVar {
					return tt.envValue
				}
				return ""
			}
			retriever := NewFrontendRetriever(getEnv, io.NewIOController())

			frontend, err := retriever.GetFrontend(tt.buildOptions)

			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError, "expected error to match")
				assert.Nil(t, frontend, "expected frontend to be nil")
			} else {
				assert.NoError(t, err, "expected no error")
				assert.Equal(t, tt.expectedFrontend, frontend, "expected frontend to match")
			}
		})
	}
}
