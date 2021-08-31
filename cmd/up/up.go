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

	"github.com/moby/term"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	k8sClient "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/diverts"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

// Up starts a development container
func Up() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var remote int
	var autoDeploy bool
	var build bool
	var forcePull bool
	var reset bool
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

			checkLocalWatchesConfiguration()

			if autoDeploy {
				log.Warning(`The 'deploy' flag is deprecated and will be removed in a future release.
    Set the 'autocreate' field in your okteto manifest to get the same behavior.
    More information is available here: https://okteto.com/docs/reference/cli/#up`)
			}

			ctx := context.Background()
			if err := utils.LoadEnvironment(ctx, false); err != nil {
				return err
			}

			dev, err := loadDevOrInit(namespace, k8sContext, devPath)
			if err != nil {
				return err
			}

			if err := loadDevOverrides(dev, forcePull, remote, autoDeploy); err != nil {
				return err
			}

			log.ConfigureFileLogger(config.GetDeploymentHome(dev.Namespace, dev.Name), config.VersionString)

			if err := checkStignoreConfiguration(dev); err != nil {
				log.Infof("failed to check '.stignore' configuration: %s", err.Error())
			}

			if err := addStignoreSecrets(dev); err != nil {
				return err
			}

			if _, ok := os.LookupEnv("OKTETO_AUTODEPLOY"); ok {
				autoDeploy = true
			}

			up := &upContext{
				Dev:            dev,
				Exit:           make(chan error, 1),
				resetSyncthing: reset,
				StartTime:      time.Now(),
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

			err = up.start(autoDeploy, build)
			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the up command is executed")
	cmd.Flags().IntVarP(&remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&autoDeploy, "deploy", "d", false, "create deployment when it doesn't exist in a namespace")
	cmd.Flags().MarkHidden("deploy")
	cmd.Flags().BoolVarP(&build, "build", "", false, "build on-the-fly the dev image using the info provided by the 'build' okteto manifest field")
	cmd.Flags().BoolVarP(&forcePull, "pull", "", false, "force dev image pull")
	cmd.Flags().BoolVarP(&reset, "reset", "", false, "reset the file synchronization database")
	return cmd
}

func loadDevOrInit(namespace, k8sContext, devPath string) (*model.Dev, error) {
	dev, err := utils.LoadDev(devPath, namespace, k8sContext)

	if err == nil {
		return dev, nil
	}
	if !strings.Contains(err.Error(), "okteto init") {
		return nil, err
	}
	if !utils.AskIfOktetoInit(devPath) {
		return nil, err
	}

	workDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("unknown current folder: %s", err)
	}
	if err := initCMD.Run(namespace, k8sContext, devPath, "", workDir, false); err != nil {
		return nil, err
	}

	log.Success(fmt.Sprintf("okteto manifest (%s) created", devPath))
	return utils.LoadDev(devPath, namespace, k8sContext)
}

func loadDevOverrides(dev *model.Dev, forcePull bool, remote int, autoDeploy bool) error {
	if remote > 0 {
		dev.RemotePort = remote
	}

	if dev.RemoteModeEnabled() {
		if err := sshKeys(); err != nil {
			return err
		}

		dev.LoadRemote(ssh.GetPublicKey())
	}

	if !dev.Autocreate {
		dev.Autocreate = autoDeploy
	}

	if forcePull {
		dev.LoadForcePull()
	}

	dev.Username = okteto.GetUsername()
	if registryURL, err := okteto.GetRegistry(); err == nil {
		dev.RegistryURL = registryURL
	}

	return nil
}

func (up *upContext) start(autoDeploy, build bool) error {
	var err error
	up.Client, up.RestConfig, err = k8sClient.GetLocalWithContext(up.Dev.Context)
	if err != nil {
		kubecfg := config.GetKubeConfigFile()
		log.Infof("failed to load local Kubeconfig: %s", err)
		if up.Dev.Context == "" {
			return fmt.Errorf("failed to load your local Kubeconfig %q", kubecfg)
		}
		return fmt.Errorf("failed to load your local Kubeconfig: %q context not found in %q", up.Dev.Context, kubecfg)
	}

	ctx := context.Background()
	ns, err := namespaces.Get(ctx, up.Dev.Namespace, up.Client)
	if err != nil {
		return err
	}

	if !namespaces.IsOktetoAllowed(ns) {
		return fmt.Errorf("'okteto up' is not allowed in the current namespace")
	}

	up.isOktetoNamespace = namespaces.IsOktetoNamespace(ns)

	if up.Dev.Divert != nil {
		if err := diverts.Create(ctx, up.Dev, up.isOktetoNamespace, up.Client); err != nil {
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

	analytics.TrackUp(true, up.Dev.Name, up.getInteractive(), len(up.Dev.Services) == 0, up.isSwap, up.Dev.Divert != nil)

	go up.activateLoop(autoDeploy, build)

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
func (up *upContext) activateLoop(autoDeploy, build bool) {
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

		err := up.activate(autoDeploy, build)
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

func (up *upContext) getCurrentK8sObject(ctx context.Context, autoDeploy bool) (*model.K8sObject, bool, error) {
	k8sObject, err := apps.GetResource(ctx, up.Dev, up.Dev.Namespace, up.Client)
	if err == nil {
		if k8sObject.GetAnnotation(model.OktetoAutoCreateAnnotation) != model.OktetoUpCmd {
			up.isSwap = true
		}
		return k8sObject, false, nil
	}

	if !errors.IsNotFound(err) || up.isRetry {
		return nil, false, fmt.Errorf("couldn't get %s %s/%s, please try again: %s", strings.ToLower(string(k8sObject.ObjectType)), up.Dev.Namespace, up.Dev.Name, err)
	}

	if len(up.Dev.Labels) > 0 {
		if err == errors.ErrNotFound {
			err = errors.UserError{
				E:    fmt.Errorf("Didn't find a deployment in namespace %s that matches the labels in your Okteto manifest", up.Dev.Namespace),
				Hint: "Update your labels or use 'okteto namespace' to select a different namespace and try again"}
		}
		return nil, false, err
	}

	if !up.Dev.Autocreate {
		err = errors.UserError{
			E: fmt.Errorf("Deployment '%s' not found in namespace '%s'", up.Dev.Name, up.Dev.Namespace),
			Hint: `Verify that your application has been deployed and your Kubernetes context is pointing to the right namespace
    Or set the 'autocreate' field in your okteto manifest if you want to create a standalone development container
    More information is available here: https://okteto.com/docs/reference/cli/#up`,
		}
		return nil, false, err
	}
	k8sObject.GetSandbox()
	return k8sObject, true, nil
}

// waitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *upContext) waitUntilExitOrInterrupt() error {
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
		}
	}
}

func (up *upContext) buildDevImage(ctx context.Context, k8sObject *model.K8sObject, create bool) error {
	if _, err := os.Stat(up.Dev.Image.Dockerfile); err != nil {
		return errors.UserError{
			E:    fmt.Errorf("'--build' argument given but there is no Dockerfile"),
			Hint: "Try creating a Dockerfile or specify 'context' and 'dockerfile' fields.",
		}
	}

	oktetoRegistryURL := ""
	if up.isOktetoNamespace {
		var err error
		oktetoRegistryURL, err = okteto.GetRegistry()
		if err != nil {
			return err
		}
	}

	if oktetoRegistryURL == "" && create && up.Dev.Image.Name == "" {
		return fmt.Errorf("no value for 'Image' has been provided in your okteto manifest")
	}

	if up.Dev.Image.Name == "" {
		devContainer := apps.GetDevContainer(&k8sObject.PodTemplateSpec.Spec, up.Dev.Container)
		if devContainer == nil {
			return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
		}
		up.Dev.Image.Name = devContainer.Image
	}

	buildKitHost, isOktetoCluster, err := buildCMD.GetBuildKitHost()
	if err != nil {
		return err
	}
	log.Information("Running your build in %s...", buildKitHost)

	imageTag := registry.GetImageTag(up.Dev.Image.Name, up.Dev.Name, up.Dev.Namespace, oktetoRegistryURL)
	log.Infof("building dev image tag %s", imageTag)

	buildArgs := model.SerializeBuildArgs(up.Dev.Image.Args)
	if err := buildCMD.Run(ctx, up.Dev.Namespace, buildKitHost, isOktetoCluster, up.Dev.Image.Context, up.Dev.Image.Dockerfile, imageTag, up.Dev.Image.Target, false, up.Dev.Image.CacheFrom, buildArgs, nil, "tty"); err != nil {
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

func (up *upContext) setDevContainer(k8sObject *model.K8sObject) error {
	devContainer := apps.GetDevContainer(&k8sObject.PodTemplateSpec.Spec, up.Dev.Container)
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
			Hint: `Okteto volume is full.
    Increase your persistent volume size, run 'okteto down -v' and try 'okteto up' again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest/#persistentvolume-object-optional`,
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
		analytics.TrackUpError(true, up.isSwap)
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
	if dev.Context != "" {
		log.Println(fmt.Sprintf("    %s   %s", log.BlueString("Context:"), dev.Context))
	}
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
