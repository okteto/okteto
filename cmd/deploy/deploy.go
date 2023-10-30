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
	"errors"
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
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/format"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

const (
	headerUpgrade          = "Upgrade"
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
)

var (
	errDepenNotAvailableInVanilla = errors.New("dependency deployment is only supported in contexts with Okteto installed")
)

// Options represents options for deploy command
type Options struct {
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the path to the manifest used though the command execution.
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

type builderInterface interface {
	Build(ctx context.Context, options *types.BuildOptions) error
	GetServicesToBuild(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error)
	GetBuildEnvVars() map[string]string
}

type portGetterFunc func(string) (int, error)

type getDeployerFunc func(
	context.Context, *Options,
	builderInterface,
	configMapHandler,
	okteto.K8sClientProvider,
	kubeConfigHandler,
	portGetterFunc,
) (deployerInterface, error)

// DeployCommand defines the config for deploying an app
type DeployCommand struct {
	GetManifest        func(path string) (*model.Manifest, error)
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider
	Builder            builderInterface
	GetExternalControl func(cfg *rest.Config) ExternalResourceInterface
	GetDeployer        getDeployerFunc
	EndpointGetter     func() (EndpointGetter, error)
	DeployWaiter       DeployWaiter
	CfgMapHandler      configMapHandler
	Fs                 afero.Fs
	DivertDriver       divert.Driver
	PipelineCMD        pipelineCMD.PipelineDeployerInterface
	AnalyticsTracker   analyticsTrackerInterface

	PipelineType       model.Archetype
	isRemote           bool
	runningInInstaller bool
}

type analyticsTrackerInterface interface {
	TrackDeploy(dm analytics.DeployMetadata)
	TrackImageBuild(...*analytics.ImageBuildMetadata)
}

type ExternalResourceInterface interface {
	Deploy(ctx context.Context, name string, ns string, externalInfo *externalresource.ExternalResource) error
	List(ctx context.Context, ns string, labelSelector string) ([]externalresource.ExternalResource, error)
}

type deployerInterface interface {
	deploy(context.Context, *Options) error
	cleanUp(context.Context, error)
}

func NewDeployExternalK8sControl(cfg *rest.Config) ExternalResourceInterface {
	return externalresource.NewExternalK8sControl(cfg)
}

// Deploy deploys the okteto manifest
func Deploy(ctx context.Context, at analyticsTrackerInterface) *cobra.Command {
	options := &Options{}
	fs := &DeployCommand{
		Fs: afero.NewOsFs(),
	}
	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Execute the list of commands specified in the 'deploy' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			// validate cmd options
			if options.Dependencies && !okteto.IsOkteto() {
				return fmt.Errorf("'dependencies' is only supported in contexts that have Okteto installed")
			}

			if err := validateAndSet(options.Variables, os.Setenv); err != nil {
				return err
			}

			// This is needed because the deploy command needs the original kubeconfig configuration even in the execution within another
			// deploy command. If not, we could be proxying a proxy and we would be applying the incorrect deployed-by label
			os.Setenv(constants.OktetoSkipConfigCredentialsUpdate, "false")

			err := checkOktetoManifestPathFlag(options, fs.Fs)
			if err != nil {
				return err
			}

			// Loads, updates and uses the context from path. If not found, it creates and uses a new context
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
			pc, err := pipelineCMD.NewCommand()
			if err != nil {
				return fmt.Errorf("could not create pipeline command: %w", err)
			}
			c := &DeployCommand{
				GetManifest: model.GetManifestV2,

				GetExternalControl: NewDeployExternalK8sControl,
				K8sClientProvider:  k8sClientProvider,
				GetDeployer:        GetDeployer,
				Builder:            buildv2.NewBuilderFromScratch(at),
				DeployWaiter:       NewDeployWaiter(k8sClientProvider),
				EndpointGetter:     NewEndpointGetter,
				isRemote:           utils.LoadBoolean(constants.OktetoDeployRemote),
				CfgMapHandler:      NewConfigmapHandler(k8sClientProvider),
				Fs:                 afero.NewOsFs(),
				PipelineCMD:        pc,
				runningInInstaller: config.RunningInInstaller(),
				AnalyticsTracker:   at,
			}
			startTime := time.Now()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				err := c.RunDeploy(ctx, options)

				c.trackDeploy(options.Manifest, options.RunInRemote, startTime, err)
				exit <- err
			}()

			select {
			case <-stop:
				oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
				oktetoLog.Spinner("Shutting down...")
				oktetoLog.StartSpinner()
				defer oktetoLog.StopSpinner()

				deployer, err := c.GetDeployer(ctx, options, nil, nil, k8sClientProvider, NewKubeConfig(), model.GetAvailablePort)
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
	oktetoLog.SetStage("Load manifest")
	manifest, err := dc.GetManifest(deployOptions.ManifestPath)
	if err != nil {
		return err
	}
	deployOptions.Manifest = manifest
	oktetoLog.Debug("found okteto manifest")
	dc.PipelineType = deployOptions.Manifest.Type

	if deployOptions.Manifest.Deploy == nil && !deployOptions.Manifest.HasDependencies() {
		return oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands
	}

	if len(deployOptions.servicesToDeploy) > 0 && deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.ComposeSection == nil {
		return oktetoErrors.ErrDeployCantDeploySvcsIfNotCompose
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	// We need to create a client that doesn't go through the proxy to create
	// the configmap without the deployedByLabel
	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}
	dc.addEnvVars(cwd)

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c); err != nil {
		return err
	}

	if dc.isRemote || dc.runningInInstaller {
		currentVars, err := dc.CfgMapHandler.getConfigmapVariablesEncoded(ctx, deployOptions.Name, deployOptions.Manifest.Namespace)
		if err != nil {
			return err
		}

		// when running in remote or installer variables should be retrieved from the saved value at configmap
		deployOptions.Variables = []string{}
		for _, v := range types.DecodeStringToDeployVariable(currentVars) {
			deployOptions.Variables = append(deployOptions.Variables, fmt.Sprintf("%s=%s", v.Name, v.Value))
		}
	}

	data := &pipeline.CfgData{
		Name:       deployOptions.Name,
		Namespace:  deployOptions.Manifest.Namespace,
		Repository: os.Getenv(model.GithubRepositoryEnvVar),
		Branch:     os.Getenv(constants.OktetoGitBranchEnvVar),
		Filename:   deployOptions.ManifestPathFlag,
		Status:     pipeline.ProgressingStatus,
		Manifest:   deployOptions.Manifest.Manifest,
		Icon:       deployOptions.Manifest.Icon,
		Variables:  deployOptions.Variables,
	}

	if !deployOptions.Manifest.IsV2 && deployOptions.Manifest.Type == model.StackType && deployOptions.Manifest.Deploy != nil {
		data.Manifest = deployOptions.Manifest.Deploy.ComposeSection.Stack.Manifest
	}

	cfg, err := dc.CfgMapHandler.translateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	os.Setenv(constants.OktetoNameEnvVar, deployOptions.Name)

	if err := dc.deployDependencies(ctx, deployOptions); err != nil {
		if errStatus := dc.CfgMapHandler.updateConfigMap(ctx, cfg, data, err); errStatus != nil {
			return errStatus
		}
		return err
	}

	if deployOptions.Manifest.Deploy == nil {
		return nil
	}

	if err := buildImages(ctx, dc.Builder, deployOptions); err != nil {
		if errStatus := dc.CfgMapHandler.updateConfigMap(ctx, cfg, data, err); errStatus != nil {
			return errStatus
		}
		return err
	}

	if err := dc.recreateFailedPods(ctx, deployOptions.Name); err != nil {
		oktetoLog.Infof("failed to recreate failed pods: %s", err.Error())
	}

	deployer, err := dc.GetDeployer(ctx, deployOptions, dc.Builder, dc.CfgMapHandler, dc.K8sClientProvider, NewKubeConfig(), model.GetAvailablePort)
	if err != nil {
		return err
	}

	err = deployer.deploy(ctx, deployOptions)
	if err != nil {
		if err == oktetoErrors.ErrIntSig {
			return nil
		}
		err = oktetoErrors.UserError{E: err}
		data.Status = pipeline.ErrorStatus
	} else {
		oktetoLog.SetStage("")
		hasDeployed, err := pipeline.HasDeployedSomething(ctx, deployOptions.Name, deployOptions.Manifest.Namespace, c)
		if err != nil {
			return err
		}
		if hasDeployed {
			if deployOptions.Wait {
				if err := dc.DeployWaiter.wait(ctx, deployOptions); err != nil {
					return err
				}
			}
			if !utils.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
				eg, err := dc.EndpointGetter()
				if err != nil {
					oktetoLog.Infof("could not create endpoint getter: %s", err)
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

	if errStatus := dc.CfgMapHandler.updateConfigMap(ctx, cfg, data, err); errStatus != nil {
		return errStatus
	}

	return err
}

func buildImages(ctx context.Context, builder builderInterface, deployOptions *Options) error {
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
		if errBuild := builder.Build(ctx, buildOptions); errBuild != nil {
			return errBuild
		}
	} else {
		servicesToBuild, err := builder.GetServicesToBuild(ctx, deployOptions.Manifest, setToSlice(servicesToBuildSet))
		if err != nil {
			return err
		}

		if len(servicesToBuild) != 0 {
			buildOptions := &types.BuildOptions{
				EnableStages: true,
				Manifest:     deployOptions.Manifest,
				CommandArgs:  servicesToBuild,
			}

			if errBuild := builder.Build(ctx, buildOptions); errBuild != nil {
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

func shouldRunInRemote(opts *Options) bool {
	// already in remote so we need to deploy locally
	if utils.LoadBoolean(constants.OktetoDeployRemote) {
		return false
	}

	// --remote flag enabled from command line
	if opts.RunInRemote {
		return true
	}

	// remote option set in the manifest via a remote deployer image or the remote option enabled
	if opts.Manifest != nil && opts.Manifest.Deploy != nil {
		if opts.Manifest.Deploy.Image != "" || opts.Manifest.Deploy.Remote {
			return true
		}
	}

	if utils.LoadBoolean(constants.OktetoForceRemote) {
		return true
	}

	return false

}

// GetDeployer returns a remote or a local deployer
// k8sProvider, kubeconfig and portGetter should not be nil values
func GetDeployer(ctx context.Context,
	opts *Options,
	builder builderInterface,
	cmapHandler configMapHandler,
	k8sProvider okteto.K8sClientProvider,
	kubeconfig kubeConfigHandler,
	portGetter portGetterFunc,
) (deployerInterface, error) {
	if shouldRunInRemote(opts) {
		// run remote
		oktetoLog.Info("Deploying remotely...")
		return newRemoteDeployer(builder), nil
	}

	// run local
	oktetoLog.Info("Deploying locally...")

	deployer, err := newLocalDeployer(ctx, opts, cmapHandler, k8sProvider, kubeconfig, portGetter)
	if err != nil {
		eWrapped := fmt.Errorf("could not initialize local deploy command: %w", err)
		if uError, ok := err.(oktetoErrors.UserError); ok {
			uError.E = eWrapped
			return nil, uError
		}
		return nil, eWrapped
	}
	return deployer, nil
}

func isRemoteDeployer(runInRemoteFlag bool, deployImage string) bool {
	// isDeployRemote represents whether the process is coming from a remote deploy
	// if true it should get the local deployer
	isDeployRemote := utils.LoadBoolean(constants.OktetoDeployRemote)

	// remote deployment should be done when flag RunInRemote is active OR deploy.image is fulfilled
	return !isDeployRemote && (runInRemoteFlag || deployImage != "")
}

// deployDependencies deploy the dependencies in the manifest
func (dc *DeployCommand) deployDependencies(ctx context.Context, deployOptions *Options) error {
	if len(deployOptions.Manifest.Dependencies) > 0 && !okteto.Context().IsOkteto {
		return errDepenNotAvailableInVanilla
	}

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

		err := dep.ExpandVars(deployOptions.Variables)
		if err != nil {
			return fmt.Errorf("could not expand variables in dependencies: %w", err)
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

		if err := dc.PipelineCMD.ExecuteDeployPipeline(ctx, pipOpts); err != nil {
			return err
		}
	}
	oktetoLog.SetStage("")
	return nil
}

func (dc *DeployCommand) recreateFailedPods(ctx context.Context, name string) error {
	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return fmt.Errorf("could not get kubernetes client: %s", err)
	}

	pods, err := c.CoreV1().Pods(okteto.Context().Namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", model.DeployedByLabel, format.ResourceK8sMetaString(name))})
	if err != nil {
		return fmt.Errorf("could not list pods: %s", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" {
			err := c.CoreV1().Pods(okteto.Context().Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("could not delete pod %s: %s", pod.Name, err)
			}
		}
	}
	return nil
}

func (dc *DeployCommand) trackDeploy(manifest *model.Manifest, runInRemoteFlag bool, startTime time.Time, err error) {
	deployType := "custom"
	hasDependencySection := false
	hasBuildSection := false
	isRunningOnRemoteDeployer := false
	if manifest != nil {
		if manifest.IsV2 &&
			manifest.Deploy != nil {
			isRunningOnRemoteDeployer = isRemoteDeployer(runInRemoteFlag, manifest.Deploy.Image)
			if manifest.Deploy.ComposeSection != nil &&
				manifest.Deploy.ComposeSection.ComposesInfo != nil {
				deployType = "compose"
			}
		}

		hasDependencySection = manifest.HasDependencies()
		hasBuildSection = manifest.HasBuildSection()
	}

	dc.AnalyticsTracker.TrackDeploy(analytics.DeployMetadata{
		Success:                err == nil,
		IsOktetoRepo:           utils.IsOktetoRepo(),
		Duration:               time.Since(startTime),
		PipelineType:           dc.PipelineType,
		DeployType:             deployType,
		IsPreview:              os.Getenv(model.OktetoCurrentDeployBelongsToPreview) == "true",
		HasDependenciesSection: hasDependencySection,
		HasBuildSection:        hasBuildSection,
		IsRemote:               isRunningOnRemoteDeployer,
	})
}

func checkOktetoManifestPathFlag(options *Options, fs afero.Fs) error {
	if options.ManifestPath != "" {
		// if path is absolute, its transformed from root path to a rel path
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

		// check whether the manifest file provided by -f exists or not
		if _, err := fs.Stat(options.ManifestPath); err != nil {
			return fmt.Errorf("%s file doesn't exist", options.ManifestPath)
		}
	}
	return nil
}
