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
package checker

import (
	"context"

	"github.com/okteto/okteto/cmd/build/v2/environment"
	"github.com/okteto/okteto/cmd/build/v2/metadata"
	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/registry"
)

const (
	parallelCheckStrategyEnvVar = "OKTETO_BUILD_CHECK_STRATEGY_PARALLEL"
)

// ImageTagger defines the interface for managing image tags and references.
type ImageTagger interface {
	GetGlobalTagFromDevIfNeccesary(tags, namespace, registryURL, buildHash string, ic registry.ImageCtrl) string
	GetImageReferencesForTag(manifestName, svcToBuildName, tag string) []string
	GetImageReferencesForDeploy(manifestName, svcToBuildName string) []string
}

// SmartBuildController defines the interface for smart build operations.
type SmartBuildController interface {
	GetBuildHash(buildInfo *build.Info, service string) string
	CloneGlobalImageToDev(globalImage, svcImage string) (string, error)
}

// CheckStrategy defines the interface for different cache checking strategies.
type CheckStrategy interface {
	CheckServicesCache(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, svcsToBuild []string) (cachedServices []string, notCachedServices []string, err error)
	CloneGlobalImagesToDev(manifestName string, buildManifest build.ManifestBuild, images []string) error
	GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error)
}

// ImageCacheChecker is responsible for checking image cache status and managing image operations.
type ImageCacheChecker struct {
	checkStrategy CheckStrategy
}

// NewImageCacheChecker creates a new ImageCacheChecker instance with the appropriate check strategy.
// It determines whether to use parallel or sequential checking based on the OKTETO_BUILD_CHECK_STRATEGY_PARALLEL environment variable.
func NewImageCacheChecker(namespace string, registryURL string, tagger ImageTagger, smartBuildCtrl SmartBuildController, imageCtrl registry.ImageCtrl, metadataCollector *metadata.MetadataCollector, registry DigestResolver, logger *io.Controller, buildEnvVarsSetter *environment.ServiceEnvVarsSetter) *ImageCacheChecker {
	cacheChecker := NewImageChecker(tagger, namespace, registryURL, imageCtrl, registry, logger)
	var checkStrategy CheckStrategy
	if env.LoadBoolean(parallelCheckStrategyEnvVar) {
		// TODO: Implement parallel check strategy
	} else {
		checkStrategy = newSequentialCheckStrategy(smartBuildCtrl, tagger, cacheChecker, metadataCollector, logger, buildEnvVarsSetter)
	}

	return &ImageCacheChecker{
		checkStrategy: checkStrategy,
	}
}

// CheckImages checks which images are cached and which need to be built.
// It delegates to the configured check strategy to determine cache status for the provided images.
// Returns two slices: cached images and non-cached images that need to be built.
func (i *ImageCacheChecker) CheckImages(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, images []string) ([]string, []string, error) {
	cached, notCached, err := i.checkStrategy.CheckServicesCache(ctx, manifestName, buildManifest, images)
	return cached, notCached, err
}

// GetImageDigestReferenceForServiceDeploy retrieves the digest reference for a service
// that will be used during deployment. This ensures the correct image version is deployed.
func (i *ImageCacheChecker) GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error) {
	return i.checkStrategy.GetImageDigestReferenceForServiceDeploy(manifestName, service, buildInfo)
}

// CloneGlobalImagesToDev clones multiple global images to the development environment.
// This is useful for ensuring development environments have access to the latest global images.
func (i *ImageCacheChecker) CloneGlobalImagesToDev(manifestName string, buildManifest build.ManifestBuild, images []string) error {
	if err := i.checkStrategy.CloneGlobalImagesToDev(manifestName, buildManifest, images); err != nil {
		return err
	}
	return nil
}
