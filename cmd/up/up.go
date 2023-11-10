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

package up

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/mitchellh/go-ps"
	"github.com/moby/term"
	buildv1 "github.com/okteto/okteto/cmd/build/v1"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/manifest"
	"github.com/okteto/okteto/cmd/namespace"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/discovery"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/okteto/okteto/pkg/types"

	"github.com/spf13/afero"
	"github.com/spf13/cobra"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const (
	ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

	composeVolumesUrl = "https://www.okteto.com/docs/reference/compose/#volumes-string-optional"
)

var (
	errConfigNotConfigured = fmt.Errorf("kubeconfig not found")
)

// UpOptions represents the options available on up command
type UpOptions struct {
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the path to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath     string
	Namespace        string
	K8sContext       string
	DevName          string
	Envs             []string
	Remote           int
	Deploy           bool
	ForcePull        bool
	Reset            bool
	commandToExecute []string
}

// Up starts a development container
func Up(at analyticsTrackerInterface) *cobra.Command {
	upOptions := &UpOptions{}
	cmd := &cobra.Command{
		Use:   "up [svc]",
		Short: "Launch your development environment",
		Args:  utils.MaximumNArgsAccepted(1, "https://okteto.com/docs/reference/cli/#up"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if okteto.InDevContainer() {
				return oktetoErrors.ErrNotInDevContainer
			}

			if err := upOptions.AddArgs(cmd, args); err != nil {
				return err
			}

			u := utils.UpgradeAvailable()
			if len(u) > 0 {
				warningFolder := filepath.Join(config.GetOktetoHome(), ".warnings")
				if utils.GetWarningState(warningFolder, "version") != u {
					oktetoLog.Yellow("Okteto %s is available. To upgrade:", u)
					oktetoLog.Yellow("    %s", utils.GetUpgradeCommand())
					if err := utils.SetWarningState(warningFolder, "version", u); err != nil {
						oktetoLog.Infof("failed to set warning version state: %s", err.Error())
					}
				}
			}

			checkLocalWatchesConfiguration()

			ctx := context.Background()

			upMeta := analytics.NewUpMetricsMetadata()

			// when cmd up finishes, send the event
			// metadata retrieved during the run of the cmd
			defer at.TrackUp(upMeta)

			startOkContextConfig := time.Now()
			if upOptions.ManifestPath != "" {
				// if path is absolute, its transformed to rel from root
				initialCWD, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("failed to get the current working directory: %w", err)
				}
				manifestPathFlag, err := oktetoPath.GetRelativePathFromCWD(initialCWD, upOptions.ManifestPath)
				if err != nil {
					return err
				}
				// as the installer uses root for executing the pipeline, we save the rel path from root as ManifestPathFlag option
				upOptions.ManifestPathFlag = manifestPathFlag

				// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
				uptManifestPath, err := model.UpdateCWDtoManifestPath(upOptions.ManifestPath)
				if err != nil {
					return err
				}
				upOptions.ManifestPath = uptManifestPath
			}
			manifestOpts := contextCMD.ManifestOptions{Filename: upOptions.ManifestPath, Namespace: upOptions.Namespace, K8sContext: upOptions.K8sContext}
			oktetoManifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				if err.Error() == fmt.Errorf(oktetoErrors.ErrNotLogged, okteto.CloudURL).Error() {
					return err
				}

				if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
					return err
				}

				if upOptions.ManifestPath == "" {
					upOptions.ManifestPath = utils.DefaultManifest
				}

				if !utils.AskIfOktetoInit(upOptions.ManifestPath) {
					return err
				}

				oktetoManifest, err = LoadManifestWithInit(ctx, upOptions.K8sContext, upOptions.Namespace, upOptions.ManifestPath)
				if err != nil {
					return err
				}
			}

			upMeta.OktetoContextConfig(time.Since(startOkContextConfig))
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

			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if oktetoManifest.Name == "" {
				oktetoLog.Info("okteto manifest doesn't have a name, inferring it...")
				c, _, err := okteto.NewK8sClientProvider().Provide(okteto.Context().Cfg)
				if err != nil {
					return err
				}
				inferer := devenvironment.NewNameInferer(c)
				oktetoManifest.Name = inferer.InferName(ctx, wd, okteto.Context().Namespace, upOptions.ManifestPathFlag)
			}
			os.Setenv(constants.OktetoNameEnvVar, oktetoManifest.Name)

			if len(oktetoManifest.Dev) == 0 {
				if oktetoManifest.Type == model.StackType {
					return fmt.Errorf("your docker compose file is not currently supported: Okteto requires a 'host volume' to be defined. See %s", composeVolumesUrl)
				}
				oktetoLog.Warning("okteto manifest has no 'dev' section.")
				answer, err := utils.AskYesNo("Do you want to configure okteto manifest now?", utils.YesNoDefault_Yes)
				if err != nil {
					return err
				}
				if answer {
					mc := &manifest.ManifestCommand{
						K8sClientProvider: okteto.NewK8sClientProvider(),
					}
					if upOptions.ManifestPath == "" {
						upOptions.ManifestPath = utils.DefaultManifest
					}
					oktetoManifest, err = mc.RunInitV2(ctx, &manifest.InitOpts{
						DevPath:          upOptions.ManifestPath,
						Namespace:        upOptions.Namespace,
						Context:          upOptions.K8sContext,
						ShowCTA:          false,
						Workdir:          wd,
						AutoDeploy:       true,
						AutoConfigureDev: true,
					})
					if err != nil {
						return err
					}
					if oktetoManifest.Namespace == "" {
						oktetoManifest.Namespace = okteto.Context().Namespace
					}
					if oktetoManifest.Context == "" {
						oktetoManifest.Context = okteto.Context().Name
					}
					oktetoManifest.IsV2 = true
					for devName, d := range oktetoManifest.Dev {
						if err := d.SetDefaults(); err != nil {
							return err
						}
						d.Name = devName
						d.Namespace = oktetoManifest.Namespace
						d.Context = oktetoManifest.Context
					}
				}
			}

			up := &upContext{
				Manifest:          oktetoManifest,
				Dev:               nil,
				Exit:              make(chan error, 1),
				resetSyncthing:    upOptions.Reset,
				StartTime:         time.Now(),
				Registry:          registry.NewOktetoRegistry(okteto.Config{}),
				Options:           upOptions,
				Fs:                afero.NewOsFs(),
				analyticsTracker:  at,
				analyticsMeta:     upMeta,
				K8sClientProvider: okteto.NewK8sClientProvider(),
				tokenUpdater:      newTokenUpdaterController(),
				builder:           buildv2.NewBuilderFromScratch(at),
			}
			up.inFd, up.isTerm = term.GetFdInfo(os.Stdin)
			if up.isTerm {
				var err error
				up.stateTerm, err = term.SaveState(up.inFd)
				if err != nil {
					oktetoLog.Infof("failed to save the state of the terminal: %s", err.Error())
					return fmt.Errorf("failed to save the state of the terminal")
				}
				oktetoLog.Infof("Terminal: %v", up.stateTerm)
			}

			k8sClient, _, err := okteto.GetK8sClient()
			if err != nil {
				return fmt.Errorf("failed to load k8s client: %v", err)
			}

			// if manifest v1 - either set autocreate: true or pass --deploy (okteto forces autocreate: true)
			// if manifest v2 - either set autocreate: true or pass --deploy with a deploy section at the manifest
			forceAutocreate := false
			if upOptions.Deploy && !up.Manifest.IsV2 {
				// the autocreate property is forced to be true
				forceAutocreate = true
			} else if upOptions.Deploy || (up.Manifest.IsV2 && !pipeline.IsDeployed(ctx, up.Manifest.Name, up.Manifest.Namespace, k8sClient)) {
				err := up.deployApp(ctx)

				// only allow error.ErrManifestFoundButNoDeployAndDependenciesCommands to go forward - autocreate property will deploy the app
				if err != nil && !errors.Is(err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands) {
					return err
				}

			} else if !upOptions.Deploy && (up.Manifest.IsV2 && pipeline.IsDeployed(ctx, up.Manifest.Name, up.Manifest.Namespace, k8sClient)) {
				oktetoLog.Information("'%s' was already deployed. To redeploy run 'okteto deploy' or 'okteto up --deploy'", up.Manifest.Name)
			}

			dev, err := utils.GetDevFromManifest(oktetoManifest, upOptions.DevName)
			if err != nil {
				if !errors.Is(err, utils.ErrNoDevSelected) {
					return err
				}
				selector := utils.NewOktetoSelector("Select which development container to activate:", "Development container")
				dev, err = utils.SelectDevFromManifest(oktetoManifest, selector, oktetoManifest.Dev.GetDevs())
				if err != nil {
					return err
				}
			}
			if len(upOptions.commandToExecute) > 0 {
				dev.Command.Values = upOptions.commandToExecute
			}

			up.Dev = dev
			if forceAutocreate {
				// update autocreate property if needed to be forced
				oktetoLog.Info("Setting Autocreate to true because manifest v1 and flag --deploy")
				up.Dev.Autocreate = true
			}

			// only if the context is an okteto one, we should verify if the namespace has to be woken up
			if okteto.Context().IsOkteto {
				// We execute it in a goroutine to not impact the command performance
				go func() {
					okClient, err := okteto.NewOktetoClient()
					if err != nil {
						oktetoLog.Infof("failed to create okteto client: '%s'", err.Error())
						return
					}
					if err := wakeNamespaceIfApplies(ctx, up.Dev.Namespace, k8sClient, okClient); err != nil {
						// If there is an error waking up namespace, we don't want to fail the up command
						oktetoLog.Infof("failed to wake up the namespace: %s", err.Error())
					}
				}()
			}

			// build images and set env vars for the services at the manifest
			if err := buildServicesAndSetBuildEnvs(ctx, oktetoManifest, up.builder); err != nil {
				return err
			}

			if err := loadManifestOverrides(dev, upOptions); err != nil {
				return err
			}

			if syncthing.ShouldUpgrade() {
				oktetoLog.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					oktetoLog.Infof("failed to upgrade syncthing: %s", err)

					if !syncthing.IsInstalled() {
						return fmt.Errorf("couldn't download syncthing, please try again")
					}

					oktetoLog.Yellow("couldn't upgrade syncthing, will try again later")
					oktetoLog.Println()
				} else {
					oktetoLog.Success("Dependencies successfully installed")
				}
			}

			oktetoLog.ConfigureFileLogger(config.GetAppHome(dev.Namespace, dev.Name), config.VersionString)

			if err := checkStignoreConfiguration(dev); err != nil {
				oktetoLog.Infof("failed to check '.stignore' configuration: %s", err.Error())
			}

			if err := addStignoreSecrets(dev); err != nil {
				return err
			}

			if err := addSyncFieldHash(dev); err != nil {
				return err
			}

			if err := setSyncDefaultsByDevMode(dev, up.getSyncTempDir); err != nil {
				return err
			}

			if _, ok := os.LookupEnv(model.OktetoAutoDeployEnvVar); ok {
				upOptions.Deploy = true
			}

			if up.Manifest.Type == model.OktetoManifestType && !up.Manifest.IsV2 {
				oktetoLog.Warning("okteto manifest v1 is deprecated and will be removed in okteto 3.0")
				oktetoLog.Println(oktetoLog.BlueString(`    Follow this guide to upgrade to the new okteto manifest schema:
    https://www.okteto.com/docs/reference/manifest-migration/`))
			}

			if err = up.start(); err != nil {
				switch err.(type) {
				default:
					return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
				case oktetoErrors.CommandError:
					oktetoLog.Infof("CommandError: %v", err)
					return err
				case oktetoErrors.UserError:
					return err
				}
			}

			up.analyticsMeta.CommandSuccess()
			return nil
		},
	}

	cmd.Flags().StringVarP(&upOptions.ManifestPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringVarP(&upOptions.Namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().StringVarP(&upOptions.K8sContext, "context", "c", "", "context where the up command is executed")
	cmd.Flags().StringArrayVarP(&upOptions.Envs, "env", "e", []string{}, "envs to add to the development container")
	cmd.Flags().IntVarP(&upOptions.Remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&upOptions.Deploy, "deploy", "d", false, "Force execution of the commands in the 'deploy' section of the okteto manifest (defaults to 'false')")
	cmd.Flags().BoolVarP(&upOptions.ForcePull, "pull", "", false, "force dev image pull")
	if err := cmd.Flags().MarkHidden("pull"); err != nil {
		oktetoLog.Infof("failed to mark 'pull' flag as hidden: %s", err)
	}
	cmd.Flags().BoolVarP(&upOptions.Reset, "reset", "", false, "reset the file synchronization database")
	cmd.Flags().StringArrayVarP(&upOptions.commandToExecute, "command", "", []string{}, "external commands to be supplied to 'okteto up'")
	return cmd
}

// AddArgs sets the args as options and return err if it's not compatible
func (o *UpOptions) AddArgs(cmd *cobra.Command, args []string) error {

	maxV1Args := 1
	docsURL := "https://okteto.com/docs/reference/cli/#up"
	if len(args) > maxV1Args {
		if err := cmd.Help(); err != nil {
			oktetoLog.Infof("could not show help: %s", err)
		}

		return oktetoErrors.UserError{
			E:    fmt.Errorf("%q accepts at most %d arg(s), but received %d", cmd.CommandPath(), maxV1Args, len(args)),
			Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
		}
	} else if len(args) == 1 {
		o.DevName = args[0]
	}

	return nil
}

func LoadManifestWithInit(ctx context.Context, k8sContext, namespace, devPath string) (*model.Manifest, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ctxOptions := &contextCMD.ContextOptions{
		Context:   k8sContext,
		Namespace: namespace,
		Show:      true,
	}
	if err := contextCMD.NewContextCommand().Run(ctx, ctxOptions); err != nil {
		return nil, err
	}

	mc := &manifest.ManifestCommand{
		K8sClientProvider: okteto.NewK8sClientProvider(),
	}
	manifest, err := mc.RunInitV2(ctx, &manifest.InitOpts{DevPath: devPath, ShowCTA: false, Workdir: dir})
	if err != nil {
		return nil, err
	}

	if manifest.Namespace == "" {
		manifest.Namespace = okteto.Context().Namespace
	}
	if manifest.Context == "" {
		manifest.Context = okteto.Context().Name
	}
	manifest.IsV2 = true
	for devName, d := range manifest.Dev {
		if err := d.SetDefaults(); err != nil {
			return nil, err
		}
		d.Name = devName
		d.Namespace = manifest.Namespace
		d.Context = manifest.Context
	}

	return manifest, nil
}

func loadManifestOverrides(dev *model.Dev, upOptions *UpOptions) error {
	if upOptions.Remote > 0 {
		dev.RemotePort = upOptions.Remote
	}

	if dev.RemoteModeEnabled() {
		if err := sshKeys(); err != nil {
			return err
		}

		dev.LoadRemote(ssh.GetPublicKey())
	}

	if upOptions.ForcePull {
		dev.LoadForcePull()
	}

	if len(upOptions.Envs) > 0 {
		overridedEnvVars, err := getOverridedEnvVarsFromCmd(dev.Environment, upOptions.Envs)
		if err != nil {
			return err
		} else {
			dev.Environment = *overridedEnvVars
		}
	}

	dev.Username = okteto.Context().Username
	dev.RegistryURL = okteto.Context().Registry

	return nil
}

func setSyncDefaultsByDevMode(dev *model.Dev, getSyncTempDir func() (string, error)) error {
	if dev.IsHybridModeEnabled() {
		syncTempDir, err := getSyncTempDir()
		if err != nil || syncTempDir == "" {
			return err
		}

		dev.PersistentVolumeInfo.Enabled = false
		dev.Sync.Folders = []model.SyncFolder{
			{
				LocalPath:  syncTempDir,
				RemotePath: "/okteto",
			},
		}
	}
	return nil
}

func getOverridedEnvVarsFromCmd(manifestEnvVars model.Environment, commandEnvVariables []string) (*model.Environment, error) {
	envVarsToValues := make(map[string]string)
	for _, manifestEnv := range manifestEnvVars {
		envVarsToValues[manifestEnv.Name] = manifestEnv.Value
	}

	for _, v := range commandEnvVariables {
		kv := strings.SplitN(v, "=", 2)
		if len(kv) != 2 {
			if kv[0] == "" {
				return nil, fmt.Errorf("invalid variable value '%s': please review the accepted formats at https://www.okteto.com/docs/reference/manifest/#environment-string-optional ", v)
			}
			kv = append(kv, os.Getenv(kv[0]))
		}

		varNameToAdd, varValueToAdd := kv[0], kv[1]
		if strings.HasPrefix(varNameToAdd, "OKTETO_") || varNameToAdd == model.OktetoBuildkitHostURLEnvVar {
			return nil, oktetoErrors.ErrBuiltInOktetoEnvVarSetFromCMD
		}

		expandedEnv, err := model.ExpandEnv(varValueToAdd, true)
		if err != nil {
			return nil, err
		}

		envVarsToValues[varNameToAdd] = expandedEnv
	}

	overridedEnvVars := model.Environment{}
	for k, v := range envVarsToValues {
		overridedEnvVars = append(overridedEnvVars, model.EnvVar{Name: k, Value: v})
	}

	return &overridedEnvVars, nil
}

func (up *upContext) deployApp(ctx context.Context) error {
	k8sProvider := okteto.NewK8sClientProvider()
	pc, err := pipelineCMD.NewCommand()
	if err != nil {
		return err
	}
	k8sClientProvider := okteto.NewK8sClientProvider()
	c := &deploy.DeployCommand{
		GetManifest:        up.getManifest,
		GetDeployer:        deploy.GetDeployer,
		TempKubeconfigFile: deploy.GetTempKubeConfigFile(up.Manifest.Name),
		K8sClientProvider:  k8sClientProvider,
		Builder:            up.builder,
		GetExternalControl: deploy.NewDeployExternalK8sControl,
		Fs:                 up.Fs,
		CfgMapHandler:      deploy.NewConfigmapHandler(k8sProvider),
		PipelineCMD:        pc,
		DeployWaiter:       deploy.NewDeployWaiter(k8sClientProvider),
		EndpointGetter:     deploy.NewEndpointGetter,
		AnalyticsTracker:   up.analyticsTracker,
	}

	startTime := time.Now()
	err = c.RunDeploy(ctx, &deploy.Options{
		Name:             up.Manifest.Name,
		ManifestPathFlag: up.Options.ManifestPathFlag,
		ManifestPath:     up.Options.ManifestPath,
		Timeout:          5 * time.Minute,
		Build:            false,
	})
	up.analyticsMeta.HasRunDeploy()

	isRemote := false
	if up.Manifest.Deploy != nil {
		isRemote = up.Manifest.Deploy.Image != ""
	}

	// tracking deploy either its been successful or not
	c.AnalyticsTracker.TrackDeploy(analytics.DeployMetadata{
		Success:                err == nil,
		IsOktetoRepo:           utils.IsOktetoRepo(),
		Duration:               time.Since(startTime),
		PipelineType:           up.Manifest.Type,
		DeployType:             "automatic",
		IsPreview:              os.Getenv(model.OktetoCurrentDeployBelongsToPreview) == "true",
		HasDependenciesSection: up.Manifest.HasDependenciesSection(),
		HasBuildSection:        up.Manifest.HasBuildSection(),
		Err:                    err,
		IsRemote:               up.Manifest.IsV2 && isRemote,
	})
	return err
}

func (up *upContext) getManifest(path string) (*model.Manifest, error) {
	if up.Manifest != nil {
		return up.Manifest, nil
	}
	return model.GetManifestV2(path)
}

func (up *upContext) start() error {
	up.pidController = newPIDController(up.Dev.Namespace, up.Dev.Name)

	if err := up.pidController.create(); err != nil {
		oktetoLog.Infof("failed to create pid file for %s - %s: %s", up.Dev.Namespace, up.Dev.Name, err)

		return oktetoErrors.UserError{
			E: fmt.Errorf("couldn't create a pid file for %s - %s", up.Dev.Namespace, up.Dev.Name),
			Hint: `This error can occur if the ".okteto" folder in your home has misconfigured permissions.
    To resolve, try 'sudo chown -R <your-user>: ~/.okteto'

    Alternatively, check the permissions of that directory and its content to ensure your user has write permissions.`,
		}
	}

	defer up.pidController.delete()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	pidFileCh := make(chan error, 1)

	up.analyticsMeta.ManifestProps(up.Manifest)
	up.analyticsMeta.DevProps(up.Dev)
	up.analyticsMeta.RepositoryProps(utils.IsOktetoRepo())

	go up.activateLoop()

	go up.pidController.notifyIfPIDFileChange(pidFileCh)

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		up.interruptReceived = true
		up.shutdown()
		oktetoLog.Println()
	case err := <-up.Exit:
		if up.Dev.IsHybridModeEnabled() {
			up.shutdownHybridMode()
		}
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	case err := <-pidFileCh:
		if up.Dev.IsHybridModeEnabled() {
			up.shutdownHybridMode()
		}
		oktetoLog.Infof("exit signal received due to pid file modification: %s", err)
		return err
	}
	return nil
}

// activateLoop activates the development container in a retry loop
func (up *upContext) activateLoop() {
	isTransientError := false
	t := time.NewTicker(1 * time.Second)
	iter := 0
	defer t.Stop()

	defer func() {
		if err := config.DeleteStateFile(up.Dev.Name, up.Dev.Namespace); err != nil {
			oktetoLog.Infof("failed to delete state file: %s", err)
		}
	}()
	for {
		if up.isRetry || isTransientError {
			oktetoLog.Infof("waiting for shutdown sequence to finish")
			<-up.ShutdownCompleted
			pidFromFile, err := up.pidController.get()
			if err != nil {
				oktetoLog.Infof("error getting pid: %w")
			}
			if pidFromFile != strconv.Itoa(os.Getpid()) {
				if up.Dev.IsHybridModeEnabled() {
					up.shutdownHybridMode()
				}
				up.Exit <- oktetoErrors.UserError{
					E:    fmt.Errorf("development container has been deactivated by another 'okteto up' command"),
					Hint: "Use 'okteto exec' to open another terminal to your development container",
				}
				return
			}
			if iter == 0 {
				oktetoLog.Yellow("Connection lost to your development container, reconnecting...")
			}
			iter++
			iter = iter % 10
			if isTransientError {
				<-t.C
			}
		}

		err := up.activate()
		if err != nil {
			oktetoLog.Infof("activate failed with: %s", err)

			if err == oktetoErrors.ErrLostSyncthing {
				isTransientError = false
				iter = 0
				continue
			}

			if errors.Is(err, okteto.ErrK8sUnauthorised) {
				oktetoLog.Info("updating kubeconfig token")
				if err := up.tokenUpdater.UpdateKubeConfigToken(); err != nil {
					up.Exit <- fmt.Errorf("error updating k8s token: %w", err)
					return
				}
				continue
			}

			if oktetoErrors.IsTransient(err) {
				isTransientError = true
				continue
			}

			up.Exit <- err
			return
		}
		up.Exit <- nil
		return
	}
}

// waitUntilExitOrInterruptOrApply blocks execution until a stop signal is sent, a disconnect event or an error or the app is modify
func (up *upContext) waitUntilExitOrInterruptOrApply(ctx context.Context) error {
	for {
		select {
		case err := <-up.CommandResult:
			oktetoLog.Println()
			if err != nil {
				oktetoLog.Infof("command failed: %s", err)
				if oktetoErrors.IsTransient(err) {
					return err
				}
				return oktetoErrors.CommandError{
					E:      oktetoErrors.ErrCommandFailed,
					Reason: err,
				}
			}

			oktetoLog.Info("command completed")
			return nil

		case err := <-up.Disconnect:
			if err == oktetoErrors.ErrInsufficientSpace {
				return up.getInsufficientSpaceError(err)
			}
			return err

		case err := <-up.GlobalForwarderStatus:
			oktetoLog.Infof("exiting by error in global forward checker: %v", err)
			return err

		case err := <-up.applyToApps(ctx):
			oktetoLog.Infof("exiting by applyToAppsChan: %v", err)
			return err
		}
	}
}

func (up *upContext) applyToApps(ctx context.Context) chan error {
	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.Context().Cfg)
	if err != nil {
		return nil
	}
	result := make(chan error, 1)
	for _, tr := range up.Translations {
		go tr.App.Watch(ctx, result, k8sClient)
	}
	return result
}

func (up *upContext) buildDevImage(ctx context.Context, app apps.App) error {
	dockerfile := up.Dev.Image.Dockerfile
	image := up.Dev.Image.Name
	args := up.Dev.Image.Args
	context := up.Dev.Image.Context
	target := up.Dev.Image.Target
	cacheFrom := up.Dev.Image.CacheFrom
	if v, ok := up.Manifest.Build[up.Dev.Name]; up.Manifest.IsV2 && ok {
		dockerfile = v.Dockerfile
		image = v.Image
		args = v.Args
		context = v.Context
		target = v.Target
		cacheFrom = v.CacheFrom
		if image != "" {
			up.Dev.EmptyImage = false
		}
	}

	if _, err := os.Stat(up.Dev.Image.GetDockerfilePath()); err != nil {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("'--build' argument given but there is no Dockerfile"),
			Hint: "Try creating a Dockerfile file or specify the 'context' and 'dockerfile' fields in your okteto manifest.",
		}
	}

	oktetoRegistryURL := okteto.Context().Registry
	if oktetoRegistryURL == "" && up.Dev.Autocreate && image == "" {
		return fmt.Errorf("no value for 'image' has been provided in your okteto manifest")
	}

	if image == "" {
		devContainer := apps.GetDevContainer(app.PodSpec(), up.Dev.Container)
		if devContainer == nil {
			return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
		}
		image = devContainer.Image
	}

	oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)

	imageTag := up.Registry.GetImageTag(image, up.Dev.Name, up.Dev.Namespace)
	oktetoLog.Infof("building dev image tag %s", imageTag)

	buildArgs := model.SerializeBuildArgs(args)

	buildOptions := &types.BuildOptions{
		Path:       context,
		File:       dockerfile,
		Tag:        imageTag,
		Target:     target,
		CacheFrom:  cacheFrom,
		BuildArgs:  buildArgs,
		OutputMode: oktetoLog.TTYFormat,
	}
	builder := buildv1.NewBuilderFromScratch()
	if err := builder.Build(ctx, buildOptions); err != nil {
		return err
	}
	for _, s := range up.Dev.Services {
		if s.Image.Name == up.Dev.Image.Name {
			s.Image.Name = imageTag
			s.SetLastBuiltAnnotation()
		}
	}
	up.Dev.Image.Name = imageTag
	up.Dev.SetLastBuiltAnnotation()
	return nil
}

func (up *upContext) setDevContainer(app apps.App) error {
	devContainer := apps.GetDevContainer(app.PodSpec(), up.Dev.Container)
	if devContainer == nil {
		return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
	}

	up.Dev.Container = devContainer.Name

	if up.Dev.Image.Name == "" {
		up.Dev.Image.Name = devContainer.Image
	}

	return nil
}

func (up *upContext) getInsufficientSpaceError(err error) error {
	if up.Dev.PersistentVolumeEnabled() {

		return oktetoErrors.UserError{
			E: err,
			Hint: fmt.Sprintf(`Okteto volume is full.
    Increase your persistent volume size, run '%s' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`, utils.GetDownCommand(up.Options.ManifestPathFlag)),
		}
	}
	return oktetoErrors.UserError{
		E: err,
		Hint: `The synchronization service is running out of space.
    Enable persistent volumes in your okteto manifest and try again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`,
	}

}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *upContext) shutdown() {
	if up.isTerm {
		if err := term.RestoreTerminal(up.inFd, up.stateTerm); err != nil {
			oktetoLog.Infof("failed to restore terminal: %s", err.Error())
		}

		oktetoLog.StopSpinner()
	}

	oktetoLog.Infof("starting shutdown sequence")
	if !up.success {
		up.analyticsMeta.FailActivate()
	}

	if up.Cancel != nil {
		up.Cancel()
		oktetoLog.Info("sent cancellation signal")
	}

	if up.Sy != nil {
		oktetoLog.Infof("stopping syncthing")
		if err := up.Sy.SoftTerminate(); err != nil {
			oktetoLog.Infof("failed to stop syncthing during shutdown: %s", err.Error())
		}
	}

	oktetoLog.Infof("stopping forwarders")
	if up.Forwarder != nil {
		up.Forwarder.Stop()
	}

	if up.Dev.IsHybridModeEnabled() {
		oktetoLog.Infof("stopping local process...")
		up.shutdownHybridMode()
	}

	oktetoLog.Info("completed shutdown sequence")
	up.ShutdownCompleted <- true

}

func (up *upContext) shutdownHybridMode() {
	if up.hybridCommand == nil {
		return
	}
	if up.hybridCommand.Process == nil {
		return
	}

	pList, err := ps.Processes()
	if err != nil {
		oktetoLog.Warning("error getting list of processes %v", err)
		return
	}

	terminateChildProcess(up.hybridCommand.Process.Pid, pList)

	if err := terminateProcess(up.hybridCommand.Process.Pid); err != nil {
		oktetoLog.Debugf("error terminating process %s: %v", up.hybridCommand.Process.Pid, err)
	}
}

func terminateChildProcess(parent int, pList []ps.Process) {
	// assure all the child processes are terminated when command is exited
	for _, pR := range pList {
		// skip when process is not child from parent
		if pR.PPid() != parent {
			continue
		}
		// iterate over the children of the parent
		terminateChildProcess(pR.Pid(), pList)

		if err := terminateProcess(pR.Pid()); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				continue
			}
			oktetoLog.Debugf("error terminating process %s: %v", pR.Pid(), err)
		}
	}
}

func terminateProcess(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		oktetoLog.Debugf("error getting process %s: %v", pid, err)
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		oktetoLog.Debugf("error terminating process %s: %v", p.Pid, err)
		return err
	}
	if _, err := p.Wait(); err != nil {
		oktetoLog.Debugf("error waiting for process to exit %s: %v", p.Pid, err)
		return err
	}
	return nil
}

func printDisplayContext(up *upContext) {
	oktetoLog.Println(fmt.Sprintf("    %s   %s", oktetoLog.BlueString("Context:"), okteto.RemoveSchema(up.Dev.Context)))
	oktetoLog.Println(fmt.Sprintf("    %s %s", oktetoLog.BlueString("Namespace:"), up.Dev.Namespace))
	oktetoLog.Println(fmt.Sprintf("    %s      %s", oktetoLog.BlueString("Name:"), up.Dev.Name))

	anyGlobalForward := false
	if len(up.Manifest.GlobalForward) > 0 {
		anyGlobalForward = true

		oktetoLog.Println(fmt.Sprintf("    %s   %d -> %s:%d", oktetoLog.BlueString("Forward:"), up.Manifest.GlobalForward[0].Local, up.Manifest.GlobalForward[0].ServiceName, up.Manifest.GlobalForward[0].Remote))

		for i := 1; i < len(up.Manifest.GlobalForward); i++ {
			oktetoLog.Println(fmt.Sprintf("               %d -> %s:%d", up.Manifest.GlobalForward[i].Local, up.Manifest.GlobalForward[i].ServiceName, up.Manifest.GlobalForward[i].Remote))
		}
	}

	if len(up.Dev.Forward) > 0 {

		fromIdxToShowWithoutForwardLabel := 0
		if !anyGlobalForward {
			fromIdxToShowWithoutForwardLabel = 1
			if up.Dev.Forward[0].Service {
				oktetoLog.Println(fmt.Sprintf("    %s   %d -> %s:%d", oktetoLog.BlueString("Forward:"), up.Dev.Forward[0].Local, up.Dev.Forward[0].ServiceName, up.Dev.Forward[0].Remote))
			} else {
				oktetoLog.Println(fmt.Sprintf("    %s   %d -> %d", oktetoLog.BlueString("Forward:"), up.Dev.Forward[0].Local, up.Dev.Forward[0].Remote))
			}
		}

		for i := fromIdxToShowWithoutForwardLabel; i < len(up.Dev.Forward); i++ {
			if up.Dev.Forward[i].Service {
				oktetoLog.Println(fmt.Sprintf("               %d -> %s:%d", up.Dev.Forward[i].Local, up.Dev.Forward[i].ServiceName, up.Dev.Forward[i].Remote))
				continue
			}
			oktetoLog.Println(fmt.Sprintf("               %d -> %d", up.Dev.Forward[i].Local, up.Dev.Forward[i].Remote))
		}
	}

	if len(up.Dev.Reverse) > 0 {
		oktetoLog.Println(fmt.Sprintf("    %s   %d <- %d", oktetoLog.BlueString("Reverse:"), up.Dev.Reverse[0].Local, up.Dev.Reverse[0].Remote))
		for i := 1; i < len(up.Dev.Reverse); i++ {
			oktetoLog.Println(fmt.Sprintf("               %d <- %d", up.Dev.Reverse[i].Local, up.Dev.Reverse[i].Remote))
		}
	}

	oktetoLog.Println()
}

// buildServicesAndSetBuildEnvs get services to build and run build to set build envs
func buildServicesAndSetBuildEnvs(ctx context.Context, m *model.Manifest, builder builderInterface) error {
	svcsToBuild, err := builder.GetServicesToBuild(ctx, m, []string{})
	if err != nil {
		return err
	}
	if len(svcsToBuild) == 0 {
		return nil
	}
	buildOptions := &types.BuildOptions{
		CommandArgs: svcsToBuild,
		Manifest:    m,
	}
	return builder.Build(ctx, buildOptions)
}

// wakeNamespaceIfApplies wakes the namespace if it is sleeping
func wakeNamespaceIfApplies(ctx context.Context, ns string, k8sClient kubernetes.Interface, okClient types.OktetoInterface) error {
	n, err := k8sClient.CoreV1().Namespaces().Get(ctx, ns, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// If the namespace is not sleeping, do nothing
	if n.Labels[constants.NamespaceStatusLabel] != constants.NamespaceStatusSleeping {
		return nil
	}

	oktetoLog.Information("Namespace '%s' is sleeping, waking it up...", ns)
	return okClient.Namespaces().Wake(ctx, ns)
}

// tokenUpgrader updates the token of the config when the token is outdated
type tokenUpdater interface {
	UpdateKubeConfigToken() error
}

// oktetoClientProvider provides an okteto client ready to use or fail
type oktetoClientProvider interface {
	Provide(...okteto.Option) (types.OktetoInterface, error)
}

type tokenUpdaterController struct {
	oktetoClientProvider oktetoClientProvider
}

func newTokenUpdaterController() *tokenUpdaterController {
	return &tokenUpdaterController{
		oktetoClientProvider: okteto.NewOktetoClientProvider(),
	}
}

func (tuc *tokenUpdaterController) UpdateKubeConfigToken() error {
	oktetoClient, err := tuc.oktetoClientProvider.Provide()
	if err != nil {
		return err
	}
	token, err := oktetoClient.Kubetoken().GetKubeToken(okteto.Context().Name, okteto.Context().Namespace)
	if err != nil {
		return err
	}
	// update the token in the okteto context for future client initializations
	okCtx := okteto.Context()

	ctxUserID := okCtx.UserID
	cfg := okCtx.Cfg
	if cfg == nil {
		return errConfigNotConfigured
	}
	if _, ok := okCtx.Cfg.AuthInfos[ctxUserID]; !ok {
		return fmt.Errorf("user %s not found in kubeconfig", ctxUserID)
	}
	okCtx.Cfg.AuthInfos[ctxUserID].Token = token.Status.Token
	return nil
}
