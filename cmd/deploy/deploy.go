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
	"path/filepath"
	"strings"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCmd "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/httproutes"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/repository"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/validator"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
	dependencyEnvVarPrefix = "OKTETO_DEPENDENCY_"
	deployComposePhaseName = "compose"
)

var (
	errDepenNotAvailableInVanilla = errors.New("dependency deployment is only supported in contexts with Okteto installed")
)

// Options represents options for deploy command
type Options struct {
	Manifest *model.Manifest
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the path to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath          string
	Name                  string
	Namespace             string
	K8sContext            string
	Variables             []string
	StackServicesToDeploy []string
	Timeout               time.Duration
	NoBuild               bool
	Dependencies          bool
	RunWithoutBash        bool
	RunInRemote           bool
	RunInRemoteSet        bool
	Wait                  bool
	ShowCTA               bool
}

type builderInterface interface {
	Build(ctx context.Context, options *types.BuildOptions) error
	GetServicesToBuildDuringExecution(ctx context.Context, manifest *model.Manifest, svcsToDeploy []string) ([]string, error)
	GetBuildEnvVars() map[string]string
}

type getDeployerFunc func(
	context.Context, *Options,
	buildEnvVarsGetter,
	ConfigMapHandler,
	okteto.K8sClientProviderWithLogger,
	*io.Controller,
	*io.K8sLogger,
	dependencyEnvVarsGetter,
	buildCmd.BuildkitConnector,
) (Deployer, error)

type cleanUpFunc func(context.Context, error)

// Command defines the config for deploying an app
type Command struct {
	GetManifest          func(path string, fs afero.Fs) (*model.Manifest, error)
	K8sClientProvider    okteto.K8sClientProviderWithLogger
	Builder              builderInterface
	RemoteConnector      buildCmd.BuildkitConnector
	GetDeployer          getDeployerFunc
	EndpointGetter       func(k8sLogger *io.K8sLogger) (EndpointGetter, error)
	DeployWaiter         Waiter
	CfgMapHandler        ConfigMapHandler
	Fs                   afero.Fs
	PipelineCMD          pipelineCMD.DeployerInterface
	AnalyticsTracker     AnalyticsTrackerInterface
	IoCtrl               *io.Controller
	K8sLogger            *io.K8sLogger
	InsightsTracker      buildDeployTrackerInterface
	DivertDeployerGetter getDivertDeployer

	PipelineType model.Archetype
	// onCleanUp is a list of functions to be executed when the execution is interrupted. This is a hack
	// to be able to call to deployer's cleanUp function as the deployer is gotten at runtime.
	// This can probably be improved using context cancellation
	onCleanUp []cleanUpFunc

	IsRemote           bool
	RunningInInstaller bool
}

type AnalyticsTrackerInterface interface {
	TrackDeploy(dm analytics.DeployMetadata)
	buildTrackerInterface
}

type buildTrackerInterface interface {
	TrackImageBuild(context.Context, *analytics.ImageBuildMetadata)
}

type deployTrackerInterface interface {
	TrackDeploy(ctx context.Context, name, namespace string, success bool)
}

type buildDeployTrackerInterface interface {
	buildTrackerInterface
	deployTrackerInterface
}

// Deployer defines the operations to deploy the custom commands, divert and external resources
// defined in an Okteto manifest
type Deployer interface {
	Deploy(context.Context, *Options) error
	CleanUp(ctx context.Context, err error)
}

// DivertDeployer defines the operations to deploy the divert section of an Okteto manifest
type DivertDeployer interface {
	Deploy(ctx context.Context) error
}

type getDivertDeployer func(divert *model.DivertDeploy, name, namespace string, c kubernetes.Interface, ioCtrl *io.Controller) (DivertDeployer, error)

func newDivertDeployer(d *model.DivertDeploy, name, namespace string, c kubernetes.Interface, ioCtrl *io.Controller) (DivertDeployer, error) {
	return divert.New(d, name, namespace, c, ioCtrl)
}

// Deploy deploys the okteto manifest
func Deploy(ctx context.Context, at AnalyticsTrackerInterface, insightsTracker buildDeployTrackerInterface, ioCtrl *io.Controller, k8sLogger *io.K8sLogger) *cobra.Command {
	options := &Options{}
	cmd := &cobra.Command{
		Use:   "deploy",
		Short: "Deploy your Development Environment by running the commands specified in the 'deploy' section of your Okteto Manifest",
		Example: `# Execute okteto deploy
$ okteto deploy

# Execute okteto deploy in remote
$ okteto deploy --remote


# Execute okteto deploy skipping the build
$ okteto deploy --no-build=true`,
		Args: utils.NoArgsAccepted(""),
		RunE: func(cmd *cobra.Command, _ []string) error {
			// check if remote flag is used by the user
			options.RunInRemoteSet = cmd.Flag("remote").Changed
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

			err := checkOktetoManifestPathFlag(options, afero.NewOsFs())
			if err != nil {
				return err
			}

			if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Show: true, Namespace: options.Namespace, Context: options.K8sContext}); err != nil {
				return err
			}

			if !okteto.IsOkteto() {
				return oktetoErrors.ErrContextIsNotOktetoCluster
			}

			create, err := utils.ShouldCreateNamespace(ctx, okteto.GetContext().Namespace)
			if err != nil {
				return err
			}
			if create {
				nsCmd, err := namespace.NewCommand(ioCtrl)
				if err != nil {
					return err
				}
				if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.GetContext().Namespace}); err != nil {
					return err
				}
			}

			options.ShowCTA = oktetoLog.IsInteractive()

			k8sClientProvider := okteto.NewK8sClientProviderWithLogger(k8sLogger)
			pc, err := pipelineCMD.NewCommand()
			if err != nil {
				return fmt.Errorf("could not create pipeline command: %w", err)
			}

			onBuildFinish := []buildv2.OnBuildFinish{
				at.TrackImageBuild,
				insightsTracker.TrackImageBuild,
			}

			okCtx := &okteto.ContextStateless{Store: okteto.GetContextStore()}
			conn := buildCmd.GetBuildkitConnector(okCtx, ioCtrl)

			c := &Command{
				GetManifest: model.GetManifestV2,

				K8sClientProvider:    k8sClientProvider,
				GetDeployer:          GetDeployer,
				Builder:              buildv2.NewBuilderFromScratch(ioCtrl, onBuildFinish, conn),
				RemoteConnector:      conn,
				DeployWaiter:         NewDeployWaiter(k8sClientProvider, k8sLogger),
				EndpointGetter:       NewEndpointGetter,
				IsRemote:             env.LoadBoolean(constants.OktetoDeployRemote),
				CfgMapHandler:        NewConfigmapHandler(k8sClientProvider, k8sLogger),
				Fs:                   afero.NewOsFs(),
				PipelineCMD:          pc,
				RunningInInstaller:   config.RunningInInstaller(),
				AnalyticsTracker:     at,
				IoCtrl:               ioCtrl,
				K8sLogger:            k8sLogger,
				DivertDeployerGetter: newDivertDeployer,

				onCleanUp:       []cleanUpFunc{},
				InsightsTracker: insightsTracker,
			}
			startTime := time.Now()

			stop := make(chan os.Signal, 1)
			signal.Notify(stop, os.Interrupt)
			exit := make(chan error, 1)

			go func() {
				if options.Namespace == "" {
					options.Namespace = okteto.GetContext().Namespace
				}
				err := c.Run(ctx, options)
				c.InsightsTracker.TrackDeploy(ctx, options.Name, options.Namespace, err == nil)
				c.TrackDeploy(options.Manifest, options.RunInRemote, startTime, err)
				exit <- err
			}()

			select {
			case <-stop:
				oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
				oktetoLog.Spinner("Shutting down...")
				oktetoLog.StartSpinner()
				defer oktetoLog.StopSpinner()

				c.cleanUp(ctx, oktetoErrors.ErrIntSig)
				return oktetoErrors.ErrIntSig
			case err := <-exit:
				return err
			}
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "the name of the Development Environment")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "the path to the Okteto Manifest")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrite the current Okteto Namespace")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "overwrite the current Okteto Context")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable for the deploy commands (can be set more than once)")
	cmd.Flags().BoolVarP(&options.NoBuild, "no-build", "", false, "skips the re-build of images")
	cmd.Flags().BoolVarP(&options.Dependencies, "dependencies", "", false, "force deployment of repositories in the 'dependencies' section")
	cmd.Flags().BoolVarP(&options.RunWithoutBash, "no-bash", "", false, "execute the command using the container's default shell instead of bash")
	cmd.Flags().BoolVarP(&options.RunInRemote, "remote", "", false, "run the deploy commands using Remote Execution")

	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the deployment finishes and pods are healthy")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", getDefaultTimeout(), "when using `wait`, the maximum time to wait for the resources of the deployment to be healthy")

	return cmd
}

// calculateManifestPathToBeStored calculates the manifest path that has to be stored in the config map for UI operations.
// It calculates the absolute path from the received path and then, it gets the relative path the top level git dir (repo root)
func (dc *Command) calculateManifestPathToBeStored(topLevelGitDir, manifestPath string) string {
	absoluteManifestPath, err := filepath.Abs(manifestPath)
	if err != nil {
		dc.IoCtrl.Logger().Debugf("failed to get absolute path for manifest path %q: %s", manifestPath, err)
		return ""
	}
	manifestPathForConfigMap, err := filepath.Rel(topLevelGitDir, absoluteManifestPath)
	if err != nil {
		dc.IoCtrl.Logger().Infof("failed to get relative path for manifest path %q from the repository dir %q: %s", absoluteManifestPath, topLevelGitDir, err)
		return ""
	}

	// If the relative path to the repository contains "..", it means the manifest path is not within the
	// repository, so it should not be stored in the config map
	if strings.Contains(manifestPathForConfigMap, "..") {
		return ""
	}

	return manifestPathForConfigMap
}

// Run runs the deploy sequence
func (dc *Command) Run(ctx context.Context, deployOptions *Options) error {
	oktetoLog.SetStage("Load manifest")
	manifest, err := dc.GetManifest(deployOptions.ManifestPath, dc.Fs)
	if err != nil {
		return err
	}
	deployOptions.Manifest = manifest
	oktetoLog.Debug("found okteto manifest")
	dc.PipelineType = deployOptions.Manifest.Type

	if deployOptions.Manifest.Deploy == nil && !deployOptions.Manifest.HasDependencies() {
		return oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands
	}

	// We need to create a client that doesn't go through the proxy to create
	// the configmap without the deployedByLabel
	c, _, err := dc.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dc.K8sLogger)
	if err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	topLevelGitDir, err := repository.FindTopLevelGitDir(cwd)
	if err != nil {
		oktetoLog.Warning("Repository not detected: the env vars '%s' and '%s' might not be available.\n    For more information, check out: https://www.okteto.com/docs/core/okteto-variables/#default-environment-variables", constants.OktetoGitBranchEnvVar, constants.OktetoGitCommitEnvVar)
	}

	if topLevelGitDir != "" {
		dc.IoCtrl.Logger().Debugf("repository detected at %s", topLevelGitDir)
		dc.addEnvVars(topLevelGitDir)
	} else {
		dc.addEnvVars(cwd)
	}

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c, dc.K8sLogger); err != nil {
		return err
	}

	if dc.RunningInInstaller {
		currentVars, err := dc.CfgMapHandler.GetConfigmapVariablesEncoded(ctx, deployOptions.Name, deployOptions.Namespace)
		if err != nil {
			return err
		}

		// when running in remote or installer variables should be retrieved from the saved value at configmap
		deployOptions.Variables = []string{}
		for _, v := range types.DecodeStringToDeployVariable(currentVars) {
			deployOptions.Variables = append(deployOptions.Variables, fmt.Sprintf("%s=%s", v.Name, v.Value))
		}
	}

	// This is the manifest path to be stored in the config map. It should be relative to the repository root, so next operations
	// triggered from the UI would take the correct manifest. So, it is calculated from the topLevelGitDir and the absolute path
	// of the manifest file.
	// Having the following structure:
	// root-repo
	// |- dirA
	//   |- dirB
	//    |- okteto.yml
	// If the command executed is okteto deploy from "dirB", we should store the manifest path as "dirA/dirB/okteto.yml"
	// NOTE: This is only stored if ManifestPathFlag (-f options) is passed
	manifestPathForConfigMap := ""
	if topLevelGitDir != "" && deployOptions.ManifestPathFlag != "" {
		// If deployOptions.ManifestPath is set, means that we have changed the working directory to the one storing the manifest, so we need to take the absolute path from ManifestPath
		// as the original deployOptions.ManifestPathFlag wouldn't build the right path if we get the absolute one
		if deployOptions.ManifestPath != "" {
			manifestPathForConfigMap = dc.calculateManifestPathToBeStored(topLevelGitDir, deployOptions.ManifestPath)
		} else {
			manifestPathForConfigMap = dc.calculateManifestPathToBeStored(topLevelGitDir, deployOptions.ManifestPathFlag)
		}
	}

	dc.IoCtrl.Logger().Debugf("manifest path to store in metadata: %q", manifestPathForConfigMap)
	data := &pipeline.CfgData{
		Name:       deployOptions.Name,
		Namespace:  deployOptions.Namespace,
		Repository: os.Getenv(model.GithubRepositoryEnvVar),
		Branch:     os.Getenv(constants.OktetoGitBranchEnvVar),
		Filename:   manifestPathForConfigMap,
		Status:     pipeline.ProgressingStatus,
		Manifest:   deployOptions.Manifest.Manifest,
		Icon:       deployOptions.Manifest.Icon,
		Variables:  deployOptions.Variables,
	}

	if deployOptions.Manifest.Type == model.StackType && deployOptions.Manifest.Deploy != nil {
		data.Manifest = deployOptions.Manifest.Deploy.ComposeSection.Stack.Manifest
	}

	cfg, err := dc.CfgMapHandler.TranslateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	os.Setenv(constants.OktetoNameEnvVar, deployOptions.Name)

	if err := dc.deployDependencies(ctx, deployOptions); err != nil {
		if errStatus := dc.CfgMapHandler.UpdateConfigMap(ctx, cfg, data, err); errStatus != nil {
			return errStatus
		}
		return err
	}

	if deployOptions.Manifest.Deploy == nil {
		return nil
	}

	if err := buildImages(ctx, dc.Builder, dc.CfgMapHandler, deployOptions); err != nil {
		if errStatus := dc.CfgMapHandler.UpdateConfigMap(ctx, cfg, data, err); errStatus != nil {
			return errStatus
		}
		return err
	}

	if err := dc.recreateFailedPods(ctx, deployOptions.Name); err != nil {
		oktetoLog.Infof("failed to recreate failed pods: %s", err.Error())
	}

	oktetoLog.EnableMasking()
	err = dc.deploy(ctx, deployOptions, cwd, c)
	oktetoLog.DisableMasking()
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

	if err != nil {
		if errors.Is(err, oktetoErrors.ErrIntSig) {
			return nil
		}
		// transform internal errors to user errors
		if !errors.As(err, &oktetoErrors.UserError{}) {
			err = oktetoErrors.UserError{E: err}
		}
		data.Status = pipeline.ErrorStatus
	} else {
		// This has to be set only when the command succeeds for the case in which the deploy is executed within an
		// installer. When running in installer, if the command fails, and we set an empty stage, we would display
		// a stage with "Internal Server Error" duplicating the message we already display on error. For that reason,
		// we should not set empty stage on error.
		oktetoLog.SetStage("")
		hasDeployed, err := pipeline.HasDeployedSomething(ctx, deployOptions.Name, okteto.GetContext().Namespace, c)
		if err != nil {
			return err
		}
		if hasDeployed {
			if deployOptions.Wait {
				if err := dc.DeployWaiter.wait(ctx, deployOptions); err != nil {
					if err := dc.CfgMapHandler.UpdateConfigMap(ctx, cfg, data, err); err != nil {
						oktetoLog.Infof("could not update configmap with timeout error: %s", err)
						return err
					}
					return err
				}
			}
			if !env.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
				eg, err := dc.EndpointGetter(dc.K8sLogger)
				if err != nil {
					oktetoLog.Infof("could not create endpoint getter: %s", err)
				}
				if err := eg.showEndpoints(ctx, &EndpointsOptions{Name: deployOptions.Name, Namespace: okteto.GetContext().Namespace}); err != nil {
					oktetoLog.Infof("could not retrieve endpoints: %s", err)
				}
			}
			if deployOptions.ShowCTA {
				oktetoLog.Success(succesfullyDeployedmsg, deployOptions.Name)
			}
			pipeline.AddDevAnnotations(ctx, deployOptions.Manifest, c)
		}
		data.Status = pipeline.DeployedStatus
	}

	if errStatus := dc.CfgMapHandler.UpdateConfigMap(ctx, cfg, data, err); errStatus != nil {
		return errStatus
	}

	return err
}

func (dc *Command) deploy(ctx context.Context, deployOptions *Options, cwd string, c kubernetes.Interface) error {
	// If the command is configured to execute things remotely (--remote, deploy.image or deploy.remote) it should be executed in the remote. If not, it should be executed locally
	deployer, err := dc.GetDeployer(
		ctx,
		deployOptions,
		dc.Builder.GetBuildEnvVars,
		dc.CfgMapHandler,
		dc.K8sClientProvider,
		dc.IoCtrl,
		dc.K8sLogger,
		GetDependencyEnvVars,
		dc.RemoteConnector,
	)
	if err != nil {
		return err
	}

	// Once we have the deployer, we add the clean up function to the list of clean up functions to be executed to clean all the resources
	dc.onCleanUp = append(dc.onCleanUp, deployer.CleanUp)

	err = deployer.Deploy(ctx, deployOptions)
	if err != nil {
		return err
	}

	// Compose and endpoints are always deployed locally as part of the main command execution even when the flag --remote is set

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c, dc.K8sLogger); err != nil {
		return err
	}

	// deploy compose if any
	if deployOptions.Manifest.Deploy.ComposeSection != nil {
		stage := "Deploying compose"
		oktetoLog.SetStage(stage)
		oktetoLog.Information("Running stage '%s'", stage)
		startTime := time.Now()
		err := dc.deployStack(ctx, deployOptions)
		elapsedTime := time.Since(startTime)
		if addPhaseErr := dc.CfgMapHandler.AddPhaseDuration(ctx, deployOptions.Name, okteto.GetContext().Namespace, deployComposePhaseName, elapsedTime); addPhaseErr != nil {
			oktetoLog.Info("error adding phase to configmap: %s", err)
		}
		if err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying compose: %s", err.Error())
			return err
		}
	}

	// deploy endpoints if any
	if deployOptions.Manifest.Deploy.Endpoints != nil {
		stage := "Endpoints configuration"
		oktetoLog.SetStage(stage)
		oktetoLog.Information("Running stage '%s'", stage)
		if err := dc.deployEndpoints(ctx, deployOptions); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error generating endpoints: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	if deployOptions.Manifest.Deploy.Divert != nil && deployOptions.Manifest.Deploy.Divert.Namespace != okteto.GetContext().Namespace {
		stage := "Deploy Divert"
		oktetoLog.SetStage(stage)
		oktetoLog.Information("Running stage '%s'", stage)
		driver, err := dc.DivertDeployerGetter(deployOptions.Manifest.Deploy.Divert, deployOptions.Name, deployOptions.Namespace, c, dc.IoCtrl)
		if err != nil {
			return err
		}
		if err := driver.Deploy(ctx); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error creating divert: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	return nil
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

// ShouldRunInRemote determines if the deploy command should run in remote
// default behavior is set by cluster config, but can be overridden by the user using the flag --remote or the manifest deploy.remote
func ShouldRunInRemote(opts *Options) bool {
	if env.LoadBoolean(constants.OktetoForceRemote) {
		// the user forces --remote=false
		if opts.RunInRemoteSet && !opts.RunInRemote {
			return false
		}

		// the user forces manifest.deploy.remote=false
		if opts.Manifest != nil && opts.Manifest.Deploy != nil {
			if opts.Manifest.Deploy.Remote != nil && !*opts.Manifest.Deploy.Remote {
				return false
			}
		}
		return true
	}

	// remote option set in the command line
	if opts.RunInRemote {
		return true
	}

	// remote option set in the manifest via the remote option enabled
	if opts.Manifest != nil && opts.Manifest.Deploy != nil {
		if opts.Manifest.Deploy.Image != "" {
			return true
		}
		if opts.Manifest.Deploy.Remote != nil && *opts.Manifest.Deploy.Remote {
			return true
		}
	}

	if opts.Manifest != nil && opts.Manifest.Deploy != nil && (len(opts.Manifest.Deploy.Commands) > 0 || opts.Manifest.Deploy.Divert != nil || len(opts.Manifest.External) > 0) {
		oktetoLog.Information("Okteto recommends that you enable remote execution for your deploy commands.\n    More information available here: https://www.okteto.com/docs/core/remote-execution")
	}
	return false
}

// GetDeployer returns a remote or a local deployer
// k8sProvider, kubeconfig and portGetter should not be nil values
func GetDeployer(ctx context.Context,
	opts *Options,
	buildEnvVarsGetter buildEnvVarsGetter,
	cmapHandler ConfigMapHandler,
	k8sProvider okteto.K8sClientProviderWithLogger,
	ioCtrl *io.Controller,
	k8Logger *io.K8sLogger,
	dependencyEnvVarsGetter dependencyEnvVarsGetter,
	conn buildCmd.BuildkitConnector,
) (Deployer, error) {
	if ShouldRunInRemote(opts) {
		oktetoLog.Info("Deploying remotely...")
		return newRemoteDeployer(buildEnvVarsGetter, ioCtrl, dependencyEnvVarsGetter, conn), nil
	}

	var execDir string
	if opts.Manifest.Deploy != nil {
		execDir = opts.Manifest.Deploy.Context
	}

	oktetoLog.Info("Deploying locally...")
	// In case the command has to run locally, we need the "local" runner
	runner, err := deployable.NewDeployRunnerForLocal(
		ctx,
		opts.Name,
		opts.RunWithoutBash,
		opts.ManifestPathFlag,
		execDir,
		cmapHandler,
		k8sProvider,
		model.GetAvailablePort,
		k8Logger,
		ioCtrl)
	if err != nil {
		eWrapped := fmt.Errorf("could not initialize local deploy command: %w", err)
		if uError, ok := err.(oktetoErrors.UserError); ok {
			uError.E = eWrapped
			return nil, uError
		}
		return nil, eWrapped
	}

	return newLocalDeployer(runner), nil
}

// isRemoteDeployer should be considered remote when flag RunInRemote is active OR deploy.image is fulfilled OR remote flag in manifest is set
func isRemoteDeployer(runInRemoteFlag bool, deployImage string, manifestRemoteFlag bool) bool {
	return runInRemoteFlag || deployImage != "" || manifestRemoteFlag
}

// deployDependencies deploy the dependencies in the manifest
func (dc *Command) deployDependencies(ctx context.Context, deployOptions *Options) error {
	if len(deployOptions.Manifest.Dependencies) > 0 && !okteto.GetContext().IsOkteto {
		return errDepenNotAvailableInVanilla
	}

	for depName, dep := range deployOptions.Manifest.Dependencies {
		oktetoLog.Information("Deploying dependency  '%s'", depName)
		oktetoLog.SetStage(fmt.Sprintf("Deploying dependency %s", depName))
		if err := validator.CheckReservedVarName(dep.Variables); err != nil {
			return err
		}

		dep.Variables = append(dep.Variables, env.Var{
			Name:  "OKTETO_ORIGIN",
			Value: "okteto-deploy",
		})

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
			Namespace:    okteto.GetContext().Namespace,
			IsDependency: true,
		}

		if err := dc.PipelineCMD.ExecuteDeployPipeline(ctx, pipOpts); err != nil {
			return err
		}
		if dep.Wait {
			depBuildEnvs, err := dc.CfgMapHandler.GetDependencyBuildEnvVars(ctx, depName, deployOptions.Namespace)
			if err != nil {
				return fmt.Errorf("could not get dependency build env vars: %w", err)
			}
			for key, val := range depBuildEnvs {
				if err := os.Setenv(key, val); err != nil {
					return fmt.Errorf("could not set dependency env var %s: %w", key, err)
				}
			}
		}
		oktetoLog.SetStage("")
	}
	oktetoLog.SetStage("")
	return nil
}

func (dc *Command) recreateFailedPods(ctx context.Context, name string) error {
	c, _, err := dc.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dc.K8sLogger)
	if err != nil {
		return fmt.Errorf("could not get kubernetes client: %w", err)
	}

	pods, err := c.CoreV1().Pods(okteto.GetContext().Namespace).List(ctx, metav1.ListOptions{LabelSelector: fmt.Sprintf("%s=%s", model.DeployedByLabel, format.ResourceK8sMetaString(name))})
	if err != nil {
		return fmt.Errorf("could not list pods: %w", err)
	}
	for _, pod := range pods.Items {
		if pod.Status.Phase == "Failed" {
			err := c.CoreV1().Pods(okteto.GetContext().Namespace).Delete(ctx, pod.Name, metav1.DeleteOptions{})
			if err != nil {
				return fmt.Errorf("could not delete pod %s: %w", pod.Name, err)
			}
		}
	}
	return nil
}

func (dc *Command) TrackDeploy(manifest *model.Manifest, runInRemoteFlag bool, startTime time.Time, err error) {
	deployType := "custom"
	hasDependencySection := false
	hasBuildSection := false
	isRunningOnRemoteDeployer := false
	if manifest != nil {
		if manifest.Deploy != nil {
			isRunningOnRemoteDeployer = isRemoteDeployer(runInRemoteFlag, manifest.Deploy.Image, manifest.Deploy.Remote != nil && *manifest.Deploy.Remote)
			if manifest.Deploy.ComposeSection != nil &&
				manifest.Deploy.ComposeSection.ComposesInfo != nil {
				deployType = "compose"
			}
		}

		hasDependencySection = manifest.HasDependencies()
		hasBuildSection = manifest.HasBuildSection()
	}

	// We keep DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar for backward compatibility in case an old version of the backend
	// is being used
	isPreview := os.Getenv(model.DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar) == "true" ||
		os.Getenv(constants.OktetoIsPreviewEnvVar) == "true"
	dc.AnalyticsTracker.TrackDeploy(analytics.DeployMetadata{
		Success:                err == nil,
		IsOktetoRepo:           utils.IsOktetoRepo(),
		Duration:               time.Since(startTime),
		PipelineType:           dc.PipelineType,
		DeployType:             deployType,
		IsPreview:              isPreview,
		HasDependenciesSection: hasDependencySection,
		HasBuildSection:        hasBuildSection,
		IsRemote:               isRunningOnRemoteDeployer,
	})
}

func (dc *Command) cleanUp(ctx context.Context, err error) {
	for _, cleanUp := range dc.onCleanUp {
		cleanUp(ctx, err)
	}
}

// deployStack deploys the compose defined in the Okteto manifest
func (dc *Command) deployStack(ctx context.Context, opts *Options) error {
	composeSectionInfo := opts.Manifest.Deploy.ComposeSection
	composeSectionInfo.Stack.Namespace = okteto.GetContext().Namespace

	var composeFiles []string
	for _, composeInfo := range composeSectionInfo.ComposesInfo {
		composeFiles = append(composeFiles, composeInfo.File)
	}
	stackOpts := &stack.DeployOptions{
		StackPaths:       composeFiles,
		ForceBuild:       false,
		Wait:             opts.Wait,
		Timeout:          opts.Timeout,
		ServicesToDeploy: opts.StackServicesToDeploy,
		InsidePipeline:   true,
	}

	c, cfg, err := dc.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dc.K8sLogger)
	if err != nil {
		return err
	}

	divertDriver := divert.NewNoop()
	if opts.Manifest.Deploy.Divert != nil {
		divertDriver, err = divert.New(opts.Manifest.Deploy.Divert, opts.Manifest.Name, okteto.GetContext().Namespace, c, dc.IoCtrl)
		if err != nil {
			return err
		}
	}

	// Determine which endpoint deployer to use (Ingress or HTTPRoute)
	useHTTPRoute, clusterMetadata, err := stack.ShouldUseHTTPRoute()
	if err != nil {
		return err
	}

	var endpointDeployer stack.EndpointDeployer
	if useHTTPRoute {
		// Create HTTPRoute deployer
		httpRouteClient, err := httproutes.NewHTTPRouteClient(cfg)
		if err != nil {
			return fmt.Errorf("error creating httproute client: %w", err)
		}
		endpointDeployer = stack.NewHTTPRouteDeployer(httpRouteClient, composeSectionInfo.Stack.Name, composeSectionInfo.Stack.Namespace, clusterMetadata)
	} else {
		// Create Ingress deployer
		ingressClient, err := ingresses.GetClient(c)
		if err != nil {
			return fmt.Errorf("error getting ingress client: %w", err)
		}
		oktetoLog.Infof("Using Ingress for endpoints")
		endpointDeployer = stack.NewIngressDeployer(ingressClient, composeSectionInfo.Stack.Name, composeSectionInfo.Stack.Namespace)
	}

	sd := stack.Stack{
		K8sClient:        c,
		Config:           cfg,
		AnalyticsTracker: dc.AnalyticsTracker,
		Insights:         dc.InsightsTracker,
		IoCtrl:           dc.IoCtrl,
		Divert:           divertDriver,
		EndpointDeployer: endpointDeployer,
	}
	return sd.RunDeploy(ctx, composeSectionInfo.Stack, stackOpts)
}

// deployEndpoints deploys the endpoints defined in the Okteto manifest
func (dc *Command) deployEndpoints(ctx context.Context, opts *Options) error {

	c, _, err := dc.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, dc.K8sLogger)
	if err != nil {
		return err
	}

	iClient, err := ingresses.GetClient(c)
	if err != nil {
		return fmt.Errorf("error getting ingress client: %w", err)
	}

	translateOptions := &ingresses.TranslateOptions{
		Namespace: okteto.GetContext().Namespace,
		Name:      format.ResourceK8sMetaString(opts.Manifest.Name),
	}

	for name, endpoint := range opts.Manifest.Deploy.Endpoints {
		ingress := ingresses.Translate(name, endpoint, translateOptions)
		if err := iClient.Deploy(ctx, ingress); err != nil {
			return err
		}
	}

	return nil
}

// GetDependencyEnvVars This function gets the variables defined by the dependencies (OKTETO_DEPENDENCY_XXXX)
// deployed before the deploy phase from the environment. This function is here as the command is the one in charge
// of deploying dependencies and trigger the rest of the deploy phase.
func GetDependencyEnvVars(environGetter environGetter) map[string]string {
	varsParts := 2
	result := map[string]string{}
	for _, e := range environGetter() {
		pair := strings.SplitN(e, "=", varsParts)
		if len(pair) != varsParts {
			// If a variables doesn't have left and right side we just skip it
			continue
		}

		if strings.HasPrefix(pair[0], dependencyEnvVarPrefix) {
			result[pair[0]] = pair[1]
		}
	}

	return result
}

func checkOktetoManifestPathFlag(options *Options, fs afero.Fs) error {
	if options.ManifestPath == "" {
		return nil
	}

	// if path is absolute, its transformed from root path to a rel path
	initialCWD, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}
	manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, options.ManifestPath)
	if err != nil {
		return err
	}

	if err := validator.FileArgumentIsNotDir(fs, manifestPathFlag); err != nil {
		return err
	}

	// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
	options.ManifestPathFlag = manifestPathFlag

	// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
	uptManifestPath, err := filesystem.UpdateCWDtoManifestPath(options.ManifestPath)
	if err != nil {
		return err
	}
	options.ManifestPath = uptManifestPath

	return nil
}
