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
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
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
	CloneGlobalImageToDev(imageWithDigest, tag string) (string, error)
}

// oktetoBuilderConfigInterface returns the configuration that the builder has for the registry and project
type oktetoBuilderConfigInterface interface {
	HasGlobalAccess() bool
	IsCleanProject() bool
	GetBuildHash(*model.BuildInfo) string
	GetGitCommit() string
	IsOkteto() bool
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
		Config:            getConfig(registry, gitRepo),
	}
}

// IsV1 returns false since it is a builder v2
func (*OktetoBuilder) IsV1() bool {
	return false
}

// Build builds the images defined by a manifest
func (bc *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	if utils.LoadBoolean(constants.OktetoDeployRemote) {
		// Since the local build has already been built,
		// we have the environment variables set and we can skip this code
		return nil
	}
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

	// builtImagesControl represents the controller for the built services
	// when a service is built we track it here
	builtImagesControl := make(map[string]bool)

	oktetoLog.Infof("Images to build: [%s]", strings.Join(toBuildSvcs, ", "))
	for len(builtImagesControl) != len(toBuildSvcs) {
		for _, svcToBuild := range toBuildSvcs {
			if isServiceBuilt(svcToBuild, builtImagesControl) {
				oktetoLog.Infof("skipping image '%s' due to being already built")
				continue
			}
			if !areAllServicesBuilt(buildManifest[svcToBuild].DependsOn, builtImagesControl) {
				oktetoLog.Infof("image '%s' can't be deployed because at least one of its dependent images(%s) are not built", svcToBuild, strings.Join(buildManifest[svcToBuild].DependsOn, ", "))
				continue
			}
			if options.EnableStages {
				oktetoLog.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
			}

			buildSvcInfo := buildManifest[svcToBuild]

			// We only check that the image is built in the global registry if the noCache option is not set
			if !options.NoCache && bc.Config.IsCleanProject() {
				imageChecker := getImageChecker(buildSvcInfo, bc.Config, bc.Registry)
				if imageWithDigest, isBuilt := imageChecker.checkIfCommitHashIsBuilt(options.Manifest.Name, svcToBuild, buildSvcInfo); isBuilt {
					oktetoLog.Information("Skipping build of '%s' image because it's already built for commit %s", svcToBuild, bc.Config.GetGitCommit())
					// if the built image belongs to global registry we clone it to the dev registry
					// so that in can be used in dev containers (i.e. okteto up)
					if bc.Registry.IsGlobalRegistry(imageWithDigest) {
						oktetoLog.Debugf("Copying image '%s' from global to personal registry", svcToBuild)
						tag := bc.Config.GetBuildHash(buildSvcInfo)
						devImage, err := bc.Registry.CloneGlobalImageToDev(imageWithDigest, tag)
						if err != nil {
							return err
						}
						imageWithDigest = devImage
					}

					bc.SetServiceEnvVars(svcToBuild, imageWithDigest)
					builtImagesControl[svcToBuild] = true
					continue
				}
			}

			if !okteto.Context().IsOkteto && buildSvcInfo.Image == "" {
				return fmt.Errorf("'build.%s.image' is required if your cluster doesn't have Okteto installed", svcToBuild)
			}

			imageTag, err := bc.buildService(ctx, options.Manifest, svcToBuild, options)
			if err != nil {
				return fmt.Errorf("error building service '%s': %w", svcToBuild, err)
			}
			bc.SetServiceEnvVars(svcToBuild, imageTag)
			builtImagesControl[svcToBuild] = true
		}
	}
	if options.EnableStages {
		oktetoLog.SetStage("")
	}
	return options.Manifest.ExpandEnvVars()
}

// areServicesBuilt compares the list of services with the built control
// when all services are built returns true, when a service is still pending it will return false
func areAllServicesBuilt(services []string, control map[string]bool) bool {
	for _, service := range services {
		if _, ok := control[service]; !ok {
			return false
		}
	}
	return true
}

// isServiceBuilt returns if a service has been built
func isServiceBuilt(service string, control map[string]bool) bool {
	return control[service]
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

	tagToBuild := newImageTagger(bc.Config).tag(manifest.Name, svcName, buildSvcInfo)
	buildSvcInfo.Image = tagToBuild
	if err := buildSvcInfo.AddBuildArgs(bc.buildEnvironments); err != nil {
		return "", fmt.Errorf("error expanding build args from service '%s': %w", svcName, err)
	}

	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, buildSvcInfo, options, bc.Registry)

	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	// check if the image is pushed to the dev registry if DevTag is set
	reference := buildOptions.Tag
	if buildOptions.DevTag != "" {
		reference = buildOptions.DevTag
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(reference)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %v", reference, err)
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

	tagToBuild := newImageWithVolumesTagger(bc.Config).tag(manifest.Name, svcName, manifest.Build[svcName])
	buildSvcInfo := getBuildInfoWithVolumeMounts(manifest.Build[svcName], isStackManifest)
	svcBuild, err := build.CreateDockerfileWithVolumeMounts(fromImage, buildSvcInfo.VolumesToInclude)
	if err != nil {
		return "", err
	}
	buildOptions := build.OptsFromBuildInfo(manifest.Name, svcName, svcBuild, options, bc.Registry)
	buildOptions.Tag = tagToBuild

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

func getImageChecker(buildInfo *model.BuildInfo, cfg oktetoBuilderConfigInterface, registry registryImageCheckerInterface) imageCheckerInterface {
	var tagger imageTaggerInterface
	if shouldAddVolumeMounts(buildInfo) {
		tagger = newImageWithVolumesTagger(cfg)
	} else {
		tagger = newImageTagger(cfg)
	}
	return newImageChecker(cfg, registry, tagger)
}
