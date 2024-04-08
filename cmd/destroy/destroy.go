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

package destroy

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	contextCMD "github.com/okteto/okteto/cmd/context"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/deployable"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/kubeconfig"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/types"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/client-go/kubernetes"
)

const (
	ownerLabel           = "owner"
	nameLabel            = "name"
	helmOwner            = "helm"
	helmUninstallCommand = "helm uninstall %s"
)

type destroyer interface {
	DestroyWithLabel(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
	DestroySFSVolumes(ctx context.Context, ns string, opts namespaces.DeleteAllOptions) error
}

type secretHandler interface {
	List(ctx context.Context, ns, labelSelector string) ([]v1.Secret, error)
}

// pipelineDestroyer interface with the operations to destroy a pipeline
type pipelineDestroyer interface {
	ExecuteDestroyPipeline(ctx context.Context, opts *pipelineCMD.DestroyOptions) error
}

type pipelineDestroyerProvider func() (pipelineDestroyer, error)

// divertProvider is a function that returns a divert driver
type divertProvider func(divert *model.DivertDeploy, name, namespace string, c kubernetes.Interface) (divert.Driver, error)

// Options represents the options for destroy command
type Options struct {
	Manifest *model.Manifest
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the path to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath        string
	Name                string
	Namespace           string
	K8sContext          string
	Variables           []string
	DestroyVolumes      bool
	DestroyDependencies bool
	ForceDestroy        bool
	RunWithoutBash      bool
	DestroyAll          bool
	RunInRemote         bool
}

type destroyInterface interface {
	Destroy(context.Context, *Options) error
}

type analyticsTrackerInterface interface {
	buildTrackerInterface
	TrackDestroy(metadta analytics.DestroyMetadata)
}

type buildTrackerInterface interface {
	TrackImageBuild(context.Context, *analytics.ImageBuildMetadata)
}

type destroyCommand struct {
	executor             executor.ManifestExecutor
	nsDestroyer          destroyer
	secrets              secretHandler
	k8sClientProvider    okteto.K8sClientProvider
	ConfigMapHandler     configMapHandler
	analyticsTracker     analyticsTrackerInterface
	getManifest          func(path string, fs afero.Fs) (*model.Manifest, error)
	oktetoClient         *okteto.Client
	ioCtrl               *io.Controller
	getDivertDriver      divertProvider
	getPipelineDestroyer pipelineDestroyerProvider
	buildCtrl            buildCtrl
}

// Destroy destroys the dev application defined by the manifest
func Destroy(ctx context.Context, at analyticsTrackerInterface, insights buildTrackerInterface, ioCtrl *io.Controller, k8sLogger *io.K8sLogger) *cobra.Command {
	options := &Options{
		Variables: []string{},
	}

	cmd := &cobra.Command{
		Use:   "destroy",
		Short: `Destroy everything created by the 'okteto deploy' command`,
		Long:  `Destroy everything created by the 'okteto deploy' command. You can also include a 'destroy' section in your okteto manifest with a list of custom commands to be executed on destroy`,
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/okteto-cli/#destroy"),
		RunE: func(cmd *cobra.Command, args []string) error {
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
			if err := contextCMD.LoadContextFromPath(ctx, options.Namespace, options.K8sContext, options.ManifestPath, contextCMD.Options{Show: true}); err != nil {
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.CloudURL).Error() {
					return err
				}
				if err := contextCMD.NewContextCommand().Run(ctx, &contextCMD.Options{Namespace: options.Namespace}); err != nil {
					return err
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get the current working directory: %w", err)
			}

			if options.Name == "" {
				c, _, err := okteto.NewK8sClientProviderWithLogger(k8sLogger).ProvideWithLogger(okteto.GetContext().Cfg, k8sLogger)
				if err != nil {
					return err
				}
				inferer := devenvironment.NewNameInferer(c)
				options.Name = inferer.InferName(ctx, cwd, okteto.GetContext().Namespace, options.ManifestPathFlag)
				if err != nil {
					return fmt.Errorf("could not infer environment name")
				}
			}

			dynClient, _, err := okteto.GetDynamicClient()
			if err != nil {
				return err
			}
			discClient, _, err := okteto.GetDiscoveryClient()
			if err != nil {
				return err
			}
			k8sClient, cfg, err := okteto.GetK8sClientWithLogger(k8sLogger)
			if err != nil {
				return err
			}

			if options.Namespace == "" {
				options.Namespace = okteto.GetContext().Namespace
			}

			var okClient = &okteto.Client{}
			if okteto.GetContext().IsOkteto {
				okClient, err = okteto.NewOktetoClient()
				if err != nil {
					return err
				}
			}
			c := &destroyCommand{
				executor:          executor.NewExecutor(oktetoLog.GetOutputFormat(), options.RunWithoutBash, ""),
				ConfigMapHandler:  NewConfigmapHandler(k8sClient),
				nsDestroyer:       namespaces.NewNamespace(dynClient, discClient, cfg, k8sClient),
				secrets:           secrets.NewSecrets(k8sClient),
				k8sClientProvider: okteto.NewK8sClientProviderWithLogger(k8sLogger),
				oktetoClient:      okClient,
				buildCtrl:         newBuildCtrl(options.Name, at, insights, ioCtrl),
				analyticsTracker:  at,
				getManifest:       model.GetManifestV2,
				ioCtrl:            ioCtrl,
				getDivertDriver:   divert.New,
				getPipelineDestroyer: func() (pipelineDestroyer, error) {
					return pipelineCMD.NewCommand()
				},
			}

			// We need to create a custom kubeconfig file to avoid to modify the user's kubeconfig when running the
			// destroy operation locally. This kubeconfig contains the kubernetes configuration got from the okteto
			// context
			kubeconfigPath := getTempKubeConfigFile(options.Name)
			if err := kubeconfig.Write(okteto.GetContext().Cfg, kubeconfigPath); err != nil {
				return err
			}
			os.Setenv("KUBECONFIG", kubeconfigPath)
			defer os.Remove(kubeconfigPath)

			return c.runDestroy(ctx, options)
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", "", "development environment name")
	cmd.Flags().StringVarP(&options.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().BoolVarP(&options.DestroyVolumes, "volumes", "v", false, "remove persistent volumes")
	cmd.Flags().BoolVarP(&options.DestroyDependencies, "dependencies", "d", false, "destroy dependencies")
	cmd.Flags().BoolVar(&options.ForceDestroy, "force-destroy", false, "forces the development environment to be destroyed even if there is an error executing the custom destroy commands defined in the manifest")
	cmd.Flags().StringVarP(&options.Namespace, "namespace", "n", "", "overwrites the namespace where the development environment was deployed")
	cmd.Flags().StringVarP(&options.K8sContext, "context", "c", "", "context where the development environment was deployed")
	cmd.Flags().BoolVarP(&options.RunWithoutBash, "no-bash", "", false, "execute commands without bash")
	cmd.Flags().BoolVarP(&options.DestroyAll, "all", "", false, "destroy everything in the namespace")
	cmd.Flags().BoolVarP(&options.RunInRemote, "remote", "", false, "force run destroy commands in remote")

	return cmd
}

// getTempKubeConfigFile creates the temporal kubernetes config file needed to avoid to modify the user's kubeconfig
func getTempKubeConfigFile(name string) string {
	tempKubeconfigFileName := fmt.Sprintf("kubeconfig-destroy-%s-%d", name, time.Now().UnixMilli())
	return filepath.Join(config.GetOktetoHome(), tempKubeconfigFileName)
}

func shouldRunInRemote(opts *Options) bool {
	// already in remote so we need to deploy locally
	if env.LoadBoolean(constants.OktetoDeployRemote) {
		return false
	}

	// --remote flag enabled from command line
	if opts.RunInRemote {
		return true
	}

	//  remote option set in the manifest via a remote destroyer image or the remote option enabled
	if opts.Manifest != nil && opts.Manifest.Destroy != nil {
		if opts.Manifest.Destroy.Image != "" || opts.Manifest.Destroy.Remote {
			return true
		}
	}

	if env.LoadBoolean(constants.OktetoForceRemote) {
		return true
	}

	return false
}

// runDestroy runs the main logic of the destroy command
func (dc *destroyCommand) runDestroy(ctx context.Context, opts *Options) error {
	var err error
	isDestroyAll := false
	isRemote := false
	if opts.DestroyAll {
		isDestroyAll = true
		err = dc.destroyAll(ctx, opts)

	} else {
		// normal Destroy
		err = dc.destroy(ctx, opts)

		// Execute after the destroy function as the opts already has the manifest information to calculate it.
		isRemote = shouldRunInRemote(opts)
		if err == nil {
			if opts.Name == "" {
				oktetoLog.Success("Development environment successfully destroyed")
			} else {
				oktetoLog.Success("Development environment '%s' successfully destroyed", opts.Name)
			}
		}
	}
	metadata := &analytics.DestroyMetadata{
		Success:      err == nil,
		IsDestroyAll: isDestroyAll,
		IsRemote:     isRemote,
	}
	dc.analyticsTracker.TrackDestroy(*metadata)

	return err
}

// destroyAll executes the logic to destroy all resources within a namespace. It is different from
// the dev environment destruction
func (dc *destroyCommand) destroyAll(ctx context.Context, opts *Options) error {
	if !okteto.GetContext().IsOkteto {
		return oktetoErrors.ErrContextIsNotOktetoCluster
	}
	destroyer := newLocalDestroyerAll(dc.k8sClientProvider, dc.oktetoClient)

	oktetoLog.Info("Destroying all...")

	return destroyer.destroy(ctx, opts)
}

// destroy runs the logic needed to destroy a dev environment
func (dc *destroyCommand) destroy(ctx context.Context, opts *Options) error {
	manifest, err := dc.getManifest(opts.ManifestPath, afero.NewOsFs())
	if err != nil {
		// Log error message but application can still be deleted
		oktetoLog.Infof("could not find manifest file to be executed: %s", err)
		manifest = &model.Manifest{
			Destroy: &model.DestroyInfo{},
		}
	}

	opts.Manifest = manifest
	if opts.Manifest.Destroy != nil {
		if opts.Name == "" {
			if opts.Manifest.Name == "" {
				opts.Manifest.Name = dc.buildCtrl.name
			}
		} else {
			opts.Manifest.Name = opts.Name
		}
		if err := dc.buildCtrl.buildImageIfNecessary(ctx, opts.Manifest); err != nil {
			return err
		}
		opts.Manifest.Destroy.Image, err = env.ExpandEnvIfNotEmpty(opts.Manifest.Destroy.Image)
		if err != nil {
			return err
		}
	}

	err = opts.Manifest.ExpandEnvVars()
	if err != nil {
		return err
	}

	namespace := opts.Namespace
	if namespace == "" {
		namespace = okteto.GetContext().Namespace
	}

	oktetoLog.AddToBuffer(oktetoLog.InfoLevel, "Destroying...")

	cfgVariablesString, err := dc.ConfigMapHandler.getConfigmapVariablesEncoded(ctx, opts.Name, namespace)
	if err != nil {
		return err
	}

	cfgVariables := types.DecodeStringToDeployVariable(cfgVariablesString)
	for _, variable := range cfgVariables {
		opts.Variables = append(opts.Variables, fmt.Sprintf("%s=%s", variable.Name, variable.Value))
		if strings.TrimSpace(variable.Value) != "" {
			oktetoLog.AddMaskedWord(variable.Value)
		}
	}
	oktetoLog.EnableMasking()

	// update to change status
	data := &pipeline.CfgData{
		Name:      opts.Name,
		Namespace: namespace,
		Status:    pipeline.DestroyingStatus,
		Filename:  opts.ManifestPathFlag,
		Variables: opts.Variables,
	}
	cfg, err := dc.ConfigMapHandler.translateConfigMapAndDeploy(ctx, data)
	if err != nil {
		return err
	}

	if opts.Manifest.Context == "" {
		opts.Manifest.Context = okteto.GetContext().Name
	}
	if opts.Manifest.Namespace == "" {
		opts.Manifest.Namespace = namespace
	}
	os.Setenv(constants.OktetoNameEnvVar, opts.Name)

	if opts.DestroyDependencies {
		if err := dc.destroyDependencies(ctx, opts); err != nil {
			if err := dc.ConfigMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
				return err
			}
			return err
		}
	}

	if hasDivert(opts.Manifest) {
		oktetoLog.SetStage("Destroy Divert")
		if err := dc.destroyDivert(ctx, opts.Manifest); err != nil {
			oktetoLog.AddToBuffer(oktetoLog.ErrorLevel, "error destroying divert: %s", err.Error())
			return err
		}
		oktetoLog.SetStage("")
	}

	var commandErr error
	// As the destroy only execute the commands within the destroy section, if there are no commands,
	// it should be executed
	if opts.Manifest.Destroy != nil && len(opts.Manifest.Destroy.Commands) > 0 {
		// call to specific Destroy logic
		destroyer := dc.getDestroyer(opts)
		if err := destroyer.Destroy(ctx, opts); err != nil {
			// If there was an interruption in the execution, or it was an error, but it wasn't a force Destroy
			// we have to change the status to err
			if errors.Is(err, oktetoErrors.ErrIntSig) || !opts.ForceDestroy {
				if err := dc.ConfigMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
					return err
				}

				return err
			}

			// We store the error returned by the execution if it is a force Destroy to return it at the end
			commandErr = err
		}
	}

	oktetoLog.SetStage("")
	oktetoLog.DisableMasking()

	oktetoLog.Spinner(fmt.Sprintf("Destroying development environment '%s'...", opts.Name))
	oktetoLog.StartSpinner()
	defer oktetoLog.StopSpinner()

	if err := dc.destroyK8sResources(ctx, opts); err != nil {
		if err := dc.ConfigMapHandler.setErrorStatus(ctx, cfg, data, err); err != nil {
			return err
		}

		return err
	}

	oktetoLog.SetStage("Destroying configmap")

	if err := dc.ConfigMapHandler.destroyConfigMap(ctx, cfg, namespace); err != nil {
		return err
	}

	return commandErr
}

func (dc *destroyCommand) destroyDependencies(ctx context.Context, opts *Options) error {
	for depName, depInfo := range opts.Manifest.Dependencies {
		oktetoLog.SetStage(fmt.Sprintf("Destroying dependency '%s'", depName))

		namespace := okteto.GetContext().Namespace
		if depInfo.Namespace != "" {
			namespace = depInfo.Namespace
		}

		destOpts := &pipelineCMD.DestroyOptions{
			Name:           depName,
			DestroyVolumes: opts.DestroyVolumes,
			Namespace:      namespace,
		}
		pipelineCmd, err := dc.getPipelineDestroyer()
		if err != nil {
			return err
		}
		if err := pipelineCmd.ExecuteDestroyPipeline(ctx, destOpts); err != nil {
			return err
		}
	}
	oktetoLog.SetStage("")
	return nil
}

func (dc *destroyCommand) destroyDivert(ctx context.Context, manifest *model.Manifest) error {
	stage := "Destroy Divert"
	oktetoLog.SetStage(stage)
	oktetoLog.Information("Running stage '%s'", stage)
	c, _, err := dc.k8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return err
	}
	driver, err := dc.getDivertDriver(manifest.Deploy.Divert, manifest.Name, manifest.Namespace, c)
	if err != nil {
		return err
	}

	return driver.Destroy(ctx)
}

func (dc *destroyCommand) destroyK8sResources(ctx context.Context, opts *Options) error {
	deployedByLs, err := labels.NewRequirement(
		model.DeployedByLabel,
		selection.Equals,
		[]string{format.ResourceK8sMetaString(opts.Name)},
	)
	if err != nil {
		return err
	}
	deployedBySelector := labels.NewSelector().Add(*deployedByLs).String()
	deleteOpts := namespaces.DeleteAllOptions{
		LabelSelector:  deployedBySelector,
		IncludeVolumes: opts.DestroyVolumes,
	}

	oktetoLog.SetStage("Destroying volumes")
	if err := dc.nsDestroyer.DestroySFSVolumes(ctx, opts.Namespace, deleteOpts); err != nil {
		return err
	}

	oktetoLog.SetStage("Destroying Helm release")
	if err := dc.destroyHelmReleasesIfPresent(ctx, opts, deployedBySelector); err != nil {
		if !opts.ForceDestroy {
			return err
		}
	}

	oktetoLog.Debugf("destroying resources with deployed-by label '%s'", deployedBySelector)
	oktetoLog.SetStage(fmt.Sprintf("Destroying by label '%s'", deployedBySelector))
	if err := dc.nsDestroyer.DestroyWithLabel(ctx, opts.Namespace, deleteOpts); err != nil {
		oktetoLog.Infof("could not delete all the resources: %s", err)
		return err
	}

	return nil
}

func (dc *destroyCommand) destroyHelmReleasesIfPresent(ctx context.Context, opts *Options, labelSelector string) error {
	sList, err := dc.secrets.List(ctx, opts.Namespace, labelSelector)
	if err != nil {
		return err
	}

	oktetoLog.Debugf("checking if application installed something with helm")
	helmReleases := map[string]bool{}
	for _, s := range sList {
		if s.Type == model.HelmSecretType && s.Labels[ownerLabel] == helmOwner {
			helmReleaseName, ok := s.Labels[nameLabel]
			if !ok {
				continue
			}

			helmReleases[helmReleaseName] = true
		}
	}

	// If the application to be destroyed was deployed with helm, we try to uninstall it to avoid to leave orphan release resources
	for releaseName := range helmReleases {
		oktetoLog.Debugf("uninstalling helm release '%s'", releaseName)
		cmd := fmt.Sprintf(helmUninstallCommand, releaseName)
		cmdInfo := model.DeployCommand{Command: cmd, Name: cmd}
		oktetoLog.Information("Running '%s'", cmdInfo.Name)
		if err := dc.executor.Execute(cmdInfo, opts.Variables); err != nil {
			oktetoLog.Infof("could not uninstall helm release '%s': %s", releaseName, err)
			if !opts.ForceDestroy {
				return err
			}
		}
	}

	return nil
}

func (dc *destroyCommand) getDestroyer(opts *Options) destroyInterface {
	var destroyer destroyInterface

	if shouldRunInRemote(opts) {
		destroyer = newRemoteDestroyer(opts.Manifest, dc.ioCtrl)
		oktetoLog.Info("Destroying remotely...")
	} else {
		runner := &deployable.DestroyRunner{
			Executor: dc.executor,
		}
		destroyer = newLocalDestroyer(runner)
		oktetoLog.Info("Destroying locally...")
	}

	return destroyer
}

func hasDivert(manifest *model.Manifest) bool {
	return manifest.Deploy != nil && manifest.Deploy.Divert != nil && manifest.Deploy.Divert.Namespace != manifest.Namespace
}
