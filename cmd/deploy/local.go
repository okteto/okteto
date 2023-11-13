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
	"path/filepath"
	"strings"

	"github.com/compose-spec/godotenv"
	stackCMD "github.com/okteto/okteto/cmd/stack"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/cmd/stack"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/ingresses"
	kconfig "github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"k8s.io/client-go/rest"
)

type localDeployer struct {
	Proxy              proxyInterface
	Kubeconfig         kubeConfigHandler
	ConfigMapHandler   configMapHandler
	Executor           executor.ManifestExecutor
	TempKubeconfigFile string
	K8sClientProvider  okteto.K8sClientProvider

	GetExternalControl func(cfg *rest.Config) ExternalResourceInterface

	deployWaiter DeployWaiter
	isRemote     bool
	Fs           afero.Fs
	DivertDriver divert.Driver
}

// newLocalDeployer initializes a local deployer from a name and a boolean indicating if we should run with bash or not
func newLocalDeployer(ctx context.Context, options *Options, cmapHandler configMapHandler, k8sProvider okteto.K8sClientProvider, kubeconfig kubeConfigHandler, portGetter portGetterFunc) (*localDeployer, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get the current working directory: %w", err)
	}
	tempKubeconfigName := options.Name
	if tempKubeconfigName == "" {
		c, _, err := k8sProvider.Provide(okteto.Context().Cfg)
		if err != nil {
			return nil, err
		}
		inferer := devenvironment.NewNameInferer(c)
		tempKubeconfigName = inferer.InferName(ctx, cwd, okteto.Context().Namespace, options.ManifestPathFlag)
		if err != nil {
			return nil, fmt.Errorf("could not infer environment name")
		}
	}

	proxy, err := NewProxy(kubeconfig, portGetter)
	if err != nil {
		oktetoLog.Infof("could not configure local proxy: %s", err)
		return nil, err
	}

	return &localDeployer{
		Kubeconfig:         kubeconfig,
		Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat(), options.RunWithoutBash, ""),
		ConfigMapHandler:   cmapHandler,
		Proxy:              proxy,
		TempKubeconfigFile: GetTempKubeConfigFile(tempKubeconfigName),
		K8sClientProvider:  k8sProvider,
		GetExternalControl: NewDeployExternalK8sControl,
		deployWaiter:       NewDeployWaiter(k8sProvider),
		isRemote:           true,
		Fs:                 afero.NewOsFs(),
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

	// Injecting the PROXY into the kubeconfig file
	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", ld.TempKubeconfigFile)
	if err := ld.Kubeconfig.Modify(ld.Proxy.GetPort(), ld.Proxy.GetToken(), ld.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c); err != nil {
		return err
	}

	ld.Proxy.SetName(format.ResourceK8sMetaString(deployOptions.Name))
	if deployOptions.Manifest.Deploy.Divert != nil {
		driver, err := divert.New(deployOptions.Manifest, c)
		if err != nil {
			return err
		}
		ld.Proxy.SetDivert(driver)
		ld.DivertDriver = driver
	}

	os.Setenv(constants.OktetoNameEnvVar, deployOptions.Name)

	if err := setDeployOptionsValuesFromManifest(ctx, deployOptions, cwd, c); err != nil {
		return err
	}

	oktetoLog.SetStage("")

	// starting PROXY
	oktetoLog.Debugf("starting server on %d", ld.Proxy.GetPort())
	ld.Proxy.Start()

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
	if okteto.IsOkteto() {
		deployOptions.Variables = append(
			deployOptions.Variables,
			// Set OKTETO_DOMAIN=okteto-subdomain env variable
			fmt.Sprintf("%s=%s", model.OktetoDomainEnvVar, okteto.GetSubdomain()),
		)
	}
	oktetoLog.EnableMasking()
	err = ld.runDeploySection(ctx, deployOptions)
	oktetoLog.DisableMasking()
	oktetoLog.SetStage("done")
	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "EOF")
	return err
}

func (ld *localDeployer) runDeploySection(ctx context.Context, opts *Options) error {
	oktetoEnvFile, err := ld.createTempOktetoEnvFile()
	if err != nil {
		return err
	}

	defer func() {
		if err := ld.Fs.RemoveAll(filepath.Dir(oktetoEnvFile.Name())); err != nil {
			oktetoLog.Infof("error removing okteto env file dir: %w", err)
		}
	}()

	var envMapFromOktetoEnvFile map[string]string
	// deploy commands if any
	for _, command := range opts.Manifest.Deploy.Commands {
		oktetoLog.Information("Running '%s'", command.Name)
		oktetoLog.SetStage(command.Name)
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Executing command '%s'...", command.Name)

		if err := ld.Executor.Execute(command, opts.Variables); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error executing command '%s': %s", command.Name, err.Error())
			return fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
		}
		oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Command '%s' successfully executed", command.Name)

		envMapFromOktetoEnvFile, err = godotenv.Read(oktetoEnvFile.Name())
		if err != nil {
			oktetoLog.Warning("no valid format used in the okteto env file: %s", err.Error())
		}

		envsFromOktetoEnvFile := make([]string, 0, len(envMapFromOktetoEnvFile))
		for k, v := range envMapFromOktetoEnvFile {
			envsFromOktetoEnvFile = append(envsFromOktetoEnvFile, fmt.Sprintf("%s=%s", k, v))
		}

		// the variables in the $OKTETO_ENV file are added as environment variables
		// to the executor. If there is already a previously set value for that
		// variable, the executor will use in next command the last one added which
		// corresponds to those coming from $OKTETO_ENV.
		opts.Variables = append(opts.Variables, envsFromOktetoEnvFile...)
		oktetoLog.SetStage("")
		oktetoLog.SetLevel("")
	}

	err = ld.ConfigMapHandler.updateEnvsFromCommands(ctx, opts.Name, opts.Manifest.Namespace, opts.Variables)
	if err != nil {
		return fmt.Errorf("could not update config map with environment variables: %w", err)
	}

	// deploy compose if any
	if opts.Manifest.Deploy.ComposeSection != nil {
		oktetoLog.SetStage("Deploying compose")
		if err := ld.deployStack(ctx, opts); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying compose: %s", err.Error())
			return err
		}
	}

	// deploy endpoints if any
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
		oktetoLog.SetStage("Deploy Divert")
		if err := ld.DivertDriver.Deploy(ctx); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error creating divert: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	// deploy externals if any
	if len(opts.Manifest.External) > 0 {
		oktetoLog.SetStage("External configuration")
		if !okteto.IsOkteto() {
			oktetoLog.Warning("external resources cannot be deployed on a context not managed by okteto")
			return nil
		}

		if err := ld.deployExternals(ctx, opts, envMapFromOktetoEnvFile); err != nil {
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

func (ld *localDeployer) deployExternals(ctx context.Context, opts *Options, dynamicEnvs map[string]string) error {

	_, cfg, err := ld.K8sClientProvider.Provide(kconfig.Get([]string{ld.TempKubeconfigFile}))
	if err != nil {
		return fmt.Errorf("error getting kubernetes client: %w", err)
	}
	control := ld.GetExternalControl(cfg)
	if err != nil {
		return err
	}

	for externalName, externalInfo := range opts.Manifest.External {
		oktetoLog.Spinner(fmt.Sprintf("Deploying external resource '%s'...", externalName))
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
		if err := externalInfo.SetURLUsingEnvironFile(externalName, dynamicEnvs); err != nil {
			return err
		}

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
	if ld.Executor != nil {
		ld.Executor.CleanUp(err)
	}
}

func (ld *localDeployer) createTempOktetoEnvFile() (afero.File, error) {
	oktetoEnvFileDir, err := afero.TempDir(ld.Fs, "", "")
	if err != nil {
		return nil, err
	}

	oktetoEnvFile, err := ld.Fs.Create(filepath.Join(oktetoEnvFileDir, ".env"))
	if err != nil {
		return nil, err
	}

	os.Setenv(constants.OktetoEnvFile, oktetoEnvFile.Name())
	oktetoLog.Debug("using %s as env file for deploy command", oktetoEnvFile.Name())
	return oktetoEnvFile, nil
}
