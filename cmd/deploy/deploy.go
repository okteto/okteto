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

package deploy

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	stackCMD "github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/cobra"
	giturls "github.com/whilp/git-urls"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	headerUpgrade          = "Upgrade"
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
)

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s-%d"

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
	servicesToDeploy []string

	Repository string
	Branch     string
	Wait       bool
	Timeout    time.Duration

	ShowCTA bool
}

type kubeConfigHandler interface {
	Read() (*rest.Config, error)
	Modify(port int, sessionToken, destKubeconfigFile string) error
}

type proxyInterface interface {
	Start()
	Shutdown(ctx context.Context) error
	GetPort() int
	GetToken() string
	SetName(name string)
	SetDivert(divertedNamespace string)
}

// DeployCommand defines the config for deploying an app
type DeployCommand struct {
	GetManifest func(path string) (*model.Manifest, error)

	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	Executor           executor.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider
	Builder            *buildv2.OktetoBuilder

	PipelineType model.Archetype
}

//Deploy deploys the okteto manifest
func Deploy(ctx context.Context) *cobra.Command {
	options := &Options{}

	cmd := &cobra.Command{
		Use:   "deploy [service...]",
		Short: "Execute the list of commands specified in the 'deploy' section of your okteto manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOptionVars(options.Variables); err != nil {
				return err
			}
			if err := setOptionVarsAsEnvs(options.Variables); err != nil {
				return err
			}

			// This is needed because the deploy command needs the original kubeconfig configuration even in the execution within another
			// deploy command. If not, we could be proxying a proxy and we would be applying the incorrect deployed-by label
			os.Setenv(model.OktetoSkipConfigCredentialsUpdate, "false")
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
			if err := contextCMD.LoadManifestV2WithContext(ctx, options.Namespace, options.K8sContext, options.ManifestPath); err != nil {
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
					nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.Context().Namespace})
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}
			name := options.Name
			if options.Name == "" {
				name = utils.InferName(cwd)
				if err != nil {
					return fmt.Errorf("could not infer environment name")
				}
			}

			options.ShowCTA = oktetoLog.IsInteractive()
			options.servicesToDeploy = args

			kubeconfig := NewKubeConfig()

			proxy, err := NewProxy(kubeconfig)
			if err != nil {
				oktetoLog.Infof("could not configure local proxy: %s", err)
				return err
			}

			c := &DeployCommand{
				GetManifest:        model.GetManifestV2,
				Kubeconfig:         kubeconfig,
				Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat()),
				Proxy:              proxy,
				TempKubeconfigFile: GetTempKubeConfigFile(name),
				K8sClientProvider:  okteto.NewK8sClientProvider(),
				Builder:            buildv2.NewBuilderFromScratch(),
			}
			startTime := time.Now()
			err = c.RunDeploy(ctx, options)

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

			return err
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the okteto manifest file")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment is deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment is deployed")
	cmd.Flags().StringArrayVarP(&options.Variables, "var", "v", []string{}, "set a variable (can be set more than once)")
	cmd.Flags().BoolVarP(&options.Build, "build", "", false, "force build of images when deploying the development environment")
	cmd.Flags().BoolVarP(&options.Dependencies, "dependencies", "", false, "deploy the dependencies from manifest")

	cmd.Flags().BoolVarP(&options.Wait, "wait", "w", false, "wait until the development environment is deployed (defaults to false)")
	cmd.Flags().DurationVarP(&options.Timeout, "timeout", "t", (5 * time.Minute), "the length of time to wait for completion, zero means never. Any other values should contain a corresponding time unit e.g. 1s, 2m, 3h ")

	return cmd
}

// RunDeploy runs the deploy sequence
func (dc *DeployCommand) RunDeploy(ctx context.Context, deployOptions *Options) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	if err := addEnvVars(ctx, cwd); err != nil {
		return err
	}
	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := dc.Kubeconfig.Modify(dc.Proxy.GetPort(), dc.Proxy.GetToken(), dc.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}
	oktetoLog.SetStage("Load manifest")
	deployOptions.Manifest, err = dc.GetManifest(deployOptions.ManifestPath)
	if err != nil {
		return err
	}
	oktetoLog.Debug("found okteto manifest")
	if deployOptions.Manifest.Deploy == nil {
		return oktetoErrors.ErrManifestFoundButNoDeployCommands
	}
	if len(deployOptions.servicesToDeploy) > 0 && deployOptions.Manifest.Deploy.ComposeSection == nil {
		return oktetoErrors.ErrDeployCantDeploySvcsIfNotCompose
	}

	setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c)

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

	dc.Proxy.SetName(deployOptions.Name)
	// don't divert if current namespace is the diverted namespace
	if deployOptions.Manifest.Deploy.Divert != nil {
		if !okteto.IsOkteto() {
			return errors.ErrDivertNotSupported
		}
		if deployOptions.Manifest.Deploy.Divert.Namespace != deployOptions.Manifest.Namespace {
			dc.Proxy.SetDivert(deployOptions.Manifest.Deploy.Divert.Namespace)
		}
	}
	oktetoLog.SetStage("")

	dc.PipelineType = deployOptions.Manifest.Type

	os.Setenv(model.OktetoNameEnvVar, deployOptions.Name)

	if deployOptions.Dependencies && !okteto.IsOkteto() {
		return fmt.Errorf("'dependencies' is only supported in clusters that have Okteto installed")
	}

	setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c)
	oktetoLog.Debugf("starting server on %d", dc.Proxy.GetPort())
	dc.Proxy.Start()

	cfg, err := getConfigMapFromData(ctx, data, c)
	if err != nil {
		return err
	}

	for depName, dep := range deployOptions.Manifest.Dependencies {
		oktetoLog.Information("Deploying dependency '%s'", depName)
		dep.Variables = append(dep.Variables, model.EnvVar{
			Name:  "OKTETO_ORIGIN",
			Value: "okteto-deploy",
		})
		pipOpts := &pipelineCMD.DeployOptions{
			Name:         depName,
			Repository:   dep.Repository,
			Branch:       dep.Branch,
			File:         dep.ManifestPath,
			Variables:    model.SerializeBuildArgs(dep.Variables),
			Wait:         dep.Wait,
			Timeout:      deployOptions.Timeout,
			SkipIfExists: !deployOptions.Dependencies,
		}
		if err := pipelineCMD.ExecuteDeployPipeline(ctx, pipOpts); err != nil {
			if errStatus := updateConfigMapStatus(ctx, cfg, c, data, err); errStatus != nil {
				return errStatus
			}

			return err
		}
	}

	if deployOptions.Build {
		buildOptions := &types.BuildOptions{
			EnableStages: true,
			Manifest:     deployOptions.Manifest,
			CommandArgs:  deployOptions.servicesToDeploy,
		}
		oktetoLog.Debug("force build from manifest definition")
		if errBuild := dc.Builder.Build(ctx, buildOptions); errBuild != nil {
			return updateConfigMapStatusError(ctx, cfg, c, data, errBuild)
		}
	} else {
		svcsToBuild, errBuild := dc.Builder.GetServicesToBuild(ctx, deployOptions.Manifest, deployOptions.servicesToDeploy)
		if errBuild != nil {
			return updateConfigMapStatusError(ctx, cfg, c, data, errBuild)
		}
		if len(svcsToBuild) != 0 {
			buildOptions := &types.BuildOptions{
				CommandArgs:  svcsToBuild,
				EnableStages: true,
				Manifest:     deployOptions.Manifest,
			}

			if errBuild := dc.Builder.Build(ctx, buildOptions); errBuild != nil {
				return updateConfigMapStatusError(ctx, cfg, c, data, errBuild)
			}
		}
	}

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Deploying '%s'...", deployOptions.Name)

	defer dc.cleanUp(ctx)

	for _, variable := range deployOptions.Variables {
		value := strings.SplitN(variable, "=", 2)[1]
		if strings.TrimSpace(value) != "" {
			oktetoLog.AddMaskedWord(value)
		}
	}
	deployOptions.Variables = append(
		deployOptions.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("%s=%s", model.KubeConfigEnvVar, dc.TempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all okteto commands ran inside this deploy
		// know they are running inside another okteto deploy
		fmt.Sprintf("%s=true", model.OktetoWithinDeployCommandContextEnvVar),
		// Set OKTETO_SKIP_CONFIG_CREDENTIALS_UPDATE env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		fmt.Sprintf("%s=true", model.OktetoSkipConfigCredentialsUpdate),
		// Set OKTETO_DISABLE_SPINNER=true env variable, so all the Okteto commands disable spinner which leads to errors
		fmt.Sprintf("%s=true", model.OktetoDisableSpinnerEnvVar),
		// Set OKTETO_NAMESPACE=namespace-name env variable, so all the commandsruns on the same namespace
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
	)
	oktetoLog.EnableMasking()
	err = dc.deploy(ctx, deployOptions)
	oktetoLog.DisableMasking()
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")
	oktetoLog.SetStage("")

	if err != nil {
		if err == oktetoErrors.ErrIntSig {
			return nil
		}
		err = oktetoErrors.UserError{
			E:    err,
			Hint: "Update the 'deploy' section of your okteto manifest and try again",
		}
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
				if err := dc.wait(ctx, deployOptions); err != nil {
					return err
				}
			}
			if !utils.LoadBoolean(model.OktetoWithinDeployCommandContextEnvVar) {
				if err := dc.showEndpoints(ctx, &EndpointsOptions{Name: deployOptions.Name}); err != nil {
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

	if err := pipeline.UpdateConfigMap(ctx, cfg, data, c); err != nil {
		return err
	}

	return err
}

func updateConfigMapStatusError(ctx context.Context, cfg *corev1.ConfigMap, c kubernetes.Interface, data *pipeline.CfgData, errMain error) error {
	if err := updateConfigMapStatus(ctx, cfg, c, data, errMain); err != nil {
		return err
	}

	return errMain
}

func getConfigMapFromData(ctx context.Context, data *pipeline.CfgData, c kubernetes.Interface) (*corev1.ConfigMap, error) {
	return pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
}

func updateConfigMapStatus(ctx context.Context, cfg *corev1.ConfigMap, c kubernetes.Interface, data *pipeline.CfgData, err error) error {
	oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, err.Error())
	data.Status = pipeline.ErrorStatus

	return pipeline.UpdateConfigMap(ctx, cfg, data, c)
}

func setDeployOptionsValuesFromManifest(ctx context.Context, deployOptions *Options, cwd string, c kubernetes.Interface) {
	if deployOptions.Manifest.Context == "" {
		deployOptions.Manifest.Context = okteto.Context().Name
	}
	if deployOptions.Manifest.Namespace == "" {
		deployOptions.Manifest.Namespace = okteto.Context().Namespace
	}

	if deployOptions.Name == "" {
		if deployOptions.Manifest.Name != "" {
			deployOptions.Name = deployOptions.Manifest.Name
		} else {
			deployOptions.Name = utils.InferName(cwd)
			deployOptions.Manifest.Name = deployOptions.Name
		}

	} else {
		if deployOptions.Manifest != nil {
			deployOptions.Manifest.Name = deployOptions.Name
		}
		if deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.ComposeSection != nil && deployOptions.Manifest.Deploy.ComposeSection.Stack != nil {
			deployOptions.Manifest.Deploy.ComposeSection.Stack.Name = deployOptions.Name
		}
	}

	if deployOptions.Manifest.Deploy != nil && deployOptions.Manifest.Deploy.ComposeSection != nil && deployOptions.Manifest.Deploy.ComposeSection.Stack != nil {

		mergeServicesToDeployFromOptionsAndManifest(deployOptions)
		if len(deployOptions.servicesToDeploy) == 0 {
			deployOptions.servicesToDeploy = []string{}
			for service := range deployOptions.Manifest.Deploy.ComposeSection.Stack.Services {
				deployOptions.servicesToDeploy = append(deployOptions.servicesToDeploy, service)
			}
		}
		if len(deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo) > 0 {
			deployOptions.servicesToDeploy = stack.AddDependentServicesIfNotPresent(ctx, deployOptions.Manifest.Deploy.ComposeSection.Stack, deployOptions.servicesToDeploy, c)
			deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo[0].ServicesToDeploy = deployOptions.servicesToDeploy
		}
	}
}

func mergeServicesToDeployFromOptionsAndManifest(deployOptions *Options) {
	var manifestDeclaredServicesToDeploy []string
	for _, composeInfo := range deployOptions.Manifest.Deploy.ComposeSection.ComposesInfo {
		manifestDeclaredServicesToDeploy = append(manifestDeclaredServicesToDeploy, composeInfo.ServicesToDeploy...)
	}

	manifestDeclaredServicesToDeploySet := map[string]bool{}
	for _, service := range manifestDeclaredServicesToDeploy {
		manifestDeclaredServicesToDeploySet[service] = true
	}

	commandDeclaredServicesToDeploy := map[string]bool{}
	for _, service := range deployOptions.servicesToDeploy {
		commandDeclaredServicesToDeploy[service] = true
	}

	if reflect.DeepEqual(manifestDeclaredServicesToDeploySet, commandDeclaredServicesToDeploy) {
		return
	}

	if len(deployOptions.servicesToDeploy) > 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		oktetoLog.Warning("overwriting manifest's `services to deploy` with command line arguments")
	}
	if len(deployOptions.servicesToDeploy) == 0 && len(manifestDeclaredServicesToDeploy) > 0 {
		deployOptions.servicesToDeploy = manifestDeclaredServicesToDeploy
	}
}

func (dc *DeployCommand) deploy(ctx context.Context, opts *Options) error {
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	exit := make(chan error, 1)
	go func() {
		for _, command := range opts.Manifest.Deploy.Commands {
			oktetoLog.Information("Running %s", command.Name)
			oktetoLog.SetStage(command.Name)
			if err := dc.Executor.Execute(command, opts.Variables); err != nil {
				oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error executing command '%s': %s", command.Name, err.Error())
				exit <- fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
				return
			}
			oktetoLog.SetStage("")
		}

		if opts.Manifest.Deploy.ComposeSection != nil {
			if err := dc.deployStack(ctx, opts); err != nil {
				exit <- err
				return
			}
		}

		if opts.Manifest.Deploy.Divert != nil && opts.Manifest.Deploy.Divert.Namespace != opts.Manifest.Namespace {
			if err := dc.deployDivert(ctx, opts); err != nil {
				exit <- err
				return
			}
			oktetoLog.Success("Divert from '%s' successfully configured", opts.Manifest.Deploy.Divert.Namespace)
		}

		exit <- nil
	}()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		sp := utils.NewSpinner("Shutting down...")
		sp.Start()
		defer sp.Stop()
		dc.Executor.CleanUp(oktetoErrors.ErrIntSig)
		return oktetoErrors.ErrIntSig
	case err := <-exit:
		return err
	}
}

func (dc *DeployCommand) deployStack(ctx context.Context, opts *Options) error {
	oktetoLog.SetStage("Deploying compose")
	defer oktetoLog.SetStage("")
	composeSectionInfo := opts.Manifest.Deploy.ComposeSection
	composeSectionInfo.Stack.Namespace = okteto.Context().Namespace
	var composeFiles []string
	for _, composeInfo := range composeSectionInfo.ComposesInfo {
		composeFiles = append(composeFiles, composeInfo.File)
	}
	stackOpts := &stack.StackDeployOptions{
		StackPaths:       composeFiles,
		ForceBuild:       false,
		Wait:             opts.Wait,
		Timeout:          opts.Timeout,
		ServicesToDeploy: opts.servicesToDeploy,
		InsidePipeline:   true,
	}

	c, cfg, err := dc.K8sClientProvider.Provide(kubeconfig.Get([]string{dc.TempKubeconfigFile}))
	if err != nil {
		return err
	}
	stackCommand := stackCMD.DeployCommand{
		K8sClient:      c,
		Config:         cfg,
		IsInsideDeploy: true,
	}
	return stackCommand.RunDeploy(ctx, composeSectionInfo.Stack, stackOpts)
}

func (dc *DeployCommand) deployDivert(ctx context.Context, opts *Options) error {
	oktetoLog.SetStage("Divert configuration")
	defer oktetoLog.SetStage("")

	sp := utils.NewSpinner(fmt.Sprintf("Diverting namespace %s...", opts.Manifest.Deploy.Divert.Namespace))
	sp.Start()
	defer sp.Stop()

	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	result, err := c.NetworkingV1().Ingresses(opts.Manifest.Deploy.Divert.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return err
	}

	for i := range result.Items {
		select {
		case <-ctx.Done():
			oktetoLog.Infof("deployDivert context cancelled")
			return ctx.Err()
		default:
			sp.Update(fmt.Sprintf("Diverting ingress %s/%s...", result.Items[i].Namespace, result.Items[i].Name))
			if err := diverts.DivertIngress(ctx, opts.Manifest, &result.Items[i], c); err != nil {
				return err
			}
		}
	}
	return nil
}

func (dc *DeployCommand) cleanUp(ctx context.Context) {
	oktetoLog.Debugf("removing temporal kubeconfig file '%s'", dc.TempKubeconfigFile)
	if err := os.Remove(dc.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	oktetoLog.Debugf("stopping local server...")
	if err := dc.Proxy.Shutdown(ctx); err != nil {
		oktetoLog.Infof("could not stop local server: %s", err)
	}
}

func newProtocolTransport(clusterConfig *rest.Config, disableHTTP2 bool) (http.RoundTripper, error) {
	copiedConfig := &rest.Config{}
	*copiedConfig = *clusterConfig

	if disableHTTP2 {
		// According to https://pkg.go.dev/k8s.io/client-go/rest#TLSClientConfig, this is the way to disable HTTP/2
		copiedConfig.TLSClientConfig.NextProtos = []string{"http/1.1"}
	}

	return rest.TransportFor(copiedConfig)
}

func isSPDY(r *http.Request) bool {
	return strings.HasPrefix(strings.ToLower(r.Header.Get(headerUpgrade)), "spdy/")
}

//GetTempKubeConfigFile returns a where the temp kubeConfigFile should be stored
func GetTempKubeConfigFile(name string) string {
	return fmt.Sprintf(tempKubeConfigTemplate, config.GetUserHomeDir(), name, time.Now().UnixMilli())
}

func addEnvVars(ctx context.Context, cwd string) error {
	if os.Getenv(model.OktetoGitBranchEnvVar) == "" {
		branch, err := utils.GetBranch(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve branch name: %s", err)
		}
		os.Setenv(model.OktetoGitBranchEnvVar, branch)
	}

	if os.Getenv(model.GithubRepositoryEnvVar) == "" {
		repo, err := model.GetRepositoryURL(cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve repo name: %s", err)
		}

		if repo != "" {
			repoHTTPS, err := switchSSHRepoToHTTPS(repo)
			if err != nil {
				return err
			}
			repo = repoHTTPS.String()
		}
		os.Setenv(model.GithubRepositoryEnvVar, repo)
	}

	if os.Getenv(model.OktetoGitCommitEnvVar) == "" {
		sha, err := utils.GetGitCommit(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not retrieve sha: %s", err)
		}
		isClean, err := utils.IsCleanDirectory(ctx, cwd)
		if err != nil {
			oktetoLog.Infof("could not status: %s", err)
		}
		if !isClean {
			sha = utils.GetRandomSHA(ctx, cwd)
		}
		os.Setenv(model.OktetoGitCommitEnvVar, sha)
	}
	if os.Getenv(model.OktetoRegistryURLEnvVar) == "" {
		os.Setenv(model.OktetoRegistryURLEnvVar, okteto.Context().Registry)
	}
	if os.Getenv(model.OktetoBuildkitHostURLEnvVar) == "" {
		os.Setenv(model.OktetoBuildkitHostURLEnvVar, okteto.Context().Builder)
	}
	return nil
}

func switchSSHRepoToHTTPS(repo string) (*url.URL, error) {
	repoURL, err := giturls.Parse(repo)
	if err != nil {
		return nil, err
	}
	if repoURL.Scheme == "ssh" {
		repoURL.Scheme = "https"
		repoURL.User = nil
		repoURL.Path = strings.TrimSuffix(repoURL.Path, ".git")
		return repoURL, nil
	}
	if repoURL.Scheme == "https" {
		return repoURL, nil
	}

	return nil, fmt.Errorf("could not detect repo protocol")
}
