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
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"reflect"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	stackCMD "github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/spf13/cobra"
	giturls "github.com/whilp/git-urls"
	"golang.org/x/sync/errgroup"
	"k8s.io/client-go/rest"
)

const (
	headerUpgrade          = "Upgrade"
	succesfullyDeployedmsg = "Development environment '%s' successfully deployed"
)

var tempKubeConfigTemplate = "%s/.okteto/kubeconfig-%s-%d"

// Options options for deploy command
type Options struct {
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
}

//DeployCommand defines the config for deploying an app
type DeployCommand struct {
	GetManifest func(path string) (*model.Manifest, error)

	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	Executor           executor.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider

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
				workdir := utils.GetWorkdirFromManifestPath(options.ManifestPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				options.ManifestPath = utils.GetManifestPathFromWorkdir(options.ManifestPath, workdir)
			}
			if err := contextCMD.LoadManifestV2WithContext(ctx, options.Namespace, options.K8sContext, options.ManifestPath); err != nil {
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

	setDeployOptionsValuesFromManifest(deployOptions, cwd)

	dc.Proxy.SetName(deployOptions.Name)
	oktetoLog.SetStage("")

	dc.PipelineType = deployOptions.Manifest.Type

	os.Setenv(model.OktetoNameEnvVar, deployOptions.Name)

	if deployOptions.Dependencies && !okteto.IsOkteto() {
		return fmt.Errorf("'dependencies' is only available in clusters managed by Okteto")
	}

	if deployOptions.Build {
		for service := range deployOptions.Manifest.Build {
			oktetoLog.Debug("force build from manifest definition")
			if err := runBuildAndSetEnvs(ctx, service, deployOptions.Manifest); err != nil {
				return err
			}
		}

	} else if err := checkBuildFromManifest(ctx, deployOptions.Manifest); err != nil {
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
			return err
		}
	}

	deployOptions.Manifest, err = deployOptions.Manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	setDeployOptionsValuesFromManifest(deployOptions, cwd)

	oktetoLog.Debugf("starting server on %d", dc.Proxy.GetPort())
	dc.Proxy.Start()

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Deploying '%s'...", deployOptions.Name)
	data := &pipeline.CfgData{
		Name:       deployOptions.Name,
		Namespace:  deployOptions.Manifest.Namespace,
		Repository: os.Getenv(model.GithubRepositoryEnvVar),
		Branch:     os.Getenv(model.OktetoGitBranchEnvVar),
		Filename:   deployOptions.Manifest.Filename,
		Status:     pipeline.ProgressingStatus,
		Manifest:   deployOptions.Manifest.Manifest,
		Icon:       deployOptions.Manifest.Icon,
	}

	if !deployOptions.Manifest.IsV2 && deployOptions.Manifest.Type == model.StackType {
		data.Manifest = deployOptions.Manifest.Deploy.ComposeSection.Stack.Manifest
	}
	c, _, err := dc.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}
	cfg, err := pipeline.TranslateConfigMapAndDeploy(ctx, data, c)
	if err != nil {
		return err
	}

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
			if err := pipeline.AddDevAnnotations(ctx, deployOptions.Manifest, deployOptions.servicesToDeploy, c); err != nil {
				oktetoLog.Warning("could not add dev annotations due to: %s", err.Error())
			}
		}
		data.Status = pipeline.DeployedStatus
	}
	if err := pipeline.UpdateConfigMap(ctx, cfg, data, c); err != nil {
		return err
	}
	return err
}

func setDeployOptionsValuesFromManifest(deployOptions *Options, cwd string) {
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
		if len(deployOptions.servicesToDeploy) > 0 {
			servicesToDeploy := map[string]bool{}
			for _, service := range deployOptions.servicesToDeploy {
				servicesToDeploy[service] = true
			}

			onlyDeployEndpointsFromServicesToDeploy(deployOptions.Manifest.Deploy.ComposeSection.Stack.Endpoints, servicesToDeploy)

			onlyDeployVolumesFromServicesToDeploy(deployOptions.Manifest.Deploy.ComposeSection.Stack, servicesToDeploy)

		} else {
			deployOptions.servicesToDeploy = []string{}
			for service := range deployOptions.Manifest.Deploy.ComposeSection.Stack.Services {
				deployOptions.servicesToDeploy = append(deployOptions.servicesToDeploy, service)
			}
		}
	}

	if len(deployOptions.servicesToDeploy) == 0 {
		deployOptions.servicesToDeploy = []string{deployOptions.Name}
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

func onlyDeployEndpointsFromServicesToDeploy(endpoints model.EndpointSpec, servicesToDeploy map[string]bool) {
	for key, spec := range endpoints {
		newRules := []model.EndpointRule{}
		for _, rule := range spec.Rules {
			if servicesToDeploy[rule.Service] {
				newRules = append(newRules, rule)
			}
		}
		spec.Rules = newRules
		endpoints[key] = spec
	}
}

func onlyDeployVolumesFromServicesToDeploy(stack *model.Stack, servicesToDeploy map[string]bool) {

	volumesToDeploy := map[string]bool{}

	for serviceName, serviceSpec := range stack.Services {
		if servicesToDeploy[serviceName] {
			for _, volume := range serviceSpec.Volumes {
				if stack.Volumes[volume.LocalPath] != nil {
					volumesToDeploy[volume.LocalPath] = true
				}
			}
		}
	}

	for volume := range stack.Volumes {
		if !volumesToDeploy[volume] {
			delete(stack.Volumes, volume)
		}
	}
}

func (dc *DeployCommand) deploy(ctx context.Context, opts *Options) error {
	stopCmds := make(chan os.Signal, 1)
	signal.Notify(stopCmds, os.Interrupt)
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
		exit <- nil
	}()
	shouldExit := false
	for {
		select {
		case <-stopCmds:
			oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
			sp := utils.NewSpinner("Shutting down...")
			sp.Start()
			defer sp.Stop()
			dc.Executor.CleanUp(errors.New("interrupt signal received"))
			return oktetoErrors.ErrIntSig
		case err := <-exit:
			if err != nil {
				return err
			}
			shouldExit = true
		}
		if shouldExit {
			break
		}
	}

	stopCompose := make(chan os.Signal, 1)
	signal.Notify(stopCompose, os.Interrupt)
	exitCompose := make(chan error, 1)

	go func() {
		if opts.Manifest.Deploy.ComposeSection != nil {
			oktetoLog.SetStage("Deploying compose")
			err := dc.deployStack(ctx, opts)
			exitCompose <- err
			return
		}
		exitCompose <- nil
	}()
	shouldExit = false
	for {
		select {
		case <-stopCmds:
			os.Unsetenv(model.OktetoDisableSpinnerEnvVar)
			oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
			return oktetoErrors.ErrIntSig
		case err := <-exitCompose:
			if err != nil {
				return fmt.Errorf("Error deploying compose: %w", err)
			}
			shouldExit = true
		}
		if shouldExit {
			break
		}
	}
	return nil
}

func (dc *DeployCommand) deployStack(ctx context.Context, opts *Options) error {
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
	oktetoLog.Information("Deploying compose")
	return stackCommand.RunDeploy(ctx, composeSectionInfo.Stack, stackOpts)
}

func checkImageAtGlobalAndSetEnvs(service string, options *build.BuildOptions) (bool, error) {
	globalReference := strings.Replace(options.Tag, okteto.DevRegistry, okteto.GlobalRegistry, 1)

	imageWithDigest, err := registry.GetImageTagWithDigest(globalReference)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not built at global registry, not running optimization for deployment")
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if err := SetManifestEnvVars(service, imageWithDigest); err != nil {
		return false, err
	}
	oktetoLog.Debug("image already built at global registry, running optimization for deployment")
	return true, nil

}

func runBuildAndSetEnvs(ctx context.Context, service string, manifest *model.Manifest) error {
	oktetoLog.SetStage(fmt.Sprintf("Building service %s", service))
	buildInfo := manifest.Build[service]
	isStack := manifest.Type == model.StackType
	oktetoLog.Information("Building image for service '%s'", service)
	volumesToInclude := build.GetVolumesToInclude(buildInfo.VolumesToInclude)
	if len(volumesToInclude) > 0 {
		buildInfo.VolumesToInclude = nil
	}
	if isStack && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}
	options := build.OptsFromManifest(service, buildInfo, &build.BuildOptions{})
	if err := build.Run(ctx, options); err != nil {
		return err
	}
	imageWithDigest, err := registry.GetImageTagWithDigest(options.Tag)
	if err != nil {
		return fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
	}

	if len(volumesToInclude) > 0 {
		oktetoLog.Information("Including volume hosts for service '%s'", service)
		svcBuild, err := registry.CreateDockerfileWithVolumeMounts(options.Tag, volumesToInclude)
		if err != nil {
			return err
		}
		svcBuild.VolumesToInclude = volumesToInclude
		options = build.OptsFromManifest(service, svcBuild, &build.BuildOptions{})
		if err := build.Run(ctx, options); err != nil {
			return err
		}
		imageWithDigest, err = registry.GetImageTagWithDigest(options.Tag)
		if err != nil {
			return fmt.Errorf("error accessing image at registry %s: %v", options.Tag, err)
		}
	}
	oktetoLog.Success("Image for service '%s' pushed to registry: %s", service, options.Tag)
	if err := SetManifestEnvVars(service, imageWithDigest); err != nil {
		return err
	}
	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
		stack := manifest.Deploy.ComposeSection.Stack
		if svc, ok := stack.Services[service]; ok {
			svc.Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
		}
	}
	oktetoLog.SetStage("")
	return nil
}

// SetManifestEnvVars set okteto build env vars
func SetManifestEnvVars(service, reference string) error {
	reg, repo, tag, image := registry.GetReferecenceEnvs(reference)

	oktetoLog.Debugf("envs registry=%s repository=%s image=%s tag=%s", reg, repo, image, tag)

	service = strings.ReplaceAll(service, "-", "_")
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REGISTRY", strings.ToUpper(service)), reg)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_REPOSITORY", strings.ToUpper(service)), repo)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", strings.ToUpper(service)), reference)
	os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_TAG", strings.ToUpper(service)), tag)

	oktetoLog.Debug("manifest env vars set")
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
		value := fmt.Sprintf("%s%s", model.OktetoGitCommitPrefix, sha)
		os.Setenv(model.OktetoGitCommitEnvVar, value)
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

func checkServicesToBuild(service string, manifest *model.Manifest, ch chan string) error {
	buildInfo := manifest.Build[service]
	isStack := manifest.Type == model.StackType
	if isStack && okteto.IsOkteto() && !registry.IsOktetoRegistry(buildInfo.Image) {
		buildInfo.Image = ""
	}
	opts := build.OptsFromManifest(service, buildInfo, &build.BuildOptions{})

	if build.ShouldOptimizeBuild(opts.Tag) {
		oktetoLog.Debug("found OKTETO_GIT_COMMIT, optimizing the build flow")
		if skipBuild, err := checkImageAtGlobalAndSetEnvs(service, opts); err != nil {
			return err
		} else if skipBuild {
			oktetoLog.Debugf("Skipping '%s' build. Image already exists at Okteto Registry", service)
			return nil
		}
	}

	imageWithDigest, err := registry.GetImageTagWithDigest(opts.Tag)
	if err == oktetoErrors.ErrNotFound {
		oktetoLog.Debug("image not found, building image")
		ch <- service
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
	}
	oktetoLog.Debug("Skipping build for image for service")

	if err := SetManifestEnvVars(service, imageWithDigest); err != nil {
		return err
	}
	if manifest.Deploy != nil && manifest.Deploy.ComposeSection != nil && manifest.Deploy.ComposeSection.Stack != nil {
		stack := manifest.Deploy.ComposeSection.Stack
		if svc, ok := stack.Services[service]; ok && svc.Image == "" {
			stack.Services[service].Image = fmt.Sprintf("${OKTETO_BUILD_%s_IMAGE}", strings.ToUpper(strings.ReplaceAll(service, "-", "_")))
		}
	}
	return nil
}

func checkBuildFromManifest(ctx context.Context, manifest *model.Manifest) error {
	buildManifest := manifest.Build
	// check if images are at registry (global or dev) and set envs or send to build
	toBuild := make(chan string, len(buildManifest))
	g, _ := errgroup.WithContext(ctx)

	for service := range buildManifest {
		svc := service
		g.Go(func() error {
			return checkServicesToBuild(svc, manifest, toBuild)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	close(toBuild)

	if len(toBuild) == 0 {
		oktetoLog.Information("Images were already built. To rebuild your images run 'okteto build' or 'okteto deploy --build'")
		return nil
	}

	for svc := range toBuild {
		oktetoLog.Information("Building image for service '%s'...", svc)
		oktetoLog.Information("To rebuild your image manually run 'okteto build %s' or 'okteto deploy --build'", svc)
		if err := runBuildAndSetEnvs(ctx, svc, manifest); err != nil {
			return err
		}
	}

	return nil
}
