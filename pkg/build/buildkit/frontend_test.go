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
	"context"
	"testing"

	"github.com/moby/buildkit/client"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
)

type fakeBuildkitInfoGetter struct {
	info *client.Info
	err  error
}

func (f *fakeBuildkitInfoGetter) Info(ctx context.Context) (*client.Info, error) {
	return f.info, f.err
}

func TestGetFrontend(t *testing.T) {
	tests := []struct {
		expectedError    error
		buildOptions     *types.BuildOptions
		expectedFrontend *Frontend
		name             string
		frontendImage    string
		buildkitVersion  string
	}{
		{
			name: "No ExtraHosts, No Custom Env",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{},
			},
			frontendImage: "",
			expectedFrontend: &Frontend{
				Frontend: defaultFrontend,
				Image:    "",
			},
			expectedError: nil,
		},
		{
			name: "No ExtraHosts, No Custom ",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{},
			},
			frontendImage: "",
			expectedFrontend: &Frontend{
				Frontend: defaultFrontend,
				Image:    "",
			},
			expectedError: nil,
		},
		{
			name: "ExtraHosts Present, No Custom Env, No buildkitVersion",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{
					{Hostname: "host1", IP: "192.168.1.1"},
				},
			},
			frontendImage: "",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    defaultDockerFrontendImage,
			},
			expectedError: nil,
		},
		{
			name: "ExtraHosts Present, Use DockerfileV0",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{
					{Hostname: "host1", IP: "192.168.1.1"},
				},
			},
			frontendImage: "",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    defaultDockerFrontendImage,
			},
			expectedError: nil,
		},
		{
			name: "Custom Frontend Image via Env, No ExtraHosts",
			buildOptions: &types.BuildOptions{
				ExtraHosts: []types.HostMap{},
			},
			frontendImage: "custom/image:latest",
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
			frontendImage: "custom/image:latest",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    "custom/image:latest",
			},
			expectedError: nil,
		},
		{
			name: "ExportCache and Custom Frontend Image via Env",
			buildOptions: &types.BuildOptions{
				ExportCache: []string{"cache1", "cache2"},
			},
			buildkitVersion: "0.11.0",
			frontendImage:   "",
			expectedFrontend: &Frontend{
				Frontend: gatewayFrontend,
				Image:    "docker/dockerfile:1.10.0",
			},
			expectedError: nil,
		},
		{
			name: "ExportCache and Custom Frontend Image via Env",
			buildOptions: &types.BuildOptions{
				ExportCache: []string{"cache1", "cache2"},
			},
			buildkitVersion: "0.4.0",
			frontendImage:   "",
			expectedFrontend: &Frontend{
				Frontend: defaultFrontend,
				Image:    "",
			},
			expectedError: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(buildkitFrontendImageEnvVar, tt.frontendImage)
			retriever, err := NewFrontendRetriever(context.Background(), &fakeBuildkitInfoGetter{
				info: &client.Info{
					BuildkitVersion: client.BuildkitVersion{
						Version: tt.buildkitVersion,
					},
				},
			}, io.NewIOController())
			assert.NoError(t, err, "expected no error")

			frontend := retriever.GetFrontend(tt.buildOptions)
			assert.Equal(t, tt.expectedFrontend, frontend, "expected frontend to match")

		})
	}
}
