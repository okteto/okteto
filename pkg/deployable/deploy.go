// Copyright 2024 The Okteto Authors
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

package deployable

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/externalresource"
	"github.com/okteto/okteto/pkg/format"
	kconfig "github.com/okteto/okteto/pkg/k8s/kubeconfig"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/spf13/afero"
	"k8s.io/client-go/rest"
)

const (
	deployCommandsPhaseName = "commands"
)

// DivertDeployer defines the operations to deploy the divert section of a deployable
type DivertDeployer interface {
	Deploy(ctx context.Context) error
}

// ProxyInterface defines the different operations to work with the proxy run by Okteto
type ProxyInterface interface {
	Start()
	Shutdown(ctx context.Context) error
	GetPort() int
	GetToken() string
	SetName(name string)
	SetDivert(driver divert.Driver)
}

// KubeConfigHandler defines the operations to handle the kubeconfig file
// needed to deal with the local Kubernetes proxy
type KubeConfigHandler interface {
	Read() (*rest.Config, error)
	Modify(port int, sessionToken, destKubeconfigFile string) error
}

// ConfigMapHandler defines the operations to handle the ConfigMap with the
// information related to the development environment
type ConfigMapHandler interface {
	UpdateEnvsFromCommands(context.Context, string, string, []string) error
	AddPhaseDuration(context.Context, string, string, string, time.Duration) error
}

// ExternalResourceInterface defines the operations to work with external resources
type ExternalResourceInterface interface {
	Deploy(ctx context.Context, name string, ns string, externalInfo *externalresource.ExternalResource) error
}

// DeployRunner is responsible for running the commands defined in a manifest, deploy the divert
// information and deploy external resources.
// This DeployRunner has the common functionality to deal with the mentioned resources when deploy is
// run locally or remotely. As this runs also in the remote, it should NEVER build any kind of image
// or execute some logic that might differ from local.
type DeployRunner struct {
	Proxy              ProxyInterface
	Kubeconfig         KubeConfigHandler
	ConfigMapHandler   ConfigMapHandler
	Executor           executor.ManifestExecutor
	K8sClientProvider  okteto.K8sClientProviderWithLogger
	Fs                 afero.Fs
	DivertDeployer     DivertDeployer
	GetExternalControl func(cfg *rest.Config) ExternalResourceInterface
	k8sLogger          *io.K8sLogger
	TempKubeconfigFile string
}

// Entity represents a set of resources that can be deployed by the runner
type Entity struct {
	External externalresource.Section
	Divert   *model.DivertDeploy
	Commands []model.DeployCommand
}

// DeployParameters represents the parameters for deploying a remote entity
type DeployParameters struct {
	Name         string
	Namespace    string
	ManifestPath string
	Deployable   Entity
	Variables    []string
}

// PortGetterFunc is a function that retrieves a free port the port for specified interface
type PortGetterFunc func(string) (int, error)

// newDeployExternalK8sControl creates a new instance of external resources controller
func newDeployExternalK8sControl(cfg *rest.Config) ExternalResourceInterface {
	return externalresource.NewExternalK8sControl(cfg)
}

// NewDeployRunnerForRemote initializes a runner for a remote environment
func NewDeployRunnerForRemote(
	name string,
	runWithoutBash bool,
	cmapHandler ConfigMapHandler,
	k8sProvider okteto.K8sClientProviderWithLogger,
	portGetter PortGetterFunc,
	k8sLogger *io.K8sLogger,
) (*DeployRunner, error) {
	kubeconfig := NewKubeConfig()
	tempKubeconfigName := name

	proxy, err := NewProxy(kubeconfig, portGetter)
	if err != nil {
		oktetoLog.Infof("could not configure local proxy: %s", err)
		return nil, err
	}

	return &DeployRunner{
		Kubeconfig:         kubeconfig,
		Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat(), runWithoutBash, ""),
		ConfigMapHandler:   cmapHandler,
		Proxy:              proxy,
		TempKubeconfigFile: GetTempKubeConfigFile(tempKubeconfigName),
		K8sClientProvider:  k8sProvider,
		GetExternalControl: newDeployExternalK8sControl,
		Fs:                 afero.NewOsFs(),
		k8sLogger:          k8sLogger,
	}, nil
}

// NewDeployRunnerForLocal initializes a runner for a local environment. The main difference with the remote
// initialization is that the name might be empty in the local runner and it has to be inferred.
func NewDeployRunnerForLocal(
	ctx context.Context,
	name string,
	runWithoutBash bool,
	manifestPathFlag string,
	cmapHandler ConfigMapHandler,
	k8sProvider okteto.K8sClientProviderWithLogger,
	portGetter PortGetterFunc,
	k8sLogger *io.K8sLogger,
) (*DeployRunner, error) {
	kubeconfig := NewKubeConfig()
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get the current working directory: %w", err)
	}
	tempKubeconfigName := name
	if tempKubeconfigName == "" {
		c, _, err := k8sProvider.ProvideWithLogger(okteto.GetContext().Cfg, k8sLogger)
		if err != nil {
			return nil, err
		}
		inferer := devenvironment.NewNameInferer(c)
		tempKubeconfigName = inferer.InferName(ctx, cwd, okteto.GetContext().Namespace, manifestPathFlag)
	}

	proxy, err := NewProxy(kubeconfig, portGetter)
	if err != nil {
		oktetoLog.Infof("could not configure local proxy: %s", err)
		return nil, err
	}

	return &DeployRunner{
		Kubeconfig:         kubeconfig,
		Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat(), runWithoutBash, ""),
		ConfigMapHandler:   cmapHandler,
		Proxy:              proxy,
		TempKubeconfigFile: GetTempKubeConfigFile(tempKubeconfigName),
		K8sClientProvider:  k8sProvider,
		GetExternalControl: newDeployExternalK8sControl,
		Fs:                 afero.NewOsFs(),
		k8sLogger:          k8sLogger,
	}, nil
}

// RunDeploy deploys the deployable received with DeployParameters
func (r *DeployRunner) RunDeploy(ctx context.Context, params DeployParameters) error {
	// We need to create a client that doesn't go through the proxy to create
	// the configmap without the deployedByLabel
	c, _, err := r.K8sClientProvider.ProvideWithLogger(okteto.GetContext().Cfg, r.k8sLogger)
	if err != nil {
		return err
	}

	// Injecting the PROXY into the kubeconfig file
	oktetoLog.Debugf("creating temporal kubeconfig file '%s'", r.TempKubeconfigFile)
	if err := r.Kubeconfig.Modify(r.Proxy.GetPort(), r.Proxy.GetToken(), r.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not create temporal kubeconfig %s", err)
		return err
	}

	r.Proxy.SetName(format.ResourceK8sMetaString(params.Name))
	if params.Deployable.Divert != nil {
		driver, err := divert.New(params.Deployable.Divert, params.Name, params.Namespace, c)
		if err != nil {
			return err
		}
		r.Proxy.SetDivert(driver)
		r.DivertDeployer = driver
	}

	os.Setenv(constants.OktetoNameEnvVar, params.Name)

	oktetoLog.SetStage("")

	// starting PROXY
	oktetoLog.Debugf("starting server on %d", r.Proxy.GetPort())
	r.Proxy.Start()

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Deploying '%s'...", params.Name)

	defer r.CleanUp(ctx, nil)

	// Token should be always masked from the logs
	oktetoLog.AddMaskedWord(okteto.GetContext().Token)
	keyValueVarParts := 2
	for _, variable := range params.Variables {
		varParts := strings.SplitN(variable, "=", keyValueVarParts)
		if len(varParts) >= keyValueVarParts && strings.TrimSpace(varParts[1]) != "" {
			oktetoLog.AddMaskedWord(varParts[1])
		}
	}

	params.Variables = append(
		params.Variables,
		// Set KUBECONFIG environment variable as environment for the commands to be executed
		fmt.Sprintf("%s=%s", constants.KubeConfigEnvVar, r.TempKubeconfigFile),
		// Set OKTETO_WITHIN_DEPLOY_COMMAND_CONTEXT env variable, so all okteto commands ran inside this deploy
		// know they are running inside another okteto deploy
		fmt.Sprintf("%s=true", constants.OktetoWithinDeployCommandContextEnvVar),
		// Set OKTETO_SKIP_CONFIG_CREDENTIALS_UPDATE env variable, so all the Okteto commands executed within this command execution
		// should not overwrite the server and the credentials in the kubeconfig
		fmt.Sprintf("%s=true", constants.OktetoSkipConfigCredentialsUpdate),
		// Set OKTETO_DISABLE_SPINNER=true env variable, so all the Okteto commands disable spinner which leads to errors
		fmt.Sprintf("%s=true", oktetoLog.OktetoDisableSpinnerEnvVar),
		// Set OKTETO_NAMESPACE=namespace-name env variable, so all the commandsruns on the same namespace
		fmt.Sprintf("%s=%s", model.OktetoNamespaceEnvVar, okteto.GetContext().Namespace),
		// Set OKTETO_AUTODISCOVERY_RELEASE_NAME=sanitized name, so the release name in case of autodiscovery of helm is valid
		fmt.Sprintf("%s=%s", constants.OktetoAutodiscoveryReleaseName, format.ResourceK8sMetaString(params.Name)),
	)
	if okteto.IsOkteto() {
		params.Variables = append(
			params.Variables,
			// Set OKTETO_DOMAIN=okteto-subdomain env variable
			fmt.Sprintf("%s=%s", model.OktetoDomainEnvVar, okteto.GetSubdomain()),
		)
	}
	oktetoLog.EnableMasking()
	err = r.runCommandsSection(ctx, params)
	return err
}

// runCommandsSection runs the commands defined in the command section of the deployable entity
func (r *DeployRunner) runCommandsSection(ctx context.Context, params DeployParameters) error {
	oktetoEnvFile, unlinkEnv, err := createTempOktetoEnvFile(r.Fs)
	if err != nil {
		return err
	}

	defer unlinkEnv()

	envStepper := NewEnvStepper(oktetoEnvFile.Name())

	if len(params.Deployable.Commands) != 0 {
		startTime := time.Now()
		// deploy commands if any
		for _, command := range params.Deployable.Commands {
			oktetoLog.Information("Running '%s'", command.Name)
			oktetoLog.SetStage(command.Name)
			oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Executing command '%s'...", command.Name)

			err := r.Executor.Execute(command, params.Variables)
			if err != nil {
				elapsedTime := time.Since(startTime)
				if err := r.ConfigMapHandler.AddPhaseDuration(ctx, params.Name, params.Namespace, deployCommandsPhaseName, elapsedTime); err != nil {
					oktetoLog.Info("error adding phase to configmap: %s", err)
				}
				oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error executing command '%s': %s", command.Name, err.Error())
				return fmt.Errorf("error executing command '%s': %s", command.Name, err.Error())
			}
			oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Command '%s' successfully executed", command.Name)

			envsFromOktetoEnvFile, err := envStepper.Step()
			if err != nil {
				oktetoLog.Warning("no valid format used in the okteto env file: %s", err.Error())
			}

			// the variables in the $OKTETO_ENV file are added as environment variables
			// to the executor. If there is already a previously set value for that
			// variable, the executor will use in next command the last one added which
			// corresponds to those coming from $OKTETO_ENV.
			params.Variables = append(params.Variables, envsFromOktetoEnvFile...)
			oktetoLog.SetStage("")
			oktetoLog.SetLevel("")
		}
		elapsedTime := time.Since(startTime)
		if err := r.ConfigMapHandler.AddPhaseDuration(ctx, params.Name, params.Namespace, deployCommandsPhaseName, elapsedTime); err != nil {
			oktetoLog.Info("error adding phase to configmap: %s", err)
		}
	}
	err = r.ConfigMapHandler.UpdateEnvsFromCommands(ctx, params.Name, params.Namespace, params.Variables)
	if err != nil {
		return fmt.Errorf("could not update config map with environment variables: %w", err)
	}

	// deploy divert if any
	if params.Deployable.Divert != nil && params.Deployable.Divert.Namespace != params.Namespace {
		oktetoLog.SetStage("Deploy Divert")
		if err := r.DivertDeployer.Deploy(ctx); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error creating divert: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	// deploy externals if any
	if len(params.Deployable.External) > 0 {
		oktetoLog.SetStage("External configuration")
		if !okteto.IsOkteto() {
			oktetoLog.Warning("external resources cannot be deployed on a context not managed by okteto")
			return nil
		}

		if err := r.deployExternals(ctx, params, envStepper.Map()); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error deploying external resources: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	return nil
}

// deployExternals deploys the external resources defined in the deployable entity
func (r *DeployRunner) deployExternals(ctx context.Context, params DeployParameters, dynamicEnvs map[string]string) error {
	_, cfg, err := r.K8sClientProvider.ProvideWithLogger(kconfig.Get([]string{r.TempKubeconfigFile}), r.k8sLogger)
	if err != nil {
		return fmt.Errorf("error getting kubernetes client: %w", err)
	}
	control := r.GetExternalControl(cfg)

	for externalName, externalInfo := range params.Deployable.External {
		oktetoLog.Spinner(fmt.Sprintf("Deploying external resource '%s'...", externalName))
		oktetoLog.StartSpinner()
		defer oktetoLog.StopSpinner()
		if err := externalInfo.SetURLUsingEnvironFile(externalName, dynamicEnvs); err != nil {
			return err
		}

		ef := externalresource.ERFilesystemManager{
			Fs:               r.Fs,
			ExternalResource: *externalInfo,
		}

		err := ef.LoadMarkdownContent(params.ManifestPath)
		if err != nil {
			oktetoLog.Infof("error loading external resource %s: %s", externalName, err.Error())
		}

		err = control.Deploy(ctx, externalName, params.Namespace, externalInfo)
		if err != nil {
			return err
		}
	}

	return nil
}

// CleanUp cleans up the resources created by the runner to avoid to leave any temporal resource
func (r *DeployRunner) CleanUp(ctx context.Context, err error) {
	oktetoLog.Debugf("removing temporal kubeconfig file '%s'", r.TempKubeconfigFile)
	if err := os.Remove(r.TempKubeconfigFile); err != nil {
		oktetoLog.Infof("could not remove temporal kubeconfig file: %s", err)
	}

	oktetoLog.Debugf("stopping local server...")
	if err := r.Proxy.Shutdown(ctx); err != nil {
		oktetoLog.Infof("could not stop local server: %s", err)
	}
	if r.Executor != nil {
		r.Executor.CleanUp(err)
	}

	oktetoLog.Debugf("executed clean up completely")
}

// createTempOktetoEnvFile creates a temporal file use to store the environment variables
func createTempOktetoEnvFile(fs afero.Fs) (afero.File, func(), error) {
	oktetoEnvFileDir, err := afero.TempDir(fs, "", "")
	if err != nil {
		return nil, func() {}, err
	}

	oktetoEnvFile, err := fs.Create(filepath.Join(oktetoEnvFileDir, ".env"))
	if err != nil {
		return nil, func() {}, err
	}

	os.Setenv(constants.OktetoEnvFile, oktetoEnvFile.Name())
	oktetoLog.Debug(fmt.Sprintf("using %s as env file for deploy command", oktetoEnvFile.Name()))

	return oktetoEnvFile, func() {
		if err := fs.RemoveAll(filepath.Dir(oktetoEnvFile.Name())); err != nil {
			oktetoLog.Infof("error removing okteto env file dir: %s", err)
		}
	}, nil
}
