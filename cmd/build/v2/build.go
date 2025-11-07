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
	"github.com/okteto/okteto/cmd/build/v2/environment"
	"github.com/okteto/okteto/cmd/build/v2/smartbuild"
	buildTypes "github.com/okteto/okteto/cmd/build/v2/types"
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

// OktetoBuilder builds the images
type OktetoBuilder struct {
	basic.Builder

	Registry      oktetoRegistryInterface
	Config        oktetoBuilderConfigInterface
	oktetoContext buildCmd.OktetoContextInterface

	tagger imageTagger

	smartBuildCtrl        *smartbuild.Ctrl
	serviceEnvVarsHandler *environment.ServiceEnvVarsHandler

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

	smartBuildConfig := smartbuild.NewConfig()
	tagger := newImageTagger(config, smartBuildConfig)
	imageCtrl := registry.NewImageCtrl(okCtx)

	serviceEnvVarsHandler := environment.NewServiceEnvVarsHandler(ioCtrl, registryCtrl)
	serviceEnvVarsHandler.SetEnvVar(OktetoEnableSmartBuildEnvVar, strconv.FormatBool(config.isSmartBuildsEnable))

	smartBuildCtrl := smartbuild.NewSmartBuildCtrl(gitRepo, registryCtrl, config.fs, ioCtrl, wdCtrl, smartBuildConfig, tagger, imageCtrl, serviceEnvVarsHandler, okCtx.GetNamespace(), okCtx.GetRegistryURL())

	ob := &OktetoBuilder{
		Builder:               basic.Builder{BuildRunner: builder, IoCtrl: ioCtrl},
		Registry:              registryCtrl,
		Config:                config,
		ioCtrl:                ioCtrl,
		smartBuildCtrl:        smartBuildCtrl,
		oktetoContext:         okCtx,
		k8sLogger:             k8sLogger,
		onBuildFinish:         onBuildFinish,
		serviceEnvVarsHandler: serviceEnvVarsHandler,
		tagger:                tagger,
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

	smartBuildConfig := smartbuild.NewConfig()
	tagger := newImageTagger(config, smartBuildConfig)
	imageCtrl := registry.NewImageCtrl(okCtx)

	serviceEnvVarsHandler := environment.NewServiceEnvVarsHandler(ioCtrl, reg)
	serviceEnvVarsHandler.SetEnvVar(OktetoEnableSmartBuildEnvVar, strconv.FormatBool(config.isSmartBuildsEnable))

	smartBuildCtrl := smartbuild.NewSmartBuildCtrl(gitRepo, reg, config.fs, ioCtrl, wdCtrl, smartBuildConfig, tagger, imageCtrl, serviceEnvVarsHandler, okCtx.GetNamespace(), okCtx.GetRegistryURL())

	return &OktetoBuilder{
		Builder:               basic.Builder{BuildRunner: builder, IoCtrl: ioCtrl},
		Registry:              reg,
		Config:                config,
		ioCtrl:                ioCtrl,
		smartBuildCtrl:        smartBuildCtrl,
		oktetoContext:         okCtx,
		onBuildFinish:         onBuildFinish,
		serviceEnvVarsHandler: serviceEnvVarsHandler,
		tagger:                tagger,
	}
}

// GetBuildEnvVars returns the build env vars
func (ob *OktetoBuilder) GetBuildEnvVars() map[string]string {
	return ob.serviceEnvVarsHandler.GetBuildEnvVars()
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

	ob.ioCtrl.Logger().Infof("Ordered services: [%s]", strings.Join(toBuildSvcs, ", "))

	svcInfos := buildTypes.NewBuildInfos(options.Manifest.Name, ob.oktetoContext.GetNamespace(), ob.Config.GetAnonymizedRepo(), toBuildSvcs)

	// send all events appended on each build
	defer func([]*buildTypes.BuildInfo) {
		for _, info := range svcInfos {
			m := info.Metadata()
			for _, fn := range ob.onBuildFinish {
				fn(ctx, m)
			}
		}
	}(svcInfos)

	var cachedSvcs []*buildTypes.BuildInfo
	notCachedSvcs := svcInfos
	if !options.NoCache && ob.smartBuildCtrl.IsEnabled() {
		if options.EnableStages {
			ob.ioCtrl.SetStage("Checking if images were already built...")
		}
		sp := ob.ioCtrl.Out().Spinner("Checking if the images are already built from cache...")
		sp.Start()
		defer sp.Stop()
		cachedSvcs, notCachedSvcs, err = ob.smartBuildCtrl.CheckServicesCache(ctx, options.Manifest.Name, options.Manifest.Build, svcInfos)
		if err != nil {
			return fmt.Errorf("error checking images: %w", err)
		}
		sp.Stop()
		ob.ioCtrl.Logger().Infof("Images cached: %v", cachedSvcs)
		ob.ioCtrl.Logger().Infof("Images not cached: %v", notCachedSvcs)
	}

	for _, svcToBuild := range notCachedSvcs {
		if options.EnableStages {
			ob.ioCtrl.SetStage(fmt.Sprintf("Building service %s", svcToBuild.Name()))
		}
		buildSvcInfo := options.Manifest.Build[svcToBuild.Name()]
		if !ob.oktetoContext.IsOktetoCluster() && buildSvcInfo.Image == "" {
			return fmt.Errorf("'build.%s.image' is required if your context doesn't have Okteto installed", svcToBuild)
		}

		buildDurationStart := time.Now()
		imageTag, err := ob.buildServiceImages(ctx, options.Manifest, svcToBuild, options)

		buildDuration := time.Since(buildDurationStart)

		waitForBuildkitAvailable := time.Duration(0)
		buildkitRunner, ok := ob.Builder.BuildRunner.(*buildCmd.OktetoBuilder)
		if ok {
			buildkitMetadata := buildkitRunner.GetMetadata()
			if buildkitMetadata != nil {
				waitForBuildkitAvailable = buildkitMetadata.WaitForBuildkitAvailableTime
			}
		}
		svcToBuild.SetBuildDuration(buildDuration, waitForBuildkitAvailable, err == nil)

		if err != nil {
			return fmt.Errorf("error building service '%s': %w", svcToBuild, err)
		}

		ob.serviceEnvVarsHandler.SetServiceEnvVars(svcToBuild.Name(), imageTag)
	}

	if options.EnableStages {
		ob.ioCtrl.SetStage("")
	}
	return options.Manifest.ExpandEnvVars()
}

// buildServiceImages builds the images for the given service.
// if service has volumes to include but is not okteto, an error is returned.
// Returned image reference includes the digest
func (bc *OktetoBuilder) buildServiceImages(ctx context.Context, manifest *model.Manifest, svc *buildTypes.BuildInfo, options *types.BuildOptions) (string, error) {
	buildSvcInfo := manifest.Build[svc.Name()]

	switch {
	case serviceHasDockerfile(buildSvcInfo):
		return bc.buildSvcFromDockerfile(ctx, manifest, svc, options)

	default:
		bc.ioCtrl.Logger().Info(fmt.Sprintf("could not build service %s, due to not having Dockerfile defined or volumes to include", svc.Name()))
	}
	return "", nil
}

func (bc *OktetoBuilder) buildSvcFromDockerfile(ctx context.Context, manifest *model.Manifest, svcInfo *buildTypes.BuildInfo, options *types.BuildOptions) (string, error) {
	bc.ioCtrl.Logger().Info(fmt.Sprintf("Building service '%s' from Dockerfile", svcInfo.Name()))
	isStackManifest := manifest.Type == model.StackType
	buildSvcInfo := bc.getBuildInfoWithoutVolumeMounts(manifest.Build[svcInfo.Name()], isStackManifest)
	var buildHash string
	if bc.smartBuildCtrl.IsEnabled() {
		buildHash = bc.smartBuildCtrl.GetBuildHash(buildSvcInfo, svcInfo.Name())
	}
	it := newImageTagger(bc.Config, bc.smartBuildCtrl)
	tagsToBuild := it.getServiceDevImageReference(manifest.Name, svcInfo.Name(), buildSvcInfo)
	imageCtrl := registry.NewImageCtrl(bc.oktetoContext)
	globalImage := it.GetGlobalTagFromDevIfNeccesary(tagsToBuild, bc.oktetoContext.GetNamespace(), bc.oktetoContext.GetRegistryURL(), buildHash, imageCtrl)
	if globalImage != "" {
		tagsToBuild = fmt.Sprintf("%s,%s", tagsToBuild, globalImage)
	}
	buildSvcInfo.Image = tagsToBuild
	if err := buildSvcInfo.AddArgs(bc.serviceEnvVarsHandler.GetBuildEnvVars()); err != nil {
		return "", fmt.Errorf("error expanding build args from service '%s': %w", svcInfo.Name(), err)
	}

	buildOptions := buildCmd.OptsFromBuildInfo(manifest, svcInfo.Name(), buildSvcInfo, options, bc.Registry, bc.oktetoContext)

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
