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
	"os"

	"github.com/Masterminds/semver/v3"
	"github.com/moby/buildkit/client"
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
	logger          *io.Controller
	buildkitVersion *semver.Version
}

type Frontend struct {
	Frontend FrontendType
	Image    string
}
type buildkitInfoGetter interface {
	Info(ctx context.Context) (*client.Info, error)
}

func NewFrontendRetriever(ctx context.Context, buildkitClient buildkitInfoGetter, logger *io.Controller) (*FrontendRetriever, error) {
	info, err := buildkitClient.Info(ctx)
	if err != nil {
		logger.Infof("failed to get buildkit info: %s", err)
		return nil, err
	}
	buildkitVersion, err := semver.NewVersion(info.BuildkitVersion.Version)
	if err != nil {
		logger.Infof("failed to parse buildkit version: %s", err)
	}
	return &FrontendRetriever{
		logger:          logger,
		buildkitVersion: buildkitVersion,
	}, nil
}

func (f *FrontendRetriever) GetFrontend(buildOptions *types.BuildOptions) *Frontend {
	customFrontendImage := os.Getenv(buildkitFrontendImageEnvVar)
	if len(buildOptions.ExtraHosts) > 0 {
		extraHostFirstVersion := semver.MustParse("0.16.0")
		if f.buildkitVersion != nil && (f.buildkitVersion.GreaterThan(extraHostFirstVersion) || f.buildkitVersion.Equal(extraHostFirstVersion)) {
			f.logger.Infof("Using default frontend (BuildKit version supports extra hosts)")
		} else {
			f.logger.Infof("Using gateway frontend because BuildKit version doesn't support extra hosts")
			return f.getGatewayFrontend(customFrontendImage)
		}
	}

	if len(buildOptions.ExportCache) > 1 {
		exportSeveralCacheFirstVersion := semver.MustParse("0.11.0")
		if f.buildkitVersion != nil && (f.buildkitVersion.GreaterThan(exportSeveralCacheFirstVersion) || f.buildkitVersion.Equal(exportSeveralCacheFirstVersion)) {
			f.logger.Infof("Using gateway frontend because BuildKit version supports multiple cache exports")
			return f.getGatewayFrontend(customFrontendImage)
		} else {
			f.logger.Infof("Using default frontend because BuildKit version doesn't support multiple cache exports in gateway frontend")
		}
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
