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
	"time"

	"github.com/okteto/okteto/cmd/build/basic"
	"github.com/okteto/okteto/cmd/build/v2/checker"
	"github.com/okteto/okteto/cmd/build/v2/environment"
	"github.com/okteto/okteto/cmd/build/v2/metadata"
	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/build"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/devenvironment"
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

type oktetoRegistryInterface interface {
	GetImageTagWithDigest(imageTag string) (string, error)
	IsOktetoRegistry(image string) bool
	GetImageReference(image string) (registry.OktetoImageReference, error)
	HasGlobalPushAccess() (bool, error)
	IsGlobalRegistry(image string) bool

	GetRegistryAndRepo(image string) (string, string)
	GetRepoNameAndTag(repo string) (string, string)
	GetDevImageFromGlobal(imageWithDigest string) string
	Clone(from, to string) (string, error)
}

// oktetoBuilderConfigInterface returns the configuration that the builder has for the registry and project
type oktetoBuilderConfigInterface interface {
	HasGlobalAccess() bool
	IsCleanProject() bool
	GetGitCommit() string
	IsOkteto() bool
	GetAnonymizedRepo() string
}

type metadataCollectorInterface interface {
	CollectMetadata(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, toBuildSvcs []string) error
	GetMetadataMap() map[string]*analytics.ImageBuildMetadata
}

type imageCheckerInterface interface {
	CheckImages(ctx context.Context, manifestName string, buildManifest build.ManifestBuild, toBuildSvcs []string) ([]string, []string, error)
	CloneGlobalImagesToDev(manifestName string, buildManifest build.ManifestBuild, svcsToClone []string) error
	GetImageDigestReferenceForServiceDeploy(manifestName, service string, buildInfo *build.Info) (string, error)
}

// OktetoBuilder builds the images
type OktetoBuilder struct {
	basic.Builder

	Registry      oktetoRegistryInterface
	Config        oktetoBuilderConfigInterface
	oktetoContext buildCmd.OktetoContextInterface
	imageChecker  imageCheckerInterface
	tagger        imageTagger

	smartBuildCtrl       *smartbuild.Ctrl
	serviceEnvVarsSetter *environment.ServiceEnvVarsSetter
	metadataCollector    metadataCollectorInterface

	ioCtrl    *io.Controller
	k8sLogger *io.K8sLogger

	onBuildFinish []OnBuildFinish
}

type OnBuildFinish func(ctx context.Context, meta *analytics.ImageBuildMetadata)

// NewBuilder creates a new okteto builder
func NewBuilder(builder buildCmd.OktetoBuilderInterface, registryCtrl oktetoRegistryInterface, ioCtrl *io.Controller, okCtx okteto.ContextInterface, k8sLogger *io.K8sLogger, onBuildFinish []OnBuildFinish) *OktetoBuilder {
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		ioCtrl.Logger().Infof("could not get working dir: %s", err)
	}
	topLevelGitDir, err := repository.FindTopLevelGitDir(wd)
	if err != nil {
		ioCtrl.Logger().Infof("could not get top level git dir: %s", err)
	}
	if topLevelGitDir != "" {
		wd = topLevelGitDir
	}
	gitRepo := repository.NewRepository(wd)
	config := getConfigStateless(registryCtrl, gitRepo, ioCtrl.Logger(), okCtx.IsOktetoCluster())
	smartBuildCtrl := smartbuild.NewSmartBuildCtrl(gitRepo, registryCtrl, config.fs, ioCtrl, wdCtrl)
	tagger := newImageTagger(config, smartBuildCtrl)
	metadataCollector := metadata.NewMetadataCollector(okCtx, config, smartBuildCtrl, ioCtrl)
	imageCtrl := registry.NewImageCtrl(okCtx)

	serviceEnvVarsSetter := environment.NewServiceEnvVarsSetter(ioCtrl, registryCtrl)
	serviceEnvVarsSetter.SetEnvVar(OktetoEnableSmartBuildEnvVar, strconv.FormatBool(config.isSmartBuildsEnable))

	imageChecker := checker.NewImageCacheChecker(
		okCtx.GetNamespace(),
		okCtx.GetRegistryURL(),
		tagger,
		smartBuildCtrl,
		imageCtrl,
		metadataCollector,
		registryCtrl,
		ioCtrl,
		serviceEnvVarsSetter)
	ob := &OktetoBuilder{
		Builder:              basic.Builder{BuildRunner: builder, IoCtrl: ioCtrl},
		Registry:             registryCtrl,
		imageChecker:         imageChecker,
		Config:               config,
		ioCtrl:               ioCtrl,
		smartBuildCtrl:       smartBuildCtrl,
		oktetoContext:        okCtx,
		k8sLogger:            k8sLogger,
		metadataCollector:    metadataCollector,
		onBuildFinish:        onBuildFinish,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		tagger:               tagger,
	}

	return ob

}

// NewBuilderFromScratch creates a new okteto builder
func NewBuilderFromScratch(ioCtrl *io.Controller, onBuildFinish []OnBuildFinish) *OktetoBuilder {
	builder := buildCmd.NewOktetoBuilder(
		&okteto.ContextStateless{
			Store: okteto.GetContextStore(),
		},
		afero.NewOsFs(),
		ioCtrl,
	)
	reg := registry.NewOktetoRegistry(okteto.Config{})
	wdCtrl := filesystem.NewOsWorkingDirectoryCtrl()
	wd, err := wdCtrl.Get()
	if err != nil {
		ioCtrl.Logger().Infof("could not get working dir: %s", err)
	}
	topLevelGitDir, err := repository.FindTopLevelGitDir(wd)
	if err != nil {
		ioCtrl.Logger().Infof("could not get top level git dir: %s", err)
	}
	if topLevelGitDir != "" {
		wd = topLevelGitDir
	}
	gitRepo := repository.NewRepository(wd)
	config := getConfig(reg, gitRepo, ioCtrl.Logger())

	okCtx := &okteto.ContextStateless{
		Store: okteto.GetContextStore(),
	}

	smartBuildCtrl := smartbuild.NewSmartBuildCtrl(gitRepo, reg, config.fs, ioCtrl, wdCtrl)
	tagger := newImageTagger(config, smartBuildCtrl)
	metadataCollector := metadata.NewMetadataCollector(okCtx, config, smartBuildCtrl, ioCtrl)
	imageCtrl := registry.NewImageCtrl(okCtx)

	serviceEnvVarsSetter := environment.NewServiceEnvVarsSetter(ioCtrl, reg)
	serviceEnvVarsSetter.SetEnvVar(OktetoEnableSmartBuildEnvVar, strconv.FormatBool(config.isSmartBuildsEnable))

	imageChecker := checker.NewImageCacheChecker(
		okCtx.GetNamespace(),
		okCtx.GetRegistryURL(),
		tagger,
		smartBuildCtrl,
		imageCtrl,
		metadataCollector,
		reg,
		ioCtrl,
		serviceEnvVarsSetter)

	return &OktetoBuilder{
		Builder:              basic.Builder{BuildRunner: builder, IoCtrl: ioCtrl},
		Registry:             reg,
		imageChecker:         imageChecker,
		Config:               config,
		ioCtrl:               ioCtrl,
		smartBuildCtrl:       smartBuildCtrl,
		oktetoContext:        okCtx,
		metadataCollector:    metadataCollector,
		onBuildFinish:        onBuildFinish,
		serviceEnvVarsSetter: serviceEnvVarsSetter,
		tagger:               tagger,
	}
}

// GetBuildEnvVars returns the build env vars
func (ob *OktetoBuilder) GetBuildEnvVars() map[string]string {
	return ob.serviceEnvVarsSetter.GetBuildEnvVars()
}

// IsV1 returns false since it is a builder v2
func (*OktetoBuilder) IsV1() bool {
	return false
}

// Build builds the images defined by a manifest
// TODO: Function with cyclomatic complexity higher than threshold. Refactor function in order to reduce its complexity
// skipcq: GO-R1005
func (ob *OktetoBuilder) Build(ctx context.Context, options *types.BuildOptions) error {
	if options.File != "" {
		workdir := filesystem.GetWorkdirFromManifestPath(options.File)
		if err := os.Chdir(workdir); err != nil {
			return err
		}
		options.File = filesystem.GetManifestPathFromWorkdir(options.File, workdir)
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
		options.Manifest.Name = inferer.InferName(ctx, wd, ob.oktetoContext.GetNamespace(), options.File)
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

	// send analytics for all builds after Build
	buildsAnalytics := make([]*analytics.ImageBuildMetadata, 0)

	// send all events appended on each build
	defer func([]*analytics.ImageBuildMetadata) {
		for _, meta := range buildsAnalytics {
			m := meta
			for _, fn := range ob.onBuildFinish {
				fn(ctx, m)
			}
		}
	}(buildsAnalytics)

	bg := newBuildGraph(buildManifest, toBuildSvcs)
	tree, err := bg.GetGraph()
	if err != nil {
		return err
	}
	tree, err = tree.Subtree(toBuildSvcs...)
	if err != nil {
		return err
	}
	toBuildSvcs = tree.Ordered()

	ob.ioCtrl.Logger().Infof("Images to build: [%s]", strings.Join(toBuildSvcs, ", "))

	sp := ob.ioCtrl.Out().Spinner("Collecting metadata for the images")
	sp.Start()
	if err := ob.metadataCollector.CollectMetadata(ctx, options.Manifest.Name, options.Manifest.Build, toBuildSvcs); err != nil {
		return err
	}
	sp.Stop()
	metaMap := ob.metadataCollector.GetMetadataMap()

	notCachedServices := toBuildSvcs
	var cachedServices []string
	if !options.NoCache && ob.smartBuildCtrl.IsEnabled() {
		sp := ob.ioCtrl.Out().Spinner("Checking if the images are already built from cache...")
		sp.Start()
		defer sp.Stop()
		cachedServices, notCachedServices, err = ob.imageChecker.CheckImages(ctx, options.Manifest.Name, options.Manifest.Build, toBuildSvcs)
		if err != nil {
			return fmt.Errorf("error checking images: %w", err)
		}
		ob.ioCtrl.Logger().Infof("Images cached: [%s]", strings.Join(cachedServices, ", "))
		ob.ioCtrl.Logger().Infof("Images not cached: [%s]", strings.Join(notCachedServices, ", "))

		sp = ob.ioCtrl.Out().Spinner("Cloning global images to dev...")
		sp.Start()
		if err := ob.imageChecker.CloneGlobalImagesToDev(options.Manifest.Name, options.Manifest.Build, cachedServices); err != nil {
			return fmt.Errorf("error cloning global images to dev: %w", err)
		}
		sp.Stop()
	}

	for _, svcToBuild := range notCachedServices {
		if options.EnableStages {
			ob.ioCtrl.SetStage(fmt.Sprintf("Building service %s", svcToBuild))
		}
		buildSvcInfo := options.Manifest.Build[svcToBuild]
		if !ob.oktetoContext.IsOktetoCluster() && buildSvcInfo.Image == "" {
			return fmt.Errorf("'build.%s.image' is required if your context doesn't have Okteto installed", svcToBuild)
		}
		buildDurationStart := time.Now()
		imageTag, err := ob.buildServiceImages(ctx, options.Manifest, svcToBuild, options)
		buildkitRunner, ok := ob.Builder.BuildRunner.(*buildCmd.OktetoBuilder)
		if ok {
			buildkitMetadata := buildkitRunner.GetMetadata()
			if buildkitMetadata != nil {
				metaMap[svcToBuild].WaitForBuildkitAvailable = buildkitMetadata.WaitForBuildkitAvailableTime
			}
		}
		if err != nil {
			return fmt.Errorf("error building service '%s': %w", svcToBuild, err)
		}
		metaMap[svcToBuild].BuildDuration = time.Since(buildDurationStart)
		metaMap[svcToBuild].Success = true

		ob.serviceEnvVarsSetter.SetServiceEnvVars(svcToBuild, imageTag)
	}

	if options.EnableStages {
		ob.ioCtrl.SetStage("")
	}
	return options.Manifest.ExpandEnvVars()
}

// buildServiceImages builds the images for the given service.
// if service has volumes to include but is not okteto, an error is returned.
// Returned image reference includes the digest
func (bc *OktetoBuilder) buildServiceImages(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	buildSvcInfo := manifest.Build[svcName]

	switch {
	case serviceHasDockerfile(buildSvcInfo):
		return bc.buildSvcFromDockerfile(ctx, manifest, svcName, options)

	default:
		bc.ioCtrl.Logger().Info(fmt.Sprintf("could not build service %s, due to not having Dockerfile defined or volumes to include", svcName))
	}
	return "", nil
}

func (bc *OktetoBuilder) buildSvcFromDockerfile(ctx context.Context, manifest *model.Manifest, svcName string, options *types.BuildOptions) (string, error) {
	bc.ioCtrl.Logger().Info(fmt.Sprintf("Building service '%s' from Dockerfile", svcName))
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := bc.getBuildInfoWithoutVolumeMounts(manifest.Build[svcName], isStackManifest)
	var buildHash string
	if bc.smartBuildCtrl.IsEnabled() {
		buildHash = bc.smartBuildCtrl.GetBuildHash(buildSvcInfo, svcName)
	}
	it := newImageTagger(bc.Config, bc.smartBuildCtrl)
	tagsToBuild := it.getServiceDevImageReference(manifest.Name, svcName, buildSvcInfo)
	imageCtrl := registry.NewImageCtrl(bc.oktetoContext)
	globalImage := it.GetGlobalTagFromDevIfNeccesary(tagsToBuild, bc.oktetoContext.GetNamespace(), bc.oktetoContext.GetRegistryURL(), buildHash, imageCtrl)
	if globalImage != "" {
		tagsToBuild = fmt.Sprintf("%s,%s", tagsToBuild, globalImage)
	}
	buildSvcInfo.Image = tagsToBuild
	if err := buildSvcInfo.AddArgs(bc.serviceEnvVarsSetter.GetBuildEnvVars()); err != nil {
		return "", fmt.Errorf("error expanding build args from service '%s': %w", svcName, err)
	}

	buildOptions := buildCmd.OptsFromBuildInfo(manifest, svcName, buildSvcInfo, options, bc.Registry, bc.oktetoContext)

	if err := bc.Builder.Build(ctx, buildOptions); err != nil {
		return "", err
	}
	var imageTagWithDigest string
	tags := strings.Split(buildOptions.Tag, ",")

	// check that all tags are pushed and return the first one to not break any scenario
	for idx, tag := range tags {
		// check if the image is pushed to the dev registry if DevTag is set
		reference := tag
		digest, err := bc.Registry.GetImageTagWithDigest(reference)
		if err != nil {
			return "", fmt.Errorf("error accessing image at registry %s: %w", reference, err)
		}
		if idx == 0 {
			imageTagWithDigest = digest
		}
	}
	return imageTagWithDigest, nil
}

// serviceHasDockerfile returns true when service BuildInfo Dockerfile is not empty
func serviceHasDockerfile(buildInfo *build.Info) bool {
	return buildInfo.Dockerfile != ""
}

func (bc *OktetoBuilder) getBuildInfoWithoutVolumeMounts(buildInfo *build.Info, isStackManifest bool) *build.Info {
	result := buildInfo.Copy()
	if len(result.VolumesToInclude) > 0 {
		result.VolumesToInclude = nil
	}
	if isStackManifest && bc.oktetoContext.IsOktetoCluster() && !bc.Registry.IsOktetoRegistry(buildInfo.Image) {
		result.Image = ""
	}
	return result
}

// getToBuildSvcs returns a list of services to be built. It gets the list from the command args if specified, if not,
// it returns the list of services from the manifest
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

// func getImageChecker(cfg oktetoBuilderConfigInterface, registry registryImageCheckerInterface, sbc smartBuildController, logger loggerInfo) imageChecker {
// 	tagger := newImageTagger(cfg, sbc)
// 	return newImageChecker(cfg, registry, tagger, logger)
// }
