// Copyright 2022 The Okteto Authors
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

package build

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
)

// BuildV2 builds the images defined by a manifest
func (bc *Command) BuildV2(ctx context.Context, manifest *model.Manifest, options *types.BuildOptions) error {
	toBuildSvcs := getToBuildSvcs(manifest, options)
	if err := validateOptions(manifest, toBuildSvcs, options); err != nil {
		if errors.Is(err, oktetoErrors.ErrNoServicesToBuildDefined) {
			oktetoLog.Infof("skipping BuildV2 due to not having any svc to build")
			return nil
		}
		return err
	}

	buildManifest := manifest.Build

	for _, svcToBuild := range toBuildSvcs {
		if options.EnableStages {
			oktetoLog.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
		}

		buildSvcInfo := buildManifest[svcToBuild]
		if !okteto.Context().IsOkteto && buildSvcInfo.Image == "" {
			return fmt.Errorf("'build.%s.image' is required if your context is not managed by Okteto", svcToBuild)
		}

		imageTag, err := bc.buildService(ctx, manifest, svcToBuild, options)
		if err != nil {
			return err
		}
		oktetoLog.Success("Image for service '%s' pushed to registry: %s", svcToBuild, imageTag)
		if err := bc.SetServicetEnvVars(svcToBuild, imageTag); err != nil {
			return err
		}
	}
	if options.EnableStages {
		oktetoLog.SetStage("")
	}
	if err := manifest.ExpandEnvVars(); err != nil {
		return err
	}

	return nil
}
func (bc *Command) buildService(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	buildSvcInfo := manifest.Build[svcName]

	switch {
	case shouldBuildFromDockerfile(buildSvcInfo) && shouldAddVolumeMounts(buildSvcInfo):
		image, err := bc.buildSvcFromDockerfile(ctx, manifest, svcName, options)
		if err != nil {
			return "", err
		}
		buildSvcInfo.Image = image
		return bc.addVolumeMounts(ctx, manifest, svcName, options)
	case shouldBuildFromDockerfile(buildSvcInfo):
		return bc.buildSvcFromDockerfile(ctx, manifest, svcName, options)
	case shouldAddVolumeMounts(buildSvcInfo):
		if okteto.IsOkteto() {
			return bc.addVolumeMounts(ctx, manifest, svcName, options)
		}

	default:
		oktetoLog.Infof("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName)
	}
	return "", nil
}

func (bc *Command) buildSvcFromDockerfile(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	oktetoLog.Information("Building image for service '%s'", svcName)
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := getBuildInfoWithoutVolumeMounts(manifest.Build[svcName], isStackManifest)

	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, buildSvcInfo, &types.BuildOptions{})

	// Check if the tag is already on global/dev registry and skip
	if build.ShouldOptimizeBuild(buildOptions.Tag) && !buildOptions.BuildToGlobal {
		oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
		globalReference := strings.Replace(buildOptions.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)
		if _, err := registry.NewOktetoRegistry().GetImageTagWithDigest(globalReference); err == nil {
			oktetoLog.Debugf("Skipping '%s' build. Image already exists at the Okteto Registry", svcName)
			return globalReference, nil
		}
		if registry.IsDevRegistry(buildOptions.Tag) {
			if _, err := registry.NewOktetoRegistry().GetImageTagWithDigest(buildOptions.Tag); err == nil {
				oktetoLog.Debugf("skipping build: image %s is already built", buildOptions.Tag)
				return buildOptions.Tag, nil
			}
		}
	}
	if err := bc.BuildV1(ctx, buildOptions); err != nil {
		return "", err
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}
	return imageTagWithDigest, nil
}

func (bc *Command) addVolumeMounts(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	oktetoLog.Information("Including volume hosts for service '%s'", svcName)
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := getBuildInfoWithVolumeMounts(manifest.Build[svcName], isStackManifest)

	fromImage := buildSvcInfo.Image
	if options.Tag != "" {
		fromImage = options.Tag
	}

	svcBuild, err := registry.CreateDockerfileWithVolumeMounts(fromImage, buildSvcInfo.VolumesToInclude)
	if err != nil {
		return "", err
	}
	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, svcBuild, &types.BuildOptions{})
	if err := bc.BuildV1(ctx, buildOptions); err != nil {
		return "", err
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}
	return imageTagWithDigest, nil
}

func shouldBuildFromDockerfile(buildInfo *model.BuildInfo) bool {
	return buildInfo.Dockerfile != ""
}

func shouldAddVolumeMounts(buildInfo *model.BuildInfo) bool {
	return len(buildInfo.VolumesToInclude) > 0
}

func getBuildInfoWithoutVolumeMounts(buildInfo *model.BuildInfo, isStackManifest bool) *model.BuildInfo {
	result := buildInfo.Copy()
	if len(result.VolumesToInclude) > 0 {
		result.VolumesToInclude = nil
	}
	if isStackManifest && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		result.Image = ""
	}
	return result
}

func getBuildInfoWithVolumeMounts(buildInfo *model.BuildInfo, isStackManifest bool) *model.BuildInfo {
	result := buildInfo.Copy()
	if isStackManifest && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		result.Image = ""
	}
	accessibleVolumeMounts := make([]model.StackVolume, 0)
	for _, volume := range buildInfo.VolumesToInclude {
		if _, err := os.Stat(volume.LocalPath); !os.IsNotExist(err) {
			accessibleVolumeMounts = append(accessibleVolumeMounts, volume)
		}
	}

	return result
}

func getToBuildSvcs(manifest *model.Manifest, options *types.BuildOptions) []string {
	if len(options.Args) != 0 {
		return options.Args
	}
	toBuild := []string{}
	for svcName := range manifest.Build {
		toBuild = append(toBuild, svcName)
	}
	return toBuild
}

func validateOptions(manifest *model.Manifest, svcsToBuild []string, options *types.BuildOptions) error {
	if len(svcsToBuild) == 0 {
		return oktetoErrors.ErrNoServicesToBuildDefined
	}

	if err := validateServices(manifest.Build, svcsToBuild); err != nil {
		return err
	}

	if len(svcsToBuild) != 1 && (options.Tag != "" || options.Target != "" || options.CacheFrom != nil || options.Secrets != nil) {
		return oktetoErrors.ErrNoFlagAllowedOnSingleImageBuild
	}

	return nil
}

func validateServices(buildSection model.ManifestBuild, svcsToBuild []string) error {
	invalid := []string{}
	for _, service := range svcsToBuild {
		if _, ok := buildSection[service]; !ok {
			invalid = append(invalid, service)
		}
	}
	if len(invalid) != 0 {
		return fmt.Errorf("invalid services names, not found at manifest: %v", invalid)
	}
	return nil
}
