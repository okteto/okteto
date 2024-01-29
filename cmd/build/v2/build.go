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
	"strconv"
	"strings"
	"sync"
	"time"

	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
)

// OktetoBuilderInterface runs the build of an image
type OktetoBuilderInterface interface {
	GetBuilder() string
	Run(ctx context.Context, buildOptions *types.BuildOptions, ioCtrl *io.Controller) error
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
	GetGitCommit() string
	IsOkteto() bool
	GetAnonymizedRepo() string
}

type analyticsTrackerInterface interface {
	TrackImageBuild(meta ...*analytics.ImageBuildMetadata)
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	Builder          OktetoBuilderInterface
	Registry         oktetoRegistryInterface
	Config           oktetoBuilderConfigInterface
	analyticsTracker analyticsTrackerInterface
	V1Builder        *buildv1.OktetoBuilder
	oktetoContext    buildCmd.OktetoContextInterface

	smartBuildCtrl *smartbuild.Ctrl

	// buildEnvironments are the environment variables created by the build steps
	buildEnvironments map[string]string

	ioCtrl    *io.Controller
	k8sLogger *io.K8sLogger

	// lock is a mutex to provide buildEnvironments map safe concurrency
	lock sync.RWMutex
}

// NewBuilder creates a new okteto builder
func NewBuilder(builder OktetoBuilderInterface, registry oktetoRegistryInterface, ioCtrl *io.Controller, analyticsTracker analyticsTrackerInterface, okCtx okteto.ContextInterface, k8sLogger *io.K8sLogger) *OktetoBuilder {
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		ioCtrl.Logger().Infof("could not get working dir: %s", err)
	}
	gitRepo := repository.NewRepository(wd)
	config := getConfigStateless(registry, gitRepo, ioCtrl.Logger(), okCtx.IsOkteto())

	buildEnvs := map[string]string{}
	buildEnvs[OktetoEnableSmartBuildEnvVar] = strconv.FormatBool(config.isSmartBuildsEnable)
	return &OktetoBuilder{
		Builder:           builder,
		Registry:          registry,
		V1Builder:         buildv1.NewBuilder(builder, registry, ioCtrl),
		buildEnvironments: buildEnvs,
		Config:            config,
		analyticsTracker:  analyticsTracker,
		ioCtrl:            ioCtrl,
		smartBuildCtrl:    smartbuild.NewSmartBuildCtrl(gitRepo, registry, config.fs, ioCtrl),
		oktetoContext:     okCtx,
		k8sLogger:         k8sLogger,
	}
}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(analyticsTracker analyticsTrackerInterface, ioCtrl *io.Controller) *OktetoBuilder {
	builder := &buildCmd.OktetoBuilder{
		OktetoContext: &okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		Fs: afero.NewOsFs(),
	}
	reg := registry.NewOktetoRegistry(okteto.Config{})
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		ioCtrl.Logger().Infof("could not get working dir: %s", err)
	}
	topLevelGitDir, err := repository.FindTopLevelGitDir(wd, afero.NewOsFs())
	if err != nil {
		ioCtrl.Logger().Infof("could not get top level git dir: %s", err)
	}
	if topLevelGitDir != "" {
		wd = topLevelGitDir
	}
	gitRepo := repository.NewRepository(wd)
	config := getConfig(reg, gitRepo, ioCtrl.Logger())

	buildEnvs := map[string]string{}
	buildEnvs[OktetoEnableSmartBuildEnvVar] = strconv.FormatBool(config.isSmartBuildsEnable)

	return &OktetoBuilder{
		Builder:           builder,
		Registry:          reg,
		V1Builder:         buildv1.NewBuilder(builder, reg, ioCtrl),
		buildEnvironments: buildEnvs,
		Config:            config,
		analyticsTracker:  analyticsTracker,
		ioCtrl:            ioCtrl,
		smartBuildCtrl:    smartbuild.NewSmartBuildCtrl(gitRepo, reg, config.fs, ioCtrl),
		oktetoContext: &okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
	}
}

// IsV1 returns false since it is a builder v2
func (*OktetoBuilder) IsV1() bool {
	return false
}

// Build builds the images defined by a manifest
// TODO: Function with cyclomatic complexity higher than threshold. Refactor function in order to reduce its complexity
// skipcq: GO-R1005
func (ob *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	if env.LoadBoolean(constants.OktetoDeployRemote) {
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
		c, _, err := okteto.NewK8sClientProviderWithLogger(ob.k8sLogger).Provide(ob.oktetoContext.GetCurrentCfg())
		if err != nil {
			return err
		}
		inferer := devenvironment.NewNameInferer(c)
		options.Manifest.Name = inferer.InferName(ctx, wd, ob.oktetoContext.GetCurrentNamespace(), options.File)
	}
	toBuildSvcs := getToBuildSvcs(options.Manifest, options)
	if err := validateOptions(options.Manifest, toBuildSvcs, options); err != nil {
		if errors.Is(err, oktetoErrors.ErrNoServicesToBuildDefined) {
			ob.ioCtrl.Logger().Info("skipping BuildV2 due to not having any svc to build")
			return nil
		}
		return err
	}

	buildManifest := options.Manifest.Build

	// builtImagesControl represents the controller for the built services
	// when a service is built we track it here
	builtImagesControl := make(map[string]bool)

	// send analytics for all builds after Build
	buildsAnalytics := make([]*analytics.ImageBuildMetadata, 0)

	// send all events appended on each build
	defer func([]*analytics.ImageBuildMetadata) {
		ob.analyticsTracker.TrackImageBuild(buildsAnalytics...)
	}(buildsAnalytics)

	ob.ioCtrl.Logger().Infof("Images to build: [%s]", strings.Join(toBuildSvcs, ", "))
	for len(builtImagesControl) != len(toBuildSvcs) {
		for _, svcToBuild := range toBuildSvcs {
			if skipServiceBuild(svcToBuild, builtImagesControl) {
				ob.ioCtrl.Logger().Infof("skipping image '%s' due to being already built", svcToBuild)
				continue
			}
			if !areAllServicesBuilt(buildManifest[svcToBuild].DependsOn, builtImagesControl) {
				ob.ioCtrl.Logger().Infof("image '%s' can't be deployed because at least one of its dependent images(%s) are not built", svcToBuild, strings.Join(buildManifest[svcToBuild].DependsOn, ", "))
				continue
			}
			if options.EnableStages {
				ob.ioCtrl.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
			}

			buildSvcInfo := buildManifest[svcToBuild]

			// create the meta pointer and append it to the analytics slice
			meta := analytics.NewImageBuildMetadata()
			buildsAnalytics = append(buildsAnalytics, meta)

			meta.Name = svcToBuild
			meta.RepoURL = ob.Config.GetAnonymizedRepo()

			repoHashDurationStart := time.Now()

			repoHash, err := ob.smartBuildCtrl.GetProjectHash(buildSvcInfo)
			if err != nil {
				ob.ioCtrl.Logger().Infof("error getting project commit hash: %s", err)
			}
			meta.RepoHash = repoHash
			meta.RepoHashDuration = time.Since(repoHashDurationStart)

			buildContextHashDurationStart := time.Now()

			serviceHash, err := ob.smartBuildCtrl.GetServiceHash(buildSvcInfo)
			if err != nil {
				ob.ioCtrl.Logger().Infof("error getting service commit hash: %s", err)
			}
			meta.BuildContextHash = serviceHash
			meta.BuildContextHashDuration = time.Since(buildContextHashDurationStart)

			// We only check that the image is built in the global registry if the noCache option is not set
			if !options.NoCache && ob.smartBuildCtrl.IsEnabled() {
				imageChecker := getImageChecker(buildSvcInfo, ob.Config, ob.Registry, ob.smartBuildCtrl, ob.ioCtrl.Logger())
				cacheHitDurationStart := time.Now()

				buildHash, err := ob.smartBuildCtrl.GetBuildHash(buildSvcInfo)
				if err != nil {
					ob.ioCtrl.Logger().Infof("error getting build hash: %s", err)
				}
				imageWithDigest, isBuilt := imageChecker.checkIfBuildHashIsBuilt(options.Manifest.Name, svcToBuild, buildHash)

				meta.CacheHit = isBuilt
				meta.CacheHitDuration = time.Since(cacheHitDurationStart)

				if isBuilt {
					ob.ioCtrl.Out().Infof("Skipping build of '%s' image because it's already built for commit %s", svcToBuild, ob.smartBuildCtrl.GetBuildCommit(buildSvcInfo))

					imageWithDigest, err = ob.smartBuildCtrl.CloneGlobalImageToDev(imageWithDigest, buildHash)
					if err != nil {
						return err
					}
					ob.SetServiceEnvVars(svcToBuild, imageWithDigest)
					builtImagesControl[svcToBuild] = true
					meta.Success = true
					continue
				}
			}

			if !ob.oktetoContext.IsOkteto() && buildSvcInfo.Image == "" {
				return fmt.Errorf("'build.%s.image' is required if your context doesn't have Okteto installed", svcToBuild)
			}
			buildDurationStart := time.Now()
			imageTag, err := ob.buildServiceImages(ctx, options.Manifest, svcToBuild, options)
			if err != nil {
				return fmt.Errorf("error building service '%s': %w", svcToBuild, err)
			}
			meta.BuildDuration = time.Since(buildDurationStart)
			meta.Success = true

			ob.SetServiceEnvVars(svcToBuild, imageTag)
			builtImagesControl[svcToBuild] = true
		}
	}
	if options.EnableStages {
		ob.ioCtrl.SetStage("")
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

// skipServiceBuild returns if a service has been built
func skipServiceBuild(service string, control map[string]bool) bool {
	return control[service]
}

// buildServiceImages builds the images for the given service.
// if service has volumes to include but is not okteto, an error is returned
// returned image reference includes the digest
// when a service includes volumes, this is the image returned
func (bc *OktetoBuilder) buildServiceImages(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	buildSvcInfo := manifest.Build[svcName]

	switch {
	case serviceHasVolumesToInclude(buildSvcInfo) && !bc.oktetoContext.IsOkteto():
		return "", oktetoErrors.UserError{
			E:    fmt.Errorf("Build with volume mounts is not supported on vanilla contexts"),
			Hint: "Please connect to a okteto context and try again",
		}
	case serviceHasDockerfile(buildSvcInfo) && serviceHasVolumesToInclude(buildSvcInfo):
		image, err := bc.buildSvcFromDockerfile(ctx, manifest, svcName, options)
		if err != nil {
			return "", err
		}
		buildSvcInfo.Image = image
		return bc.addVolumeMounts(ctx, manifest, svcName, options)
	case serviceHasDockerfile(buildSvcInfo):
		return bc.buildSvcFromDockerfile(ctx, manifest, svcName, options)
	case serviceHasVolumesToInclude(buildSvcInfo):
		if bc.oktetoContext.IsOkteto() {
			return bc.addVolumeMounts(ctx, manifest, svcName, options)
		}

	default:
		bc.ioCtrl.Logger().Info(fmt.Sprintf("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName))
	}
	return "", nil
}

func (bc *OktetoBuilder) buildSvcFromDockerfile(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	bc.ioCtrl.Logger().Info(fmt.Sprintf("Building service '%s' from Dockerfile", svcName))
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := bc.getBuildInfoWithoutVolumeMounts(manifest.Build[svcName], isStackManifest)
	var err error
	var buildHash string
	if bc.smartBuildCtrl.IsEnabled() {
		buildHash, err = bc.smartBuildCtrl.GetBuildHash(buildSvcInfo)
		if err != nil {
			bc.ioCtrl.Logger().Infof("error getting build hash: %s", err)
		}
	}
	tagToBuild := newImageTagger(bc.Config, bc.smartBuildCtrl).getServiceImageReference(manifest.Name, svcName, buildSvcInfo, buildHash)
	buildSvcInfo.Image = tagToBuild
	if err := buildSvcInfo.AddArgs(bc.buildEnvironments); err != nil {
		return "", fmt.Errorf("error expanding build args from service '%s': %w", svcName, err)
	}

	buildOptions := buildCmd.OptsFromBuildInfo(manifest.Name, svcName, buildSvcInfo, options, bc.Registry, bc.oktetoContext)

	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	var imageTagWithDigest string
	tags := strings.Split(buildOptions.Tag, ",")
	for _, tag := range tags {
		// check if the image is pushed to the dev registry if DevTag is set
		reference := tag
		if buildOptions.DevTag != "" {
			reference = buildOptions.DevTag
		}
		imageTagWithDigest, err = bc.Registry.GetImageTagWithDigest(reference)
		if err != nil {
			return "", fmt.Errorf("error accessing image at registry %s: %w", reference, err)
		}
	}
	return imageTagWithDigest, nil
}

func (bc *OktetoBuilder) addVolumeMounts(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	bc.ioCtrl.Out().Infof("Including volume hosts for service '%s'", svcName)
	isStackManifest := (manifest.Type == model.StackType) || (manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil)
	fromImage := manifest.Build[svcName].Image
	if options.Tag != "" {
		fromImage = options.Tag
	}

	buildInfoCopy := manifest.Build[svcName].Copy()
	buildInfoCopy.Image = ""

	var err error
	var buildHash string
	if bc.smartBuildCtrl.IsEnabled() {
		buildHash, err = bc.smartBuildCtrl.GetBuildHash(buildInfoCopy)
		if err != nil {
			bc.ioCtrl.Logger().Infof("error getting build hash: %s", err)
		}
	}

	tagToBuild := newImageWithVolumesTagger(bc.Config, bc.smartBuildCtrl).getServiceImageReference(manifest.Name, svcName, buildInfoCopy, buildHash)
	buildSvcInfo := getBuildInfoWithVolumeMounts(manifest.Build[svcName], isStackManifest, bc.oktetoContext.IsOkteto())
	svcBuild, err := buildCmd.CreateDockerfileWithVolumeMounts(fromImage, buildSvcInfo.VolumesToInclude)
	if err != nil {
		return "", err
	}
	buildOptions := buildCmd.OptsFromBuildInfo(manifest.Name, svcName, svcBuild, options, bc.Registry, bc.oktetoContext)
	buildOptions.Tag = tagToBuild

	if err := bc.V1Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	imageTagWithDigest, err := bc.Registry.GetImageTagWithDigest(buildOptions.Tag)
	if err != nil {
		return "", fmt.Errorf("error accessing image at registry %s: %w", options.Tag, err)
	}
	return imageTagWithDigest, nil
}

// serviceHasDockerfile returns true when service BuildInfo Dockerfile is not empty
func serviceHasDockerfile(buildInfo *build.Info) bool {
	return buildInfo.Dockerfile != ""
}

// serviceHasVolumesToInclude returns true when service BuildInfo VolumesToInclude are more than 0
func serviceHasVolumesToInclude(buildInfo *build.Info) bool {
	return len(buildInfo.VolumesToInclude) > 0
}

func (bc *OktetoBuilder) getBuildInfoWithoutVolumeMounts(buildInfo *build.Info, isStackManifest bool) *build.Info {
	result := buildInfo.Copy()
	if len(result.VolumesToInclude) > 0 {
		result.VolumesToInclude = nil
	}
	if isStackManifest && bc.oktetoContext.IsOkteto() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		result.Image = ""
	}
	return result
}

func getBuildInfoWithVolumeMounts(buildInfo *build.Info, isStackManifest bool, isOkteto bool) *build.Info {
	result := buildInfo.Copy()
	if isStackManifest && isOkteto {
		result.Image = ""
	}
	result.VolumesToInclude = getAccessibleVolumeMounts(buildInfo)
	return result
}

func getAccessibleVolumeMounts(buildInfo *build.Info) []build.VolumeMounts {
	accessibleVolumeMounts := make([]build.VolumeMounts, 0)
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

func validateServices(buildSection build.ManifestBuild, svcsToBuild []string) error {
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

func getImageChecker(buildInfo *build.Info, cfg oktetoBuilderConfigInterface, registry registryImageCheckerInterface, sbc smartBuildController, logger loggerInfo) imageCheckerInterface {
	var tagger imageTaggerInterface
	if serviceHasVolumesToInclude(buildInfo) {
		tagger = newImageWithVolumesTagger(cfg, sbc)
	} else {
		tagger = newImageTagger(cfg, sbc)
	}
	return newImageChecker(cfg, registry, tagger, logger)
}
