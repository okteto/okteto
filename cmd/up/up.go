// Copyright 2021 The Okteto Authors
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

	"github.com/joho/godotenv"
	"github.com/moby/term"
	contextCMD "github.com/okteto/okteto/cmd/context"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/spf13/cobra"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

type UpOptions struct {
	DevPath    string
	Namespace  string
	K8sContext string
	Remote     int
	AutoDeploy bool
	Build      bool
	ForcePull  bool
	Reset      bool
}

// Up starts a development container
func Up() *cobra.Command {
	upOptions := &UpOptions{}
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activates your development container",
		Args:  utils.NoArgsAccepted("https://okteto.com/docs/reference/cli/#up"),
		RunE: func(cmd *cobra.Command, args []string) error {
			if okteto.InDevContainer() {
				return errors.ErrNotInDevContainer
			}

			u := utils.UpgradeAvailable()
			if len(u) > 0 {
				warningFolder := filepath.Join(config.GetOktetoHome(), ".warnings")
				if utils.GetWarningState(warningFolder, "version") != u {
					log.Yellow("Okteto %s is available. To upgrade:", u)
					log.Yellow("    %s", utils.GetUpgradeCommand())
					if err := utils.SetWarningState(warningFolder, "version", u); err != nil {
						log.Infof("failed to set warning version state: %s", err.Error())
					}
				}
			}

			checkLocalWatchesConfiguration()

			if upOptions.AutoDeploy {
				log.Warning(`The 'deploy' flag is deprecated and will be removed in a future release.
    Set the 'autocreate' field in your okteto manifest to get the same behavior.
    More information is available here: https://okteto.com/docs/reference/cli/#up`)
			}

			ctx := context.Background()

			if model.FileExists(".env") {
				err := godotenv.Load()
				if err != nil {
					log.Errorf("error loading .env file: %s", err.Error())
				}
			}

			dev, err := contextCMD.LoadDevWithContext(ctx, upOptions.DevPath, upOptions.Namespace, upOptions.K8sContext)
			if err != nil {
				if !strings.Contains(err.Error(), "okteto init") {
					return err
				}
				if !utils.AskIfOktetoInit(upOptions.DevPath) {
					return err
				}

				dev, err = loadDevWithInit(upOptions.DevPath)
				if err != nil {
					return err
				}
			}

			if err := loadDevOverrides(dev, upOptions); err != nil {
				return err
			}

			if syncthing.ShouldUpgrade() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					log.Infof("failed to upgrade syncthing: %s", err)

					if !syncthing.IsInstalled() {
						return fmt.Errorf("couldn't download syncthing, please try again")
					}

					log.Yellow("couldn't upgrade syncthing, will try again later")
					fmt.Println()
				} else {
					log.Success("Dependencies successfully installed")
				}
			}

			log.ConfigureFileLogger(config.GetAppHome(dev.Namespace, dev.Name), config.VersionString)

			if err := checkStignoreConfiguration(dev); err != nil {
				log.Infof("failed to check '.stignore' configuration: %s", err.Error())
			}

			if err := addStignoreSecrets(dev); err != nil {
				return err
			}

			if _, ok := os.LookupEnv("OKTETO_AUTODEPLOY"); ok {
				upOptions.AutoDeploy = true
			}

			up := &upContext{
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
					log.Infof("failed to save the state of the terminal: %s", err.Error())
					return fmt.Errorf("failed to save the state of the terminal")
				}
				log.Infof("Terminal: %v", up.stateTerm)
			}

			err = up.start()

			if err := up.Client.CoreV1().PersistentVolumeClaims(dev.Namespace).Delete(ctx, fmt.Sprintf(model.DeprecatedOktetoVolumeNameTemplate, dev.Name), metav1.DeleteOptions{}); err != nil {
				log.Infof("error deleting deprecated volume: %v", err)
			}

			return err
		},
	}

	cmd.Flags().StringVarP(&upOptions.DevPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&upOptions.Namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().StringVarP(&upOptions.K8sContext, "context", "c", "", "context where the up command is executed")
	cmd.Flags().IntVarP(&upOptions.Remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&upOptions.AutoDeploy, "deploy", "d", false, "create deployment when it doesn't exist in a namespace")
	cmd.Flags().MarkHidden("deploy")
	cmd.Flags().BoolVarP(&upOptions.Build, "build", "", false, "build on-the-fly the dev image using the info provided by the 'build' okteto manifest field")
	cmd.Flags().BoolVarP(&upOptions.ForcePull, "pull", "", false, "force dev image pull")
	cmd.Flags().BoolVarP(&upOptions.Reset, "reset", "", false, "reset the file synchronization database")
	return cmd
}

func loadDevWithInit(devPath string) (*model.Dev, error) {
	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unknown current folder: %s", err)
	}
	if err := initCMD.Run(devPath, "", workDir, false); err != nil {
		return nil, err
	}

	log.Success(fmt.Sprintf("okteto manifest (%s) created", devPath))
	return utils.LoadDev(devPath)
}

func loadDevOverrides(dev *model.Dev, upOptions *UpOptions) error {
	if upOptions.Remote > 0 {
		dev.RemotePort = upOptions.Remote
	}

	if dev.RemoteModeEnabled() {
		if err := sshKeys(); err != nil {
			return err
		}

		dev.LoadRemote(ssh.GetPublicKey())
	}

	if !dev.Autocreate {
		dev.Autocreate = upOptions.AutoDeploy
	}

	if upOptions.ForcePull {
		dev.LoadForcePull()
	}

	dev.Username = okteto.Context().Username
	dev.RegistryURL = okteto.Context().Registry

	return nil
}

func (up *upContext) start() error {
	var err error
	up.Client, up.RestConfig, err = okteto.GetK8sClient()
	if err != nil {
		return fmt.Errorf("failed to load okteto context '%s': %v", up.Dev.Context, err)
	}

	ctx := context.Background()

	if up.Dev.Divert != nil {
		if err := diverts.Create(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	if err := createPIDFile(up.Dev.Namespace, up.Dev.Name); err != nil {
		log.Infof("failed to create pid file for %s - %s: %s", up.Dev.Namespace, up.Dev.Name, err)
		return fmt.Errorf("couldn't create pid file for %s - %s", up.Dev.Namespace, up.Dev.Name)
	}

	defer cleanPIDFile(up.Dev.Namespace, up.Dev.Name)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	analytics.TrackUp(true, up.Dev.Name, up.getInteractive(), len(up.Dev.Services) == 0, up.Dev.Divert != nil)

	go up.activateLoop()

	select {
	case <-stop:
		log.Infof("CTRL+C received, starting shutdown sequence")
		up.shutdown()
		fmt.Println()
	case err := <-up.Exit:
		if err != nil {
			log.Infof("exit signal received due to error: %s", err)
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
			log.Infof("waiting for shutdown sequence to finish")
			<-up.ShutdownCompleted
			if iter == 0 {
				log.Yellow("Connection lost to your development container, reconnecting...")
			}
			iter++
			iter = iter % 10
			if isTransientError {
				<-t.C
			}
		}

		err := up.activate()
		if err != nil {
			log.Infof("activate failed with: %s", err)

			if err == errors.ErrLostSyncthing {
				isTransientError = false
				iter = 0
				continue
			}

			if errors.IsTransient(err) {
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
			fmt.Println()
			if err != nil {
				log.Infof("command failed: %s", err)
				if errors.IsTransient(err) {
					return err
				}
				return errors.CommandError{
					E:      errors.ErrCommandFailed,
					Reason: err,
				}
			}

			log.Info("command completed")
			return nil

		case err := <-up.Disconnect:
			if err == errors.ErrInsufficientSpace {
				return up.getInsufficientSpaceError(err)
			}
			return err

		case err := <-up.applyToApps(ctx):
			log.Infof("exiting by applyToAppsChan: %v", err)
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
		return errors.UserError{
			E:    fmt.Errorf("'--build' argument given but there is no Dockerfile"),
			Hint: "Try creating a Dockerfile or specify 'context' and 'dockerfile' fields.",
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

	log.Information("Running your build in %s...", okteto.Context().Buildkit)

	imageTag := registry.GetImageTag(up.Dev.Image.Name, up.Dev.Name, up.Dev.Namespace, oktetoRegistryURL)
	log.Infof("building dev image tag %s", imageTag)

	buildArgs := model.SerializeBuildArgs(up.Dev.Image.Args)

	buildOptions := buildCMD.BuildOptions{
		Path:       up.Dev.Image.Context,
		File:       up.Dev.Image.Dockerfile,
		Tag:        imageTag,
		Target:     up.Dev.Image.Target,
		CacheFrom:  up.Dev.Image.CacheFrom,
		BuildArgs:  buildArgs,
		OutputMode: "tty",
	}
	if err := buildCMD.Run(ctx, up.Dev.Namespace, buildOptions); err != nil {
		if registry.IsAlreadyBuiltInGlobalRegistry(err) {
			up.Dev.Image.Name = imageTag
			return nil
		}
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

		return errors.UserError{
			E: err,
			Hint: fmt.Sprintf(`Okteto volume is full.
    Increase your persistent volume size, run '%s' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`, utils.GetDownCommand(up.Options.DevPath)),
		}
	}
	return errors.UserError{
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
			log.Infof("failed to restore terminal: %s", err.Error())
		}
		if up.spinner != nil {
			up.spinner.Stop()
		}
	}

	log.Infof("starting shutdown sequence")
	if !up.success {
		analytics.TrackUpError(true)
	}

	if up.Cancel != nil {
		up.Cancel()
		log.Info("sent cancellation signal")
	}

	if up.Sy != nil {
		log.Infof("stopping syncthing")
		if err := up.Sy.SoftTerminate(); err != nil {
			log.Infof("failed to stop syncthing during shutdown: %s", err.Error())
		}
	}

	log.Infof("stopping forwarders")
	if up.Forwarder != nil {
		up.Forwarder.Stop()
	}

	log.Info("completed shutdown sequence")
	up.ShutdownCompleted <- true

}

func printDisplayContext(dev *model.Dev, divertURL string) {
	log.Println(fmt.Sprintf("    %s   %s", log.BlueString("Context:"), okteto.RemoveSchema(dev.Context)))
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Namespace:"), dev.Namespace))
	log.Println(fmt.Sprintf("    %s      %s", log.BlueString("Name:"), dev.Name))

	if len(dev.Forward) > 0 {
		if dev.Forward[0].Service {
			log.Println(fmt.Sprintf("    %s   %d -> %s:%d", log.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].ServiceName, dev.Forward[0].Remote))
		} else {
			log.Println(fmt.Sprintf("    %s   %d -> %d", log.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].Remote))
		}

		for i := 1; i < len(dev.Forward); i++ {
			if dev.Forward[i].Service {
				log.Println(fmt.Sprintf("               %d -> %s:%d", dev.Forward[i].Local, dev.Forward[i].ServiceName, dev.Forward[i].Remote))
				continue
			}
			log.Println(fmt.Sprintf("               %d -> %d", dev.Forward[i].Local, dev.Forward[i].Remote))
		}
	}

	if len(dev.Reverse) > 0 {
		log.Println(fmt.Sprintf("    %s   %d <- %d", log.BlueString("Reverse:"), dev.Reverse[0].Local, dev.Reverse[0].Remote))
		for i := 1; i < len(dev.Reverse); i++ {
			log.Println(fmt.Sprintf("               %d <- %d", dev.Reverse[i].Local, dev.Reverse[i].Remote))
		}
	}

	if divertURL != "" {
		log.Println(fmt.Sprintf("    %s       %s", log.BlueString("URL:"), divertURL))
	}
	fmt.Println()
}
