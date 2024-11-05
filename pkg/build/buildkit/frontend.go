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
	"os"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/types"
)

// FrontendType represents the type of frontend to use for a build.
type FrontendType string

const (
	// defaultFrontend specifies the frontend to use for local builds.
	defaultFrontend FrontendType = "dockerfile.v0"

	// gatewayFrontend specifies the frontend to use for builds executed in the gateway.
	gatewayFrontend FrontendType = "gateway.v0"

	// buildkitFrontendImageEnvVar is the environment variable used to override the default frontend image.
	buildkitFrontendImageEnvVar = "OKTETO_BUILDKIT_FRONTEND_IMAGE"

	// defaultDockerFrontendImage is the default Docker image to use as the frontend when executing builds in the gateway.
	defaultDockerFrontendImage = "docker/dockerfile:1.10.0"
)

type FrontendRetriever struct {
	logger *io.Controller
}

type Frontend struct {
	Frontend FrontendType
	Image    string
}

func NewFrontendRetriever(logger *io.Controller) *FrontendRetriever {
	return &FrontendRetriever{
		logger: logger,
	}
}

func (f *FrontendRetriever) GetFrontend(buildOptions *types.BuildOptions) *Frontend {
	customFrontendImage := os.Getenv(buildkitFrontendImageEnvVar)
	if len(buildOptions.ExtraHosts) > 0 {
		f.logger.Infof("using gateway frontend because of extra hosts")
		return f.getGatewayFrontend(customFrontendImage)
	}

	if customFrontendImage != "" {
		f.logger.Infof("using gateway frontend because of custom frontend image")
		return f.getGatewayFrontend(customFrontendImage)
	}
	f.logger.Infof("using default frontend")
	return f.getLocalFrontend()
}

func (f *FrontendRetriever) getLocalFrontend() *Frontend {
	return &Frontend{
		Frontend: defaultFrontend,
		Image:    "",
	}
}

func (f *FrontendRetriever) getGatewayFrontend(customFrontendImage string) *Frontend {
	image := defaultDockerFrontendImage
	if customFrontendImage != "" {
		f.logger.Infof("using custom frontend image %s", customFrontendImage)
		image = customFrontendImage
	}
	return &Frontend{
		Frontend: gatewayFrontend,
		Image:    image,
	}
}
