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

package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/build"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/types"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder   OktetoBuilderInterface
	Registry  build.OktetoRegistryInterface
	V1Builder *buildv1.OktetoBuilder
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry build.OktetoRegistryInterface) *OktetoBuilder {
	return &OktetoBuilder{
		Builder:   builder,
		Registry:  registry,
		V1Builder: buildv1.NewBuilder(builder, registry),
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch() *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry()
	return &OktetoBuilder{
		Builder:   builder,
		Registry:  registry,
		V1Builder: buildv1.NewBuilder(builder, registry),
	}
}

// LoadContext Loads the okteto context based on a build v2
func (*OktetoBuilder) LoadContext(ctx context.Context, options *types.BuildOptions) error {
	ctxOpts := &contextCMD.ContextOptions{}

	if options.Manifest.Context != "" {
		ctxOpts.Context = options.Manifest.Context
	}
	if options.K8sContext != "" {
		ctxOpts.Context = options.K8sContext
	}

	if options.Manifest.Namespace != "" {
		ctxOpts.Namespace = options.Manifest.Namespace
		ctxOpts.Namespace = options.Manifest.Namespace
	}
	if options.Namespace != "" {
		ctxOpts.Namespace = options.Namespace
	}
	if err := contextCMD.NewContextCommand().Run(ctx, ctxOpts); err != nil {
		return err
	}

	if okteto.IsOkteto() && ctxOpts.Namespace != "" {
		create, err := utils.ShouldCreateNamespace(ctx, ctxOpts.Namespace)
		if err != nil {
			return err
		}
		if create {
			nsCmd, err := namespace.NewCommand()
			if err != nil {
				return err
			}
			if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: ctxOpts.Namespace}); err != nil {
				return err
			}
			return contextCMD.NewContextCommand().Run(ctx, ctxOpts)
		}
	}
	return nil
}

// Build builds the images defined by a manifest
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	if options.File != "" {
		workdir := utils.GetWorkdirFromManifestPath(options.File)
		if err := os.Chdir(workdir); err != nil {
			return err
		}
		options.File = utils.GetManifestPathFromWorkdir(options.File, workdir)
	}
	if options.Manifest.Name == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		options.Manifest.Name = utils.InferName(wd)
	}
	toBuildSvcs := getToBuildSvcs(options.Manifest, options)
	if err := validateOptions(options.Manifest, toBuildSvcs, options); err != nil {
		if errors.Is(err, oktetoErrors.ErrNoServicesToBuildDefined) {
			oktetoLog.Infof("skipping BuildV2 due to not having any svc to build")
			return nil
		}
		return err
	}

	buildManifest := options.Manifest.Build

	for _, svcToBuild := range toBuildSvcs {
		if options.EnableStages {
			oktetoLog.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
		}

		buildSvcInfo := buildManifest[svcToBuild]
		if !okteto.Context().IsOkteto && buildSvcInfo.Image == "" {
			return fmt.Errorf("'build.%s.image' is required if your context is not managed by Okteto", svcToBuild)
		}

		imageTag, err := bc.buildService(ctx, options.Manifest, svcToBuild, options)
		if err != nil {
			return err
		}
		oktetoLog.Success("Image for service '%s' pushed to registry: %s", svcToBuild, imageTag)
		bc.SetServiceEnvVars(svcToBuild, imageTag)
	}
	if options.EnableStages {
		oktetoLog.SetStage("")
	}
	return options.Manifest.ExpandEnvVars()
}

func (bc *OktetoBuilder) buildService(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	buildSvcInfo := manifest.Build[svcName]

	switch {
	case shouldAddVolumeMounts(buildSvcInfo) && !okteto.IsOkteto():
		return "", oktetoErrors.UserError{
			E:    fmt.Errorf("Build with volume mounts is not supported on vanilla clusters"),
			Hint: "Please connect to a okteto cluster and try again",
		}
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

func (bc *OktetoBuilder) buildSvcFromDockerfile(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	oktetoLog.Information("Building image for service '%s'", svcName)
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := getBuildInfoWithoutVolumeMounts(manifest.Build[svcName], isStackManifest)

	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, buildSvcInfo, options)

	// Check if the tag is already on global/dev registry and skip
	if build.ShouldOptimizeBuild(buildOptions) {
		tag, err := bc.optimizeBuild(buildOptions, svcName)
		if err != nil {
			return "", err
		}
		if tag != "" {
			return tag, nil
		}
	}
	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}
	return imageTagWithDigest, nil
}

func (bc *OktetoBuilder) optimizeBuild(buildOptions *types.BuildOptions, svcName string) (string, error) {
	oktetoLog.Debug("found optimizing the build flow")
	globalReference := strings.Replace(buildOptions.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)
	if _, err := bc.Registry.GetImageTagWithDigest(globalReference); err == nil {
		oktetoLog.Debugf("Skipping '%s' build. Image already exists at the Okteto Registry", svcName)
		return globalReference, nil
	}
	if registry.IsDevRegistry(buildOptions.Tag) {
		if _, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag); err == nil {
			oktetoLog.Debugf("skipping build: image %s is already built", buildOptions.Tag)
			return buildOptions.Tag, nil
		}
	}
	return "", nil
}

func (bc *OktetoBuilder) addVolumeMounts(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	oktetoLog.Information("Including volume hosts for service '%s'", svcName)
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := getBuildInfoWithVolumeMounts(manifest.Build[svcName], isStackManifest)

	fromImage := manifest.Build[svcName].Image
	if options.Tag != "" {
		fromImage = options.Tag
	}

	svcBuild, err := registry.CreateDockerfileWithVolumeMounts(fromImage, buildSvcInfo.VolumesToInclude)
	if err != nil {
		return "", err
	}
	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, svcBuild, options)

	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
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
	result.VolumesToInclude = getAccessibleVolumeMounts(buildInfo)
	return result
}

func getAccessibleVolumeMounts(buildInfo *model.BuildInfo) []model.StackVolume {
	accessibleVolumeMounts := make([]model.StackVolume, 0)
	for _, volume := range buildInfo.VolumesToInclude {
		if _, err := os.Stat(volume.LocalPath); !os.IsNotExist(err) {
			accessibleVolumeMounts = append(accessibleVolumeMounts, volume)
		}
	}
	return accessibleVolumeMounts
}

func getToBuildSvcs(manifest *model.Manifest, options *types.BuildOptions) []string {
	if len(options.CommandArgs) != 0 {
		return options.CommandArgs
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
