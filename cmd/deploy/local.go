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
	"strings"

	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	stackCMD "github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/externalresource"
	k8sExternalResources "github.com/okteto/okteto/pkg/externalresource/k8s"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	kconfig "github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
)

type localDeployer struct {
	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	Executor           executor.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider
	Builder            *buildv2.OktetoBuilder
	GetExternalControl func(cp okteto.K8sClientProvider, filename string) (ExternalResourceInterface, error)
	deployWaiter       deployWaiter
}

// newLocalDeployer initializes a local deployer from a name and a boolean indicating if we should run with bash or not
func newLocalDeployer(name string, runWithoutBash bool) (*localDeployer, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get the current working directory: %w", err)
	}
	tempKubeconfigName := name
	if tempKubeconfigName == "" {
		name = utils.InferName(cwd)
	}
	kubeconfig := NewKubeConfig()

	proxy, err := NewProxy(kubeconfig)
	if err != nil {
		oktetoLog.Infof("could not configure local proxy: %s", err)
		return nil, err
	}

	clientProvider := okteto.NewK8sClientProvider()
	return &localDeployer{
		Kubeconfig:         kubeconfig,
		Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat(), runWithoutBash),
		Proxy:              proxy,
		TempKubeconfigFile: GetTempKubeConfigFile(name),
		K8sClientProvider:  clientProvider,
		Builder:            buildv2.NewBuilderFromScratch(),
		GetExternalControl: GetExternalControl,
		deployWaiter:       newDeployWaiter(clientProvider),
	}, nil
}

func (ld *localDeployer) deploy(ctx context.Context, deployOptions *Options) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get the current working directory: %w", err)
	}

	// We need to create a client that doesn't go through the proxy to create
	// the configmap without the deployedByLabel
	c, _, err := ld.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	addEnvVars(ctx, cwd)

	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", ld.TempKubeconfigFile)
	if err := ld.Kubeconfig.Modify(ld.Proxy.GetPort(), ld.Proxy.GetToken(), ld.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}

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

	ld.Proxy.SetName(format.ResourceK8sMetaString(deployOptions.Name))
	// don't divert if current namespace is the diverted namespace
	if deployOptions.Manifest.Deploy.Divert != nil {
		if !okteto.IsOkteto() {
			return oktetoErrors.ErrDivertNotSupported
		}
		if deployOptions.Manifest.Deploy.Divert.Namespace != deployOptions.Manifest.Namespace {
			ld.Proxy.SetDivert(deployOptions.Manifest.Deploy.Divert.Namespace)
		}
	}

	os.Setenv(constants.OktetoNameEnvVar, deployOptions.Name)

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c); err != nil {
		return err
	}

	oktetoLog.SetStage("")

	// starting PROXY
	oktetoLog.Debugf("starting server on %d", ld.Proxy.GetPort())
	ld.Proxy.Start()

	cfg, err := getConfigMapFromData(ctx, data, c)
	if err != nil {
		return err
	}

	// TODO: take this out to a new function deploy dependencies
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
			if errStatus := updateConfigMapStatus(ctx, cfg, c, data, err); errStatus != nil {
				return errStatus
			}

			return err
		}
	}
	oktetoLog.SetStage("")

	if err := buildImages(ctx, ld.Builder.Build, ld.Builder.GetServicesToBuild, deployOptions); err != nil {
		return updateConfigMapStatusError(ctx, cfg, c, data, err)
	}

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Deploying '%s'...", deployOptions.Name)

	defer ld.cleanUp(ctx, nil)

	for _, variable := range deployOptions.Variables {
		value := strings.SplitN(variable, "=", 2)[1]
		if strings.TrimSpace(value) != "" {
			oktetoLog.AddMaskedWord(value)
		}
	}
	deployOptions.Variables = append(
		deployOptions.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("%s=%s", constants.KubeConfigEnvVar, ld.TempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all okteto commands ran inside this deploy
		// know they are running inside another okteto deploy
		fmt.Sprintf("%s=true", constants.OktetoWithinDeployCommandContextEnvVar),
		// Set OKTETO_SKIP_CONFIG_CREDENTIALS_UPDATE env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		fmt.Sprintf("%s=true", constants.OktetoSkipConfigCredentialsUpdate),
		// Set OKTETO_DISABLE_SPINNER=true env variable, so all the Okteto commands disable spinner which leads to errors
		fmt.Sprintf("%s=true", oktetoLog.OktetoDisableSpinnerEnvVar),
		// Set OKTETO_NAMESPACE=namespace-name env variable, so all the commandsruns on the same namespace
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.Context().Namespace),
		// Set OKTETO_AUTODISCOVERY_RELEASE_NAME=sanitized name, so the release name in case of autodiscovery of helm is valid
		fmt.Sprintf("%s=%s", constants.OktetoAutodiscoveryReleaseName, format.ResourceK8sMetaString(deployOptions.Name)),
	)
	oktetoLog.EnableMasking()
	err = ld.runDeploySection(ctx, deployOptions)
	oktetoLog.DisableMasking()
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")

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
				if err := ld.deployWaiter.wait(ctx, deployOptions); err != nil {
					return err
				}
			}
			if !utils.LoadBoolean(constants.OktetoWithinDeployCommandContextEnvVar) {
				eg := endpointGetter{
					K8sClientProvider:  ld.K8sClientProvider,
					GetExternalControl: ld.GetExternalControl,
					TempKubeconfigFile: ld.TempKubeconfigFile,
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

	if err := pipeline.UpdateConfigMap(ctx, cfg, data, c); err != nil {
		return err
	}

	return err
}

func (ld *localDeployer) runDeploySection(ctx context.Context, opts *Options) error {
	// deploy commands if any
	for _, command := range opts.Manifest.Deploy.Commands {
		oktetoLog.Information("Running '%s'", command.Name)
		oktetoLog.SetStage(command.Name)
		if err := ld.Executor.Execute(command, opts.Variables); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error executing command '%s': %s", command.Name, err.Error())
			return fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
		}
		oktetoLog.SetStage("")
	}

	// deploy compose if any
	if opts.Manifest.Deploy.ComposeSection != nil {
		oktetoLog.SetStage("Deploying compose")
		if err := ld.deployStack(ctx, opts); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying compose: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	// deploy endpoits if any
	if opts.Manifest.Deploy.Endpoints != nil {
		oktetoLog.SetStage("Endpoints configuration")
		if err := ld.deployEndpoints(ctx, opts); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error generating endpoints: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	// deploy divert if any
	if opts.Manifest.Deploy.Divert != nil && opts.Manifest.Deploy.Divert.Namespace != opts.Manifest.Namespace {
		oktetoLog.SetStage("Divert configuration")
		if err := ld.deployDivert(ctx, opts); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error creating divert: %s", err.Error())
			return err
		}
		oktetoLog.Success("Divert from '%s' successfully configured", opts.Manifest.Deploy.Divert.Namespace)
		oktetoLog.SetStage("")
	}

	// deploy externals if any
	if opts.Manifest.External != nil {
		oktetoLog.SetStage("External configuration")
		if !okteto.IsOkteto() {
			oktetoLog.Warning("external resources cannot be deployed on a cluster not managed by okteto")
			return nil
		}
		if err := ld.deployExternals(ctx, opts); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying external resources: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	return nil
}

func (ld *localDeployer) deployStack(ctx context.Context, opts *Options) error {
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

	c, cfg, err := ld.K8sClientProvider.Provide(kconfig.Get([]string{ld.TempKubeconfigFile}))
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

func (ld *localDeployer) deployDivert(ctx context.Context, opts *Options) error {

	oktetoLog.Spinner(fmt.Sprintf("Diverting namespace %s...", opts.Manifest.Deploy.Divert.Namespace))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	c, _, err := ld.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	dClient, err := diverts.GetDivertClient()
	if err != nil {
		return fmt.Errorf("error creating divert CRD client: %s", err.Error())
	}

	driver := divert.New(opts.Manifest, dClient, c)
	return driver.Deploy(ctx)
}

func (ld *localDeployer) deployEndpoints(ctx context.Context, opts *Options) error {

	c, _, err := ld.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return err
	}

	iClient, err := ingresses.GetClient(c)
	if err != nil {
		return fmt.Errorf("error getting ingress client: %s", err.Error())
	}

	translateOptions := &ingresses.TranslateOptions{
		Namespace: opts.Manifest.Namespace,
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

func (ld *localDeployer) deployExternals(ctx context.Context, opts *Options) error {
	control, err := ld.GetExternalControl(ld.K8sClientProvider, ld.TempKubeconfigFile)
	if err != nil {
		return err
	}

	for externalName, externalInfo := range opts.Manifest.External {
		oktetoLog.Spinner(fmt.Sprintf("Deploying external resource '%s'...", externalName))
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
		err := control.Deploy(ctx, externalName, opts.Manifest.Namespace, externalInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ld *localDeployer) cleanUp(ctx context.Context, err error) {
	oktetoLog.Debugf("removing temporal kubeconfig file '%s'", ld.TempKubeconfigFile)
	if err := os.Remove(ld.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	oktetoLog.Debugf("stopping local server...")
	if err := ld.Proxy.Shutdown(ctx); err != nil {
		oktetoLog.Infof("could not stop local server: %s", err)
	}
	ld.Executor.CleanUp(err)
}

func GetExternalControl(cp okteto.K8sClientProvider, filename string) (ExternalResourceInterface, error) {
	_, proxyConfig, err := cp.Provide(kconfig.Get([]string{filename}))
	if err != nil {
		return nil, err
	}

	return &externalresource.K8sControl{
		ClientProvider: k8sExternalResources.GetExternalClient,
		Cfg:            proxyConfig,
	}, nil
}
