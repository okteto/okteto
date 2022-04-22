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

package up

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/moby/term"
	contextCMD "github.com/okteto/okteto/cmd/context"
	"github.com/okteto/okteto/cmd/deploy"
	"github.com/okteto/okteto/cmd/manifest"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/cmd/utils/executor"
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/cmd/build"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/cmd/pipeline"
	"github.com/okteto/okteto/pkg/config"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

// UpOptions represents the options available on up command
type UpOptions struct {
	DevPath       string
	Namespace     string
	K8sContext    string
	DevName       string
	Devs          []string
	Remote        int
	Deploy        bool
	Build         bool
	ForcePull     bool
	Reset         bool
	Detach        bool
	DockerDesktop bool
}

// Up starts a development container
func Up() *cobra.Command {
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
			if upOptions.DockerDesktop {
				os.Setenv(model.OktetoOriginEnvVar, model.OktetoDockerDesktopOrigin)
				os.Setenv(model.OktetoAutogenerateStignoreEnvVar, "true")
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

			if upOptions.DevPath != "" {
				workdir := utils.GetWorkdirFromManifestPath(upOptions.DevPath)
				if err := os.Chdir(workdir); err != nil {
					return err
				}
				upOptions.DevPath = utils.GetManifestPathFromWorkdir(upOptions.DevPath, workdir)
			}
			manifestOpts := contextCMD.ManifestOptions{Filename: upOptions.DevPath, Namespace: upOptions.Namespace, K8sContext: upOptions.K8sContext}
			oktetoManifest, err := contextCMD.LoadManifestWithContext(ctx, manifestOpts)
			if err != nil {
				if !strings.Contains(err.Error(), "okteto init") {
					return err
				}
				if !utils.AskIfOktetoInit(upOptions.DevPath) {
					return err
				}

				oktetoManifest, err = LoadManifestWithInit(ctx, upOptions.K8sContext, upOptions.Namespace, upOptions.DevPath)
				if err != nil {
					return err
				}
			}
			wd, err := os.Getwd()
			if err != nil {
				return err
			}
			if oktetoManifest.Name == "" {
				oktetoManifest.Name = utils.InferName(wd)
			}
			os.Setenv(model.OktetoNameEnvVar, oktetoManifest.Name)

			if len(oktetoManifest.Dev) == 0 {
				oktetoLog.Warning("okteto manifest has no 'dev' section.")
				answer, err := utils.AskYesNo("Do you want to configure okteto manifest now? [y/n]")
				if err != nil {
					return err
				}
				if answer {
					mc := &manifest.ManifestCommand{
						K8sClientProvider: okteto.NewK8sClientProvider(),
					}
					if upOptions.DevPath == "" {
						upOptions.DevPath = utils.DefaultManifest
					}
					oktetoManifest, err = mc.RunInitV2(ctx, &manifest.InitOpts{
						DevPath:          upOptions.DevPath,
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
			var dev *model.Dev
			if upOptions.Detach {
				dev, err = utils.GetDevDetachMode(oktetoManifest, upOptions.Devs)
				if err != nil {
					return err
				}
			} else {
				dev, err = utils.GetDevFromManifest(oktetoManifest, upOptions.DevName)
				if err != nil {
					return err
				}
			}

			if err := setBuildEnvVars(oktetoManifest, dev.Name); err != nil {
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

			if _, ok := os.LookupEnv(model.OktetoAutoDeployEnvVar); ok {
				upOptions.Deploy = true
			}

			up := &upContext{
				Manifest:       oktetoManifest,
				Dev:            dev,
				Exit:           make(chan error, 1),
				resetSyncthing: upOptions.Reset,
				StartTime:      time.Now(),
				Options:        upOptions,
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
			up.Client, up.RestConfig, err = okteto.GetK8sClient()
			if err != nil {
				return fmt.Errorf("failed to load okteto context '%s': %v", up.Dev.Context, err)
			}

			if upOptions.Deploy || (up.Manifest.IsV2 && !pipeline.IsDeployed(ctx, up.Manifest.Name, up.Manifest.Namespace, up.Client)) {
				if !upOptions.Deploy {
					oktetoLog.Information("Deploying development environment '%s'...", up.Manifest.Name)
					oktetoLog.Information("To redeploy your development environment manually run 'okteto deploy' or 'okteto up --deploy'")
				}
				startTime := time.Now()
				err := up.deployApp(ctx)
				if err != nil && oktetoErrors.ErrManifestFoundButNoDeployCommands != err {
					return err
				}
				if oktetoErrors.ErrManifestFoundButNoDeployCommands != err && !upOptions.Detach {
					up.Dev.Autocreate = false
				}
				if err != nil {
					analytics.TrackDeploy(analytics.TrackDeployMetadata{
						Success:                err == nil,
						IsOktetoRepo:           utils.IsOktetoRepo(),
						Duration:               time.Since(startTime),
						PipelineType:           up.Manifest.Type,
						DeployType:             "automatic",
						IsPreview:              os.Getenv(model.OktetoCurrentDeployBelongsToPreview) == "true",
						HasDependenciesSection: up.Manifest.IsV2 && len(up.Manifest.Dependencies) > 0,
						HasBuildSection:        up.Manifest.IsV2 && len(up.Manifest.Build) > 0,
					})
				}

			} else if !upOptions.Deploy && (up.Manifest.IsV2 && pipeline.IsDeployed(ctx, up.Manifest.Name, up.Manifest.Namespace, up.Client)) {
				oktetoLog.Information("Development environment '%s' already deployed.", up.Manifest.Name)
				oktetoLog.Information("To redeploy your development environment run 'okteto deploy' or 'okteto up %s --deploy'", up.Dev.Name)
			}

			err = up.start()

			if err != nil {
				switch err.(type) {
				default:
					err = fmt.Errorf("%w\n    Find additional logs at: %s/okteto.log", err, config.GetAppHome(dev.Namespace, dev.Name))
				case oktetoErrors.CommandError:
					oktetoLog.Infof("CommandError: %v", err)
				case oktetoErrors.UserError:
					return err
				}

			}

			return err
		},
	}

	cmd.Flags().StringVarP(&upOptions.DevPath, "file", "f", "", "path to the manifest file")
	cmd.Flags().StringVarP(&upOptions.Namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().StringVarP(&upOptions.K8sContext, "context", "c", "", "context where the up command is executed")
	cmd.Flags().IntVarP(&upOptions.Remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&upOptions.Deploy, "deploy", "d", false, "Force execution of the commands in the 'deploy' section of the okteto manifest (defaults to 'false')")
	cmd.Flags().BoolVarP(&upOptions.Build, "build", "", false, "build on-the-fly the dev image using the info provided by the 'build' okteto manifest field")
	cmd.Flags().MarkHidden("build")
	cmd.Flags().BoolVarP(&upOptions.ForcePull, "pull", "", false, "force dev image pull")
	cmd.Flags().MarkHidden("pull")
	cmd.Flags().BoolVarP(&upOptions.Reset, "reset", "", false, "reset the file synchronization database")
	cmd.Flags().BoolVarP(&upOptions.Detach, "detach", "", false, "activate one more development containers in detached mode")
	cmd.Flags().BoolVarP(&upOptions.DockerDesktop, "docker-desktop", "", false, "if the command is executed from the Docker Desktop extension")
	cmd.Flags().MarkHidden("docker-desktop")
	return cmd
}

// AddArgs sets the args as options and return err if it's not compatible
func (o *UpOptions) AddArgs(cmd *cobra.Command, args []string) error {
	if o.Detach {
		o.Devs = args
	} else {
		maxV1Args := 1
		docsURL := "https://okteto.com/docs/reference/cli/#up"
		if len(args) > maxV1Args {
			cmd.Help()
			return oktetoErrors.UserError{
				E:    fmt.Errorf("%q accepts at most %d arg(s), but received %d", cmd.CommandPath(), maxV1Args, len(args)),
				Hint: fmt.Sprintf("Visit %s for more information.", docsURL),
			}
		} else if len(args) == 1 {
			o.DevName = args[0]
		}
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

	oktetoLog.Success(fmt.Sprintf("okteto manifest (%s) created", devPath))
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

	dev.Username = okteto.Context().Username
	dev.RegistryURL = okteto.Context().Registry

	return nil
}

func (up *upContext) deployApp(ctx context.Context) error {
	kubeconfig := deploy.NewKubeConfig()
	proxy, err := deploy.NewProxy(kubeconfig)
	if err != nil {
		return err
	}

	c := &deploy.DeployCommand{
		GetManifest:        up.getManifest,
		Kubeconfig:         kubeconfig,
		Executor:           executor.NewExecutor(oktetoLog.GetOutputFormat()),
		Proxy:              proxy,
		TempKubeconfigFile: deploy.GetTempKubeConfigFile(up.Manifest.Name),
		K8sClientProvider:  okteto.NewK8sClientProvider(),
	}

	return c.RunDeploy(ctx, &deploy.Options{
		Name:         up.Manifest.Name,
		ManifestPath: up.Manifest.Filename,
		Timeout:      5 * time.Minute,
		Build:        false,
	})
}

func (up *upContext) getManifest(path string) (*model.Manifest, error) {
	if up.Manifest != nil {
		return up.Manifest, nil
	}
	return model.GetManifestV2(path)
}
func (up *upContext) start() error {

	ctx := context.Background()

	if up.Dev.Divert != nil {
		if err := diverts.Create(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	if err := createPIDFile(up.Dev.Namespace, up.Dev.Name); err != nil {
		oktetoLog.Infof("failed to create pid file for %s - %s: %s", up.Dev.Namespace, up.Dev.Name, err)
		return fmt.Errorf("couldn't create pid file for %s - %s", up.Dev.Namespace, up.Dev.Name)
	}

	defer cleanPIDFile(up.Dev.Namespace, up.Dev.Name)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	analytics.TrackUp(analytics.TrackUpMetadata{
		IsInteractive:          up.getInteractive(),
		IsOktetoRepository:     utils.IsOktetoRepo(),
		HasDependenciesSection: up.Manifest.IsV2 && len(up.Manifest.Dependencies) > 0,
		HasBuildSection:        up.Manifest.IsV2 && len(up.Manifest.Build) > 0,
		HasDeploySection: (up.Manifest.IsV2 &&
			up.Manifest.Deploy != nil &&
			(len(up.Manifest.Deploy.Commands) > 0 || up.Manifest.Deploy.ComposeSection.ComposesInfo != nil)),
	})

	go up.activateLoop()

	select {
	case <-stop:
		oktetoLog.Infof("CTRL+C received, starting shutdown sequence")
		up.shutdown()
		oktetoLog.Println()
	case err := <-up.Exit:
		if err != nil {
			oktetoLog.Infof("exit signal received due to error: %s", err)
			return err
		}
	}
	return nil
}

// activateLoop activates the development container in a retry loop
func (up *upContext) activateLoop() {
	isTransientError := false
	t := time.NewTicker(1 * time.Second)
	iter := 0
	defer t.Stop()

	defer config.DeleteStateFile(up.Dev)

	for {
		if up.isRetry || isTransientError {
			oktetoLog.Infof("waiting for shutdown sequence to finish")
			<-up.ShutdownCompleted
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

		case err := <-up.applyToApps(ctx):
			oktetoLog.Infof("exiting by applyToAppsChan: %v", err)
			return err
		}
	}
}

func (up *upContext) applyToApps(ctx context.Context) chan error {
	result := make(chan error, 1)
	for _, tr := range up.Translations {
		go tr.App.Watch(ctx, result, up.Client)
	}
	return result
}

func (up *upContext) buildDevImage(ctx context.Context, app apps.App) error {
	if _, err := os.Stat(up.Dev.Image.Dockerfile); err != nil {
		return oktetoErrors.UserError{
			E:    fmt.Errorf("'--build' argument given but there is no Dockerfile"),
			Hint: "Try creating a Dockerfile file or specify the 'context' and 'dockerfile' fields in your okteto manifest.",
		}
	}

	oktetoRegistryURL := okteto.Context().Registry
	if oktetoRegistryURL == "" && up.Dev.Autocreate && up.Dev.Image.Name == "" {
		return fmt.Errorf("no value for 'image' has been provided in your okteto manifest")
	}

	if up.Dev.Image.Name == "" {
		devContainer := apps.GetDevContainer(app.PodSpec(), up.Dev.Container)
		if devContainer == nil {
			return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
		}
		up.Dev.Image.Name = devContainer.Image
	}

	oktetoLog.Information("Running your build in %s...", okteto.Context().Builder)

	imageTag := registry.GetImageTag(up.Dev.Image.Name, up.Dev.Name, up.Dev.Namespace, oktetoRegistryURL)
	oktetoLog.Infof("building dev image tag %s", imageTag)

	buildArgs := model.SerializeBuildArgs(up.Dev.Image.Args)

	buildOptions := &buildCMD.BuildOptions{
		Path:       up.Dev.Image.Context,
		File:       up.Dev.Image.Dockerfile,
		Tag:        imageTag,
		Target:     up.Dev.Image.Target,
		CacheFrom:  up.Dev.Image.CacheFrom,
		BuildArgs:  buildArgs,
		OutputMode: oktetoLog.TTYFormat,
	}
	if err := buildCMD.Run(ctx, buildOptions); err != nil {
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

func (up *upContext) getInteractive() bool {
	if len(up.Dev.Command.Values) == 0 {
		return true
	}
	if len(up.Dev.Command.Values) == 1 {
		switch up.Dev.Command.Values[0] {
		case "sh", "bash":
			return true
		default:
			return false
		}
	}
	return false
}

func (up *upContext) getInsufficientSpaceError(err error) error {
	if up.Dev.PersistentVolumeEnabled() {

		return oktetoErrors.UserError{
			E: err,
			Hint: fmt.Sprintf(`Okteto volume is full.
    Increase your persistent volume size, run '%s' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`, utils.GetDownCommand(up.Options.DevPath)),
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
		if up.spinner != nil {
			up.spinner.Stop()
		}
	}

	oktetoLog.Infof("starting shutdown sequence")
	if !up.success {
		analytics.TrackUpError(true)
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

	oktetoLog.Info("completed shutdown sequence")
	up.ShutdownCompleted <- true

}

func printDisplayContext(dev *model.Dev, divertURL string) {
	oktetoLog.Println(fmt.Sprintf("    %s   %s", oktetoLog.BlueString("Context:"), okteto.RemoveSchema(dev.Context)))
	oktetoLog.Println(fmt.Sprintf("    %s %s", oktetoLog.BlueString("Namespace:"), dev.Namespace))
	oktetoLog.Println(fmt.Sprintf("    %s      %s", oktetoLog.BlueString("Name:"), dev.Name))

	if len(dev.Forward) > 0 {
		if dev.Forward[0].Service {
			oktetoLog.Println(fmt.Sprintf("    %s   %d -> %s:%d", oktetoLog.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].ServiceName, dev.Forward[0].Remote))
		} else {
			oktetoLog.Println(fmt.Sprintf("    %s   %d -> %d", oktetoLog.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].Remote))
		}

		for i := 1; i < len(dev.Forward); i++ {
			if dev.Forward[i].Service {
				oktetoLog.Println(fmt.Sprintf("               %d -> %s:%d", dev.Forward[i].Local, dev.Forward[i].ServiceName, dev.Forward[i].Remote))
				continue
			}
			oktetoLog.Println(fmt.Sprintf("               %d -> %d", dev.Forward[i].Local, dev.Forward[i].Remote))
		}
	}

	if len(dev.Reverse) > 0 {
		oktetoLog.Println(fmt.Sprintf("    %s   %d <- %d", oktetoLog.BlueString("Reverse:"), dev.Reverse[0].Local, dev.Reverse[0].Remote))
		for i := 1; i < len(dev.Reverse); i++ {
			oktetoLog.Println(fmt.Sprintf("               %d <- %d", dev.Reverse[i].Local, dev.Reverse[i].Remote))
		}
	}

	if divertURL != "" {
		oktetoLog.Println(fmt.Sprintf("    %s       %s", oktetoLog.BlueString("URL:"), divertURL))
	}
	oktetoLog.Println()
}

func setBuildEnvVars(m *model.Manifest, devName string) error {
	sp := utils.NewSpinner("Loading build env vars...")
	sp.Start()
	defer sp.Stop()

	for buildName, buildInfo := range m.Build {
		opts := build.OptsFromManifest(buildName, buildInfo, &build.BuildOptions{})
		imageWithDigest, err := registry.GetImageTagWithDigest(opts.Tag)
		if err == oktetoErrors.ErrNotFound {
			os.Setenv(fmt.Sprintf("OKTETO_BUILD_%s_IMAGE", strings.ToUpper(buildName)), opts.Tag)
		} else if err != nil {
			return fmt.Errorf("error checking image at registry %s: %v", opts.Tag, err)
		} else {
			if err := deploy.SetManifestEnvVars(buildName, imageWithDigest); err != nil {
				return err
			}
		}
	}

	var err error
	if _, ok := m.Dev[devName]; ok && m.Dev[devName].Image != nil {
		m.Dev[devName].Image.Name, err = model.ExpandEnv(m.Dev[devName].Image.Name, false)
	}

	return err
}
