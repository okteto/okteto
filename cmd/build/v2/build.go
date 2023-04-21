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

package v2

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"

	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/devenvironment"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	Run(ctx context.Context, buildOptions *types.BuildOptions) error
}

type oktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	IsOktetoRegistry(image string) bool
	GetImageReference(image string) (registry.OktetoImageReference, error)
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
}

// oktetoBuilderConfigInterface returns the configuration that the builder has for the registry and project
type oktetoBuilderConfigInterface interface {
	HasGlobalAccess() bool
	IsCleanProject() bool
	GetBuildHash(*model.BuildInfo) string
	GetGitCommit() string
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder   OktetoBuilderInterface
	Registry  oktetoRegistryInterface
	V1Builder *buildv1.OktetoBuilder

	Config oktetoBuilderConfigInterface
	// buildEnvironments are the environment variables created by the build steps
	buildEnvironments map[string]string

	// lock is a mutex to provide builEnvironments map safe concurrency
	lock sync.RWMutex

	// builtImages represents the images that have been built already
	builtImages map[string]bool
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry oktetoRegistryInterface) *OktetoBuilder {
	b := NewBuilderFromScratch()
	b.Builder = builder
	b.Registry = registry
	b.V1Builder = buildv1.NewBuilder(builder, registry)
	return b
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch() *OktetoBuilder {
	builder := &build.OktetoBuilder{}
	registry := registry.NewOktetoRegistry(okteto.Config{})
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		oktetoLog.Infof("could not get working dir: %w", err)
	}
	gitRepo := repository.NewRepository(wd)
	return &OktetoBuilder{
		Builder:           builder,
		Registry:          registry,
		V1Builder:         buildv1.NewBuilder(builder, registry),
		buildEnvironments: map[string]string{},
		builtImages:       map[string]bool{},
		Config:            getConfig(registry, gitRepo),
	}
}

// IsV1 returns false since it is a builder v2
func (*OktetoBuilder) IsV1() bool {
	return false
}

// Build builds the images defined by a manifest
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	if options.File != "" {
		workdir := model.GetWorkdirFromManifestPath(options.File)
		if err := os.Chdir(workdir); err != nil {
			return err
		}
		options.File = model.GetManifestPathFromWorkdir(options.File, workdir)
	}
	if options.Manifest.Name == "" {
		wd, err := os.Getwd()
		if err != nil {
			return err
		}
		c, _, err := okteto.NewK8sClientProvider().Provide(okteto.Context().Cfg)
		if err != nil {
			return err
		}
		inferer := devenvironment.NewNameInferer(c)
		options.Manifest.Name = inferer.InferName(ctx, wd, okteto.Context().Namespace, options.File)
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

	oktetoLog.Infof("Images to build: [%s]", strings.Join(toBuildSvcs, ", "))
	for len(bc.builtImages) != len(toBuildSvcs) {
		for _, svcToBuild := range toBuildSvcs {
			if bc.builtImages[svcToBuild] {
				oktetoLog.Infof("skipping image '%s' due to being already built")
				continue
			}
			if !bc.areImagesBuilt(buildManifest[svcToBuild].DependsOn) {
				oktetoLog.Infof("image '%s' can't be deployed because at least one of its dependent images(%s) are not built", svcToBuild, strings.Join(buildManifest[svcToBuild].DependsOn, ", "))
				continue
			}
			if options.EnableStages {
				oktetoLog.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
			}

			buildSvcInfo := buildManifest[svcToBuild]
			if imageTag, isBuilt := bc.checkIfCommitIsAlreadyBuilt(options.Manifest.Name, svcToBuild, bc.Config.GetBuildHash(buildSvcInfo), options.NoCache); isBuilt {
				oktetoLog.Warning("Skipping build of '%s' image because it's already built for commit %s", svcToBuild, bc.Config.GetBuildHash(buildSvcInfo))
				bc.SetServiceEnvVars(svcToBuild, imageTag)
				bc.builtImages[svcToBuild] = true
				continue
			}

			if !okteto.Context().IsOkteto && buildSvcInfo.Image == "" {
				return fmt.Errorf("'build.%s.image' is required if your cluster doesn't have Okteto installed", svcToBuild)
			}

			imageTag, err := bc.buildService(ctx, options.Manifest, svcToBuild, options)
			if err != nil {
				return fmt.Errorf("error building service '%s': %w", svcToBuild, err)
			}
			bc.SetServiceEnvVars(svcToBuild, imageTag)
			bc.builtImages[svcToBuild] = true
		}
	}
	if options.EnableStages {
		oktetoLog.SetStage("")
	}
	return options.Manifest.ExpandEnvVars()
}

func (bc *OktetoBuilder) areImagesBuilt(imagesToCheck []string) bool {
	for _, imageToCheck := range imagesToCheck {
		if _, ok := bc.builtImages[imageToCheck]; !ok {
			return false
		}
	}
	return true
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
	oktetoLog.Infof("Building image for service '%s'", svcName)
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := bc.getBuildInfoWithoutVolumeMounts(manifest.Build[svcName], isStackManifest)

	if err := buildSvcInfo.AddBuildArgs(bc.buildEnvironments); err != nil {
		return "", fmt.Errorf("error expanding build args from service '%s': %w", svcName, err)
	}

	tagToBuild := bc.getTagToBuild(manifest.Name, svcName, buildSvcInfo)
	buildSvcInfo.Image = tagToBuild

	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, buildSvcInfo, options, bc.Registry)

	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}
	return imageTagWithDigest, nil
}

func (bc *OktetoBuilder) addVolumeMounts(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	oktetoLog.Information("Including volume hosts for service '%s'", svcName)
	isStackManifest := (manifest.Type == model.StackType) || (manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil)
	fromImage := manifest.Build[svcName].Image
	if options.Tag != "" {
		fromImage = options.Tag
	}
	buildSvcInfo := getBuildInfoWithVolumeMounts(manifest.Build[svcName], isStackManifest)

	svcBuild, err := build.CreateDockerfileWithVolumeMounts(fromImage, buildSvcInfo.VolumesToInclude)
	if err != nil {
		return "", err
	}
	tagToBuild := bc.tagsToCheck(manifest.Name, svcName, buildSvcInfo)
	if len(tagToBuild) != 0 {
		buildSvcInfo.Image = tagToBuild[0]
	}

	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, svcBuild, options, bc.Registry)

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

func (bc *OktetoBuilder) getBuildInfoWithoutVolumeMounts(buildInfo *model.BuildInfo, isStackManifest bool) *model.BuildInfo {
	result := buildInfo.Copy()
	if len(result.VolumesToInclude) > 0 {
		result.VolumesToInclude = nil
	}
	if isStackManifest && okteto.IsOkteto() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		result.Image = ""
	}
	return result
}

func getBuildInfoWithVolumeMounts(buildInfo *model.BuildInfo, isStackManifest bool) *model.BuildInfo {
	result := buildInfo.Copy()
	if isStackManifest && okteto.IsOkteto() {
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
	toBuild := []string{}
	if len(options.CommandArgs) != 0 {
		return manifest.Build.GetSvcsToBuildFromList(options.CommandArgs)
	}

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
