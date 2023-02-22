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

package deploy

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

const (
	headerUpgrade          = "Upgrade"
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
)

// Options options for deploy command
type Options struct {
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the patah to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath     string
	Name             string
	Namespace        string
	K8sContext       string
	Variables        []string
	Manifest         *model.Manifest
	Build            bool
	Dependencies     bool
	RunWithoutBash   bool
	RunInRemote      bool
	servicesToDeploy []string

	Repository string
	Branch     string
	Wait       bool
	Timeout    time.Duration

	ShowCTA bool
}

// DeployCommand defines the config for deploying an app
type DeployCommand struct {
	GetManifest func(path string) (*model.Manifest, error)

	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider
	Builder            *buildv2.OktetoBuilder
	GetExternalControl func(cp okteto.K8sClientProvider, filename string) (ExternalResourceInterface, error)
	GetDeployer        func(context.Context, *model.Manifest, *Options, string, *buildv2.OktetoBuilder) (deployerInterface, error)
	deployWaiter       deployWaiter
	cfgMapHandler      configMapHandler
	Fs                 afero.Fs
	DivertDriver       divert.Driver

	PipelineType model.Archetype
	isRemote     bool
}

type ExternalResourceInterface interface {
	Deploy(ctx context.Context, name string, ns string, externalInfo *externalresource.ExternalResource) error
	List(ctx context.Context, ns string, labelSelector string) ([]externalresource.ExternalResource, error)
	Validate(ctx context.Context, name string, ns string, externalInfo *externalresource.ExternalResource) error
}

type deployerInterface interface {
	deploy(context.Context, *Options) error
	cleanUp(context.Context, error)
}

// Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Execute locally the list of commands specified in the 'deploy' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate cmd options
			if options.Dependencies && !okteto.IsOkteto() {
				return fmt.Errorf("'dependencies' is only supported in clusters that have Okteto installed")
			}

			if err := validateAndSet(options.Variables, os.Setenv); err != nil {
				return err
			}

			// This is needed because the deploy command needs the original kubeconfig configuration even in the execution within another
			// deploy command. If not, we could be proxying a proxy and we would be applying the incorrect deployed-by label
			os.Setenv(constants.OktetoSkipConfigCredentialsUpdate, "false")
			if options.ManifestPath != "" {
				// if path is absolute, its transformed to rel from root
				initialCWD, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get the current working directory: %w", err)
				}
				manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, options.ManifestPath)
				if err != nil {
					return err
				}
				// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
				options.ManifestPathFlag = manifestPathFlag

				// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
				uptManifestPath, err := model.UpdateCWDtoManifestPath(options.ManifestPath)
				if err != nil {
					return err
				}
				options.ManifestPath = uptManifestPath
			}
			if err := contextCMD.LoadContextFromPath(ctx, options.Namespace, options.K8sContext, options.ManifestPath); err != nil {
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.CloudURL).Error() {
					return err
				}
				if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.ContextOptions{Namespace: options.Namespace}); err != nil {
					return err
				}
			}

			if okteto.IsOkteto() {
				create, err := utils.ShouldCreateNamespace(ctx, okteto.Context().Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand()
					if err != nil {
						return err
					}
					if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.Context().Namespace}); err != nil {
						return err
					}
				}
			}

			options.ShowCTA = oktetoLog.IsInteractive()
			options.servicesToDeploy = args

			k8sClientProvider := okteto.NewK8sClientProvider()
			c := &DeployCommand{
				GetManifest: model.GetManifestV2,

				GetExternalControl: getExternalControlFromCtx,
				K8sClientProvider:  k8sClientProvider,
				GetDeployer:        getDeployer,
				Builder:            buildv2.NewBuilderFromScratch(),
				deployWaiter:       newDeployWaiter(k8sClientProvider),
				isRemote:           utils.LoadBoolean(constants.OKtetoDeployRemote),
				cfgMapHandler:      newConfigmapHandler(k8sClientProvider),
				Fs:                 afero.NewOsFs(),
			}
			startTime := time.Now()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				err := c.RunDeploy(ctx, options)

				deployType := "custom"
				hasDependencySection := false
				hasBuildSection := false
				if options.Manifest != nil {
					if options.Manifest.IsV2 &&
						options.Manifest.Deploy != nil &&
						options.Manifest.Deploy.ComposeSection != nil &&
						options.Manifest.Deploy.ComposeSection.ComposesInfo != nil {
						deployType = "compose"
					}

					hasDependencySection = options.Manifest.IsV2 && len(options.Manifest.Dependencies) > 0
					hasBuildSection = options.Manifest.IsV2 && len(options.Manifest.Build) > 0
				}

				analytics.TrackDeploy(analytics.TrackDeployMetadata{
					Success:                err == nil,
					IsOktetoRepo:           utils.IsOktetoRepo(),
					Duration:               time.Since(startTime),
					PipelineType:           c.PipelineType,
					DeployType:             deployType,
					IsPreview:              os.Getenv(model.OktetoCurrentDeployBelongsToPreview) == "true",
					HasDependenciesSection: hasDependencySection,
					HasBuildSection:        hasBuildSection,
				})

				exit <- err
			}()

			select {
			case <-stop:
				oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
				oktetoLog.Spinner("Shutting down...")
				oktetoLog.StartSpinner()
				defer oktetoLog.StopSpinner()

				deployer, err := c.GetDeployer(ctx, options.Manifest, options, "", nil)
				if err != nil {
					return err
				}
				deployer.cleanUp(ctx, oktetoErrors.ErrIntSig)
				return oktetoErrors.ErrIntSig
			case err := <-exit:
				return err
			}
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment is deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment is deployed")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images when deploying the development environment")
	cmd.Flags().BoolVarP(&options.Dependencies, "dependencies", "", false, "deploy the dependencies from manifest")
	cmd.Flags().BoolVarP(&options.RunWithoutBash, "no-bash", "", false, "execute commands without bash")
	cmd.Flags().BoolVarP(&options.RunInRemote, "remote", "", false, "force run deploy commands in remote")

	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the development environment is deployed (defaults to false)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")

	return cmd
}

// RunDeploy runs the deploy sequence
func (dc *DeployCommand) RunDeploy(ctx context.Context, deployOptions *Options) error {
	var err error
	oktetoLog.SetStage("Load manifest")
	deployOptions.Manifest, err = dc.GetManifest(deployOptions.ManifestPath)
	if err != nil {
		return err
	}
	oktetoLog.Debug("found okteto manifest")
	dc.PipelineType = deployOptions.Manifest.Type

	if deployOptions.Manifest.Deploy == nil {
		return oktetoErrors.ErrManifestFoundButNoDeployCommands
	}
	if len(deployOptions.servicesToDeploy) > 0 && deployOptions.Manifest.Deploy.ComposeSection == nil {
		return oktetoErrors.ErrDeployCantDeploySvcsIfNotCompose
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	// We need to create a client that doesn't go through the proxy to create
	// the configmap without the deployedByLabel
	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)

	dc.addEnvVars(ctx, cwd)

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c); err != nil {
		return err
	}

	data := &pipeline.CfgData{
		Name:       deployOptions.Name,
		Namespace:  deployOptions.Manifest.Namespace,
		Repository: os.Getenv(model.GithubRepositoryEnvVar),
		Branch:     os.Getenv(model.OktetoGitBranchEnvVar),
		Filename:   deployOptions.ManifestPathFlag,
		Status:     pipeline.ProgressingStatus,
		Manifest:   deployOptions.Manifest.Manifest,
		Icon:       deployOptions.Manifest.Icon,
	}

	if !deployOptions.Manifest.IsV2 && deployOptions.Manifest.Type == model.StackType {
		data.Manifest = deployOptions.Manifest.Deploy.ComposeSection.Stack.Manifest
	}

	cfg, err := dc.cfgMapHandler.translateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	if err := dc.validateK8sResources(ctx, deployOptions.Manifest); err != nil {
		return err
	}

	os.Setenv(constants.OktetoNameEnvVar, deployOptions.Name)

	if err := dc.deployDependencies(ctx, deployOptions); err != nil {
		if errStatus := dc.cfgMapHandler.updateConfigMap(ctx, cfg, data, err); errStatus != nil {
			return errStatus
		}
		return err
	}

	if err := buildImages(ctx, dc.Builder.Build, dc.Builder.GetServicesToBuild, deployOptions); err != nil {
		return dc.cfgMapHandler.updateConfigMap(ctx, cfg, data, err)
	}

	deployer, err := dc.GetDeployer(ctx, deployOptions.Manifest, deployOptions, cwd, dc.Builder)
	if err != nil {
		return err
	}

	err = deployer.deploy(ctx, deployOptions)
	if err != nil {
		if err == oktetoErrors.ErrIntSig {
			return nil
		}
		err = oktetoErrors.UserError{E: err}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, err.Error())
		data.Status = pipeline.ErrorStatus
	} else {
		oktetoLog.SetStage("")
		hasDeployed, err := pipeline.HasDeployedSomething(ctx, deployOptions.Name, deployOptions.Manifest.Namespace, c)
		if err != nil {
			return err
		}
		if hasDeployed {
			if deployOptions.Wait {
				if err := dc.deployWaiter.wait(ctx, deployOptions); err != nil {
					return err
				}
			}
			if !utils.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
				eg := &endpointGetter{
					K8sClientProvider:  dc.K8sClientProvider,
					GetExternalControl: dc.GetExternalControl,
					TempKubeconfigFile: dc.TempKubeconfigFile,
				}
				if err := eg.showEndpoints(ctx, &EndpointsOptions{Name: deployOptions.Name, Namespace: deployOptions.Manifest.Namespace}); err != nil {
					oktetoLog.Infof("could not retrieve endpoints: %s", err)
				}
			}
			if deployOptions.ShowCTA {
				oktetoLog.Success(succesfullyDeployedmsg, deployOptions.Name)
				if oktetoLog.IsInteractive() {
					oktetoLog.Information("Run 'okteto up' to activate your development container")
				}
			}
			pipeline.AddDevAnnotations(ctx, deployOptions.Manifest, c)
		}
		data.Status = pipeline.DeployedStatus
	}

	if err := dc.cfgMapHandler.updateConfigMap(ctx, cfg, data, err); err != nil {
		return err
	}

	return err
}

func (dc *DeployCommand) validateK8sResources(ctx context.Context, manifest *model.Manifest) error {
	if manifest.External != nil {
		// In a cluster not managed by Okteto it is not necessary to validate the externals
		// because they will not be deployed.
		if okteto.IsOkteto() {
			control, err := dc.GetExternalControl(dc.K8sClientProvider, dc.TempKubeconfigFile)
			if err != nil {
				return err
			}

			for externalName, externalInfo := range manifest.External {
				err := control.Validate(ctx, externalName, manifest.Namespace, externalInfo)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func buildImages(ctx context.Context, build func(context.Context, *types.BuildOptions) error, getServicesToBuild func(context.Context, *model.Manifest, []string) ([]string, error), deployOptions *Options) error {
	var stackServicesWithBuild map[string]bool

	if stack := deployOptions.Manifest.GetStack(); stack != nil {
		stackServicesWithBuild = stack.GetServicesWithBuildSection()
	}

	allServicesWithBuildSection := deployOptions.Manifest.GetBuildServices()
	oktetoManifestServicesWithBuild := setDifference(allServicesWithBuildSection, stackServicesWithBuild) // Warning: this way of getting the oktetoManifestServicesWithBuild is highly dependent on the manifest struct as it is now. We are assuming that: *okteto* manifest build = manifest build - stack build section
	servicesToDeployWithBuild := setIntersection(allServicesWithBuildSection, sliceToSet(deployOptions.servicesToDeploy))
	// We need to build:
	// - All the services that have a build section defined in the *okteto* manifest
	// - Services from *deployOptions.servicesToDeploy* that have a build section

	servicesToBuildSet := setUnion(oktetoManifestServicesWithBuild, servicesToDeployWithBuild)

	if deployOptions.Build {
		buildOptions := &types.BuildOptions{
			EnableStages: true,
			Manifest:     deployOptions.Manifest,
			CommandArgs:  setToSlice(servicesToBuildSet),
		}
		oktetoLog.Debug("force build from manifest definition")
		if errBuild := build(ctx, buildOptions); errBuild != nil {
			return errBuild
		}
	} else {
		servicesToBuild, err := getServicesToBuild(ctx, deployOptions.Manifest, setToSlice(servicesToBuildSet))
		if err != nil {
			return err
		}

		if len(servicesToBuild) != 0 {
			buildOptions := &types.BuildOptions{
				EnableStages: true,
				Manifest:     deployOptions.Manifest,
				CommandArgs:  servicesToBuild,
			}

			if errBuild := build(ctx, buildOptions); errBuild != nil {
				return errBuild
			}
		}
	}

	return nil
}

func sliceToSet[T comparable](slice []T) map[T]bool {
	set := make(map[T]bool)
	for _, value := range slice {
		set[value] = true
	}
	return set
}

func setToSlice[T comparable](set map[T]bool) []T {
	slice := make([]T, 0, len(set))
	for value := range set {
		slice = append(slice, value)
	}
	return slice
}

func setIntersection[T comparable](set1, set2 map[T]bool) map[T]bool {
	intersection := make(map[T]bool)
	for value := range set1 {
		if _, ok := set2[value]; ok {
			intersection[value] = true
		}
	}
	return intersection
}

func setUnion[T comparable](set1, set2 map[T]bool) map[T]bool {
	union := make(map[T]bool)
	for value := range set1 {
		union[value] = true
	}
	for value := range set2 {
		union[value] = true
	}
	return union
}

func setDifference[T comparable](set1, set2 map[T]bool) map[T]bool {
	difference := make(map[T]bool)
	for value := range set1 {
		if _, ok := set2[value]; !ok {
			difference[value] = true
		}
	}
	return difference
}

func getDefaultTimeout() time.Duration {
	defaultTimeout := 5 * time.Minute
	t := os.Getenv(model.OktetoTimeoutEnvVar)
	if t == "" {
		return defaultTimeout
	}

	parsed, err := time.ParseDuration(t)
	if err != nil {
		oktetoLog.Infof("OKTETO_TIMEOUT value is not a valid duration: %s", t)
		oktetoLog.Infof("timeout fallback to defaultTimeout")
		return defaultTimeout
	}

	return parsed
}

func getDeployer(ctx context.Context, manifest *model.Manifest, opts *Options, cwd string, builder *buildv2.OktetoBuilder) (deployerInterface, error) {
	var (
		deployer deployerInterface
		err      error
	)

	isRemote := utils.LoadBoolean(constants.OKtetoDeployRemote)

	if isRemote || manifest.Deploy.Image == "" {
		deployer, err = newLocalDeployer(ctx, cwd, opts)
		if err != nil {
			return nil, fmt.Errorf("could not initialize local deploy command: %w", err)
		}
		oktetoLog.Info("Deploying locally...")
	} else {
		deployer = newRemoteDeployer(builder)
		oktetoLog.Info("Deploying remotely...")
	}
	return deployer, nil
}

// deployDependencies deploy the dependencies in the manifest
func (dc *DeployCommand) deployDependencies(ctx context.Context, deployOptions *Options) error {
	for depName, dep := range deployOptions.Manifest.Dependencies {
		oktetoLog.Information("Deploying dependency '%s'", depName)
		oktetoLog.SetStage(fmt.Sprintf("Deploying dependency %s", depName))
		dep.Variables = append(dep.Variables, model.EnvVar{
			Name:  "OKTETO_ORIGIN",
			Value: "okteto-deploy",
		})
		namespace := okteto.Context().Namespace
		if dep.Namespace != "" {
			namespace = dep.Namespace
		}
		pipOpts := &pipelineCMD.DeployOptions{
			Name:         depName,
			Repository:   dep.Repository,
			Branch:       dep.Branch,
			File:         dep.ManifestPath,
			Variables:    model.SerializeEnvironmentVars(dep.Variables),
			Wait:         dep.Wait,
			Timeout:      dep.GetTimeout(deployOptions.Timeout),
			SkipIfExists: !deployOptions.Dependencies,
			Namespace:    namespace,
		}
		pc, err := pipelineCMD.NewCommand()
		if err != nil {
			return fmt.Errorf("could not create pipeline command: %w", err)
		}
		if err := pc.ExecuteDeployPipeline(ctx, pipOpts); err != nil {
			return err
		}
	}
	oktetoLog.SetStage("")
	return nil
}
