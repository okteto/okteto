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
	"errors"

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

	// dockerFrontendImage specifies the Docker image to use as the frontend when executing builds in the gateway.
	dockerFrontendImage = "docker/dockerfile"

	// buildkitFrontendImageEnvVar is the environment variable used to override the default frontend image.
	buildkitFrontendImageEnvVar = "OKTETO_BUILDKIT_FRONTEND_IMAGE"
)

var (
	// errOptionsIsNil is the error returned when the build options are nil.
	errOptionsIsNil = errors.New("buildOptions cannot be nil")
)

type FrontendRetriever struct {
	getEnv func(string) string
	logger *io.Controller
}

type Frontend struct {
	Frontend FrontendType
	Image    string
}

func NewFrontendRetriever(getEnv func(string) string, logger *io.Controller) *FrontendRetriever {
	return &FrontendRetriever{
		getEnv: getEnv,
		logger: logger,
	}
}

func (f *FrontendRetriever) GetFrontend(buildOptions *types.BuildOptions) (*Frontend, error) {
	if buildOptions == nil {
		return nil, errOptionsIsNil
	}

	customFrontendImage := f.getEnv(buildkitFrontendImageEnvVar)
	if len(buildOptions.ExtraHosts) > 0 || customFrontendImage != "" {
		f.logger.Infof("using gateway frontend")
		return f.getGatewayFrontend(customFrontendImage), nil
	}
	f.logger.Infof("using local frontend")
	return f.getLocalFrontend(), nil
}

func (f *FrontendRetriever) getLocalFrontend() *Frontend {
	return &Frontend{
		Frontend: defaultFrontend,
		Image:    "",
	}
}

func (f *FrontendRetriever) getGatewayFrontend(customFrontendImage string) *Frontend {
	image := dockerFrontendImage
	if customFrontendImage != "" {
		f.logger.Infof("using custom frontend image %s", customFrontendImage)
		image = customFrontendImage
	}
	return &Frontend{
		Frontend: gatewayFrontend,
		Image:    image,
	}
}
