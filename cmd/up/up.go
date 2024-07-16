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
	oargs "github.com/okteto/okteto/cmd/args"
	buildv2 "github.com/okteto/okteto/cmd/build/v2"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/namespace"
	pipelineCMD "github.com/okteto/okteto/cmd/pipeline"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/devenvironment"
	"github.com/okteto/okteto/pkg/discovery"
	"github.com/okteto/okteto/pkg/env"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/filesystem"
	"github.com/okteto/okteto/pkg/k8s/apps"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/log/io"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	oktetoPath "github.com/okteto/okteto/pkg/path"
	"github.com/okteto/okteto/pkg/process"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	"github.com/okteto/okteto/pkg/types"
	"github.com/okteto/okteto/pkg/vars"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const (
	ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

	composeVolumesUrl = "https://www.okteto.com/docs/reference/docker-compose/#volumes-string-optional"
)

var (
	errConfigNotConfigured = fmt.Errorf("kubeconfig not found")
)

// Options represents the options available on up command
type Options struct {
	// ManifestPathFlag is the option -f as introduced by the user when executing this command.
	// This is stored at the configmap as filename to redeploy from the ui.
	ManifestPathFlag string
	// ManifestPath is the path to the manifest used though the command execution.
	// This might change its value during execution
	ManifestPath string
	Namespace    string
	K8sContext   string
	DevName      string
	Envs         []string
	Remote       int
	Deploy       bool
	ForcePull    bool
	Reset        bool
}

// Up starts a development container
func Up(at analyticsTrackerInterface, insights buildDeployTrackerInterface, ioCtrl *io.Controller, k8sLogger *io.K8sLogger, varManager *vars.Manager, fs afero.Fs) *cobra.Command {
	upOptions := &Options{}
	cmd := &cobra.Command{
		Use:   "up service [flags] -- COMMAND [args...]",
		Short: "Deploy your development environment",
		Example: `  # okteto up deploying the development environment defined in the okteto manifest
okteto up my-svc --deploy -- echo this is a test

# okteto up replacing the command defined in the okteto manifest
okteto up my-svc -- echo this is a test
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if okteto.InDevContainer() {
				return oktetoErrors.ErrNotInDevContainer
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

			ctxOpts := &contextCMD.Options{
				Show:      true,
				Context:   upOptions.K8sContext,
				Namespace: upOptions.Namespace,
			}

			if err := contextCMD.NewContextCommand(contextCMD.WithVarManager(varManager)).Run(ctx, ctxOpts); err != nil {
				return err
			}

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

				// check that the manifest file exists
				if !filesystem.FileExistsWithFilesystem(manifestPathFlag, fs) {
					return oktetoErrors.ErrManifestPathNotFound
				}

				// the Okteto manifest flag should specify a file, not a directory
				if filesystem.IsDir(manifestPathFlag, fs) {
					return oktetoErrors.ErrManifestPathIsDir
				}

				// when the manifest path is set by the cmd flag, we are moving cwd so the cmd is executed from that dir
				uptManifestPath, err := filesystem.UpdateCWDtoManifestPath(upOptions.ManifestPath)
				if err != nil {
					return err
				}
				upOptions.ManifestPath = uptManifestPath
			}
			oktetoManifest, err := model.GetManifestV2(upOptions.ManifestPath, fs)
			if err != nil {
				if !errors.Is(err, discovery.ErrOktetoManifestNotFound) {
					return err
				}

				if upOptions.ManifestPath == "" {
					upOptions.ManifestPath = utils.DefaultManifest
				}
			}

			upMeta.OktetoContextConfig(time.Since(startOkContextConfig))
			if okteto.IsOkteto() {
				create, err := utils.ShouldCreateNamespace(ctx, okteto.GetContext().Namespace)
				if err != nil {
					return err
				}
				if create {
					nsCmd, err := namespace.NewCommand(varManager)
					if err != nil {
						return err
					}
					if err := nsCmd.Create(ctx, &namespace.CreateOptions{Namespace: okteto.GetContext().Namespace}); err != nil {
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
				c, _, err := okteto.NewK8sClientProviderWithLogger(k8sLogger).Provide(okteto.GetContext().Cfg)
				if err != nil {
					return err
				}
				inferer := devenvironment.NewNameInferer(c)
				oktetoManifest.Name = inferer.InferName(ctx, wd, okteto.GetContext().Namespace, upOptions.ManifestPathFlag)
			}
			os.Setenv(constants.OktetoNameEnvVar, oktetoManifest.Name)

			if len(oktetoManifest.Dev) == 0 {
				if oktetoManifest.Type == model.StackType {
					return fmt.Errorf("your docker compose file is not currently supported: Okteto requires a 'host volume' to be defined. See %s", composeVolumesUrl)
				}
				return oktetoErrors.ErrManifestNoDevSection
			}

			onBuildFinish := []buildv2.OnBuildFinish{
				at.TrackImageBuild,
				insights.TrackImageBuild,
			}

			up := &upContext{
				Namespace:         okteto.GetContext().Namespace,
				Manifest:          oktetoManifest,
				Dev:               nil,
				Exit:              make(chan error, 1),
				resetSyncthing:    upOptions.Reset,
				StartTime:         time.Now(),
				Registry:          registry.NewOktetoRegistry(okteto.Config{}),
				Options:           upOptions,
				Fs:                fs,
				analyticsTracker:  at,
				analyticsMeta:     upMeta,
				K8sClientProvider: okteto.NewK8sClientProviderWithLogger(k8sLogger),
				varManager:        varManager,
				tokenUpdater:      newTokenUpdaterController(),
				builder:           buildv2.NewBuilderFromScratch(ioCtrl, varManager, onBuildFinish),
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

			k8sClient, _, err := okteto.GetK8sClientWithLogger(k8sLogger)
			if err != nil {
				return fmt.Errorf("failed to load k8s client: %w", err)
			}

			if upOptions.Deploy || !pipeline.IsDeployed(ctx, up.Manifest.Name, up.Namespace, k8sClient) {
				err := up.deployApp(ctx, ioCtrl, k8sLogger)

				// only allow error.ErrManifestFoundButNoDeployAndDependenciesCommands to go forward - autocreate property will deploy the app
				if err != nil && !errors.Is(err, oktetoErrors.ErrManifestFoundButNoDeployAndDependenciesCommands) {
					return err
				}
			} else if !upOptions.Deploy && pipeline.IsDeployed(ctx, up.Manifest.Name, okteto.GetContext().Namespace, k8sClient) {
				oktetoLog.Information("'%s' was already deployed. To redeploy run 'okteto deploy' or 'okteto up --deploy'", up.Manifest.Name)
			}

			devCommandParser := oargs.NewDevCommandArgParser(oargs.NewManifestDevLister(), ioCtrl, false)

			argsparserResult, err := devCommandParser.Parse(ctx, args, cmd.ArgsLenAtDash(), oktetoManifest.Dev, okteto.GetContext().Namespace)
			if err != nil {
				return err
			}

			dev, err := utils.GetDevFromManifest(oktetoManifest, argsparserResult.DevName)
			if err != nil {
				return err
			}

			if len(argsparserResult.Command) > 0 {
				dev.Command.Values = argsparserResult.Command
			}

			if err := dev.PreparePathsAndExpandEnvFiles(oktetoManifest.ManifestPath, up.Fs); err != nil {
				return fmt.Errorf("error in 'dev' section of your manifest: %w", err)
			}

			up.Dev = dev

			// only if the context is an okteto one, we should verify if the namespace has to be woken up
			if okteto.GetContext().IsOkteto {
				// We execute it in a goroutine to not impact the command performance
				go func() {
					okClient, err := okteto.NewOktetoClient()
					if err != nil {
						oktetoLog.Infof("failed to create okteto client: '%s'", err.Error())
						return
					}
					if err := wakeNamespaceIfApplies(ctx, up.Namespace, k8sClient, okClient); err != nil {
						// If there is an error waking up namespace, we don't want to fail the up command
						oktetoLog.Infof("failed to wake up the namespace: %s", err.Error())
					}
				}()
			}

			// build images and set env vars for the services at the manifest
			if err := buildServicesAndSetBuildEnvs(ctx, oktetoManifest, up.builder); err != nil {
				return err
			}

			if err := loadManifestOverrides(dev, upOptions, up.varManager); err != nil {
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

			oktetoLog.ConfigureFileLogger(config.GetAppHome(okteto.GetContext().Namespace, dev.Name), config.VersionString)

			if err := checkStignoreConfiguration(dev); err != nil {
				oktetoLog.Infof("failed to check '.stignore' configuration: %s", err.Error())
			}

			if err := addStignoreSecrets(dev, okteto.GetContext().Namespace); err != nil {
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

			if err = up.start(); err != nil {
				switch err.(type) {
				default:
					return fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(okteto.GetContext().Namespace, dev.Name))
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

	cmd.Flags().StringVarP(&upOptions.ManifestPath, "file", "f", "", "path to the Okteto manifest file")
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
	return cmd
}

func loadManifestOverrides(dev *model.Dev, upOptions *Options, varManager *vars.Manager) error {
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
		overridedEnvVars, err := getOverridedEnvVarsFromCmd(dev.Environment, upOptions.Envs, varManager)
		if err != nil {
			return err
		} else {
			dev.Environment = *overridedEnvVars
		}
	}

	dev.Username = okteto.GetContext().Username
	dev.RegistryURL = okteto.GetContext().Registry

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

func getOverridedEnvVarsFromCmd(manifestEnvVars env.Environment, commandEnvVariables []string, varManager *vars.Manager) (*env.Environment, error) {
	envVarsToValues := make(map[string]string)
	for _, manifestEnv := range manifestEnvVars {
		envVarsToValues[manifestEnv.Name] = manifestEnv.Value
	}

	for _, v := range commandEnvVariables {
		varsLength := 2
		kv := strings.SplitN(v, "=", varsLength)
		if len(kv) != varsLength {
			if kv[0] == "" {
				return nil, fmt.Errorf("invalid variable value '%s': please review the accepted formats at https://www.okteto.com/docs/reference/okteto-manifest/#environment-string-optional ", v)
			}
			kv = append(kv, os.Getenv(kv[0]))
		}

		varNameToAdd, varValueToAdd := kv[0], kv[1]
		if strings.HasPrefix(varNameToAdd, "OKTETO_") || varNameToAdd == model.OktetoBuildkitHostURLEnvVar {
			return nil, oktetoErrors.ErrBuiltInOktetoEnvVarSetFromCMD
		}

		expandedEnv, err := varManager.ExpandExcLocal(varValueToAdd)
		if err != nil {
			return nil, err
		}

		envVarsToValues[varNameToAdd] = expandedEnv
	}

	overridedEnvVars := env.Environment{}
	for k, v := range envVarsToValues {
		overridedEnvVars = append(overridedEnvVars, vars.Var{Name: k, Value: v})
	}

	return &overridedEnvVars, nil
}

func (up *upContext) deployApp(ctx context.Context, ioCtrl *io.Controller, k8slogger *io.K8sLogger) error {
	k8sProvider := okteto.NewK8sClientProviderWithLogger(k8slogger)
	pc, err := pipelineCMD.NewCommand()
	if err != nil {
		return err
	}
	c := &deploy.Command{
		GetManifest:       model.GetManifestV2,
		GetDeployer:       deploy.GetDeployer,
		K8sClientProvider: k8sProvider,
		Builder:           up.builder,
		Fs:                up.Fs,
		CfgMapHandler:     deploy.NewConfigmapHandler(k8sProvider, k8slogger),
		PipelineCMD:       pc,
		DeployWaiter:      deploy.NewDeployWaiter(k8sProvider, k8slogger),
		EndpointGetter:    deploy.NewEndpointGetter,
		AnalyticsTracker:  up.analyticsTracker,
		IoCtrl:            ioCtrl,
	}

	startTime := time.Now()
	err = c.Run(ctx, &deploy.Options{
		Name:             up.Manifest.Name,
		Namespace:        up.Namespace,
		ManifestPathFlag: up.Options.ManifestPathFlag,
		ManifestPath:     up.Options.ManifestPath,
		Timeout:          5 * time.Minute,
		NoBuild:          false,
	})
	up.analyticsMeta.HasRunDeploy()

	isRemote := false
	if up.Manifest.Deploy != nil {
		isRemote = up.Manifest.Deploy.Image != ""
	}

	// We keep DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar for backward compatibility in case an old version of the backend
	// is being used
	isPreview := os.Getenv(model.DeprecatedOktetoCurrentDeployBelongsToPreviewEnvVar) == "true" ||
		os.Getenv(constants.OktetoIsPreviewEnvVar) == "true"
	// tracking deploy either its been successful or not
	c.AnalyticsTracker.TrackDeploy(analytics.DeployMetadata{
		Success:                err == nil,
		IsOktetoRepo:           utils.IsOktetoRepo(),
		Duration:               time.Since(startTime),
		PipelineType:           up.Manifest.Type,
		DeployType:             "automatic",
		IsPreview:              isPreview,
		HasDependenciesSection: up.Manifest.HasDependenciesSection(),
		HasBuildSection:        up.Manifest.HasBuildSection(),
		Err:                    err,
		IsRemote:               isRemote,
	})
	return err
}

func (up *upContext) start() error {
	up.pidController = newPIDController(up.Namespace, up.Dev.Name)

	if err := up.pidController.create(); err != nil {
		oktetoLog.Infof("failed to create pid file for %s - %s: %s", up.Namespace, up.Dev.Name, err)

		return oktetoErrors.UserError{
			E: fmt.Errorf("couldn't create a pid file for %s - %s", up.Namespace, up.Dev.Name),
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
		if err := config.DeleteStateFile(up.Dev.Name, up.Namespace); err != nil {
			oktetoLog.Infof("failed to delete state file: %s", err)
		}
	}()
	for {
		if up.isRetry || isTransientError {
			oktetoLog.Infof("waiting for shutdown sequence to finish")
			<-up.ShutdownCompleted
			pidFromFile, err := up.pidController.get()
			if err != nil {
				oktetoLog.Infof("error getting pid: %s", err)
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

			if okteto.IsOkteto() {
				oktetoLog.Info("updating kubeconfig token")
				if err := up.tokenUpdater.UpdateKubeConfigToken(); err != nil {
					oktetoLog.Infof("error updating k8s token: %s", err)
					isTransientError = true
					continue
				}
			}

			if err == oktetoErrors.ErrLostSyncthing {
				isTransientError = false
				iter = 0
				continue
			}

			if up.isTransient(err) {
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
	// only for unix because Windows does not support SIGTTIN and SIGTTOU
	goToBackground := getSendToBackgroundSignals()
	for {
		select {
		case err := <-up.CommandResult:
			oktetoLog.Println()
			if err != nil {
				oktetoLog.Infof("command failed: %s", err)
				if up.isTransient(err) {
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

		case <-goToBackground:
			oktetoLog.Infof("SIGTTOU/SIGTTIN received, starting shutdown sequence preventing it to background the process")
			return oktetoErrors.UserError{
				E:    fmt.Errorf("connection lost to your development container"),
				Hint: "Use 'okteto up' to reconnect",
			}

		case err := <-up.applyToApps(ctx):
			oktetoLog.Infof("exiting by applyToAppsChan: %v", err)
			return err
		}
	}
}

func (up *upContext) applyToApps(ctx context.Context) chan error {
	k8sClient, _, err := up.K8sClientProvider.Provide(okteto.GetContext().Cfg)
	if err != nil {
		return nil
	}
	result := make(chan error, 1)
	for _, tr := range up.Translations {
		go tr.App.Watch(ctx, result, k8sClient)
	}
	return result
}

func (up *upContext) setDevContainer(app apps.App) error {
	devContainer := apps.GetDevContainer(app.PodSpec(), up.Dev.Container)
	if devContainer == nil {
		return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
	}

	up.Dev.Container = devContainer.Name

	if up.Dev.Image == "" {
		up.Dev.Image = devContainer.Image
	}

	return nil
}

func (up *upContext) getInsufficientSpaceError(err error) error {
	if up.Dev.PersistentVolumeEnabled() {

		return oktetoErrors.UserError{
			E: err,
			Hint: fmt.Sprintf(`Okteto volume is full.
    Increase your persistent volume size, run '%s' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/okteto-manifest/#persistentvolume-object-optional`, utils.GetDownCommand(up.Options.ManifestPathFlag)),
		}
	}
	return oktetoErrors.UserError{
		E: err,
		Hint: `The synchronization service is running out of space.
    Enable persistent volumes in your okteto manifest and try again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/okteto-manifest/#persistentvolume-object-optional`,
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

	p := process.New(up.hybridCommand.Process.Pid)
	if err := terminateProcess(p); err != nil {
		oktetoLog.Debugf("error terminating process %d: %v", up.hybridCommand.Process.Pid, err)
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

		p := process.New(pR.Pid())
		if err := terminateProcess(p); err != nil {
			if errors.Is(err, os.ErrProcessDone) {
				continue
			}
			oktetoLog.Debugf("error terminating process %d: %v", pR.Pid(), err)
		}
	}
}

func terminateProcess(p process.Interface) error {
	oktetoLog.Debugf("terminating process: %d", p.Getpid())

	if err := p.Kill(); err != nil {
		if errors.Is(err, os.ErrProcessDone) {
			return nil
		}
		oktetoLog.Debugf("error terminating process %d: %v", p.Getpid(), err)
		return err
	}
	return nil
}

func printDisplayContext(up *upContext) {
	oktetoLog.Println(fmt.Sprintf("    %s   %s", oktetoLog.BlueString("Context:"), okteto.RemoveSchema(okteto.GetContext().Name)))
	oktetoLog.Println(fmt.Sprintf("    %s %s", oktetoLog.BlueString("Namespace:"), up.Namespace))
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
	svcsToBuild, err := builder.GetServicesToBuildDuringExecution(ctx, m, []string{})
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
	token, err := oktetoClient.Kubetoken().GetKubeToken(okteto.GetContext().Name, okteto.GetContext().Namespace)
	if err != nil {
		return err
	}
	// update the token in the okteto context for future client initializations
	okCtx := okteto.GetContext()

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
