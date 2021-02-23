// Copyright 2020 The Okteto Authors
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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/pkg/term"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/exec"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/registry"
	"github.com/okteto/okteto/pkg/ssh"

	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

//Up starts a development container
func Up() *cobra.Command {
	var devPath string
	var namespace string
	var k8sContext string
	var remote int
	var autoDeploy bool
	var build bool
	var forcePull bool
	var resetSyncthing bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activates your development container",
		RunE: func(cmd *cobra.Command, args []string) error {

			if okteto.InDevContainer() {
				return errors.ErrNotInDevContainer
			}

			u := upgradeAvailable()
			if len(u) > 0 {
				warningFolder := filepath.Join(config.GetOktetoHome(), ".warnings")
				if utils.GetWarningState(warningFolder, "version") != u {
					log.Yellow("Okteto %s is available. To upgrade:", u)
					log.Yellow("    %s", getUpgradeCommand())
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
				log.Yellow(`The 'deploy' flag is deprecated and will be removed in a future release.
Set the 'autocreate' field in your okteto manifest to get the same behavior.
More information is available here: https://okteto.com/docs/reference/cli#up`)
			}

			dev, err := loadDevOrInit(namespace, k8sContext, devPath)
			if err != nil {
				return err
			}

			if err := loadDevOverrides(dev, namespace, k8sContext, forcePull, remote, autoDeploy); err != nil {
				return err
			}

			if err := checkStignoreConfiguration(dev); err != nil {
				log.Infof("failed to check '.stignore' configuration: %s", err.Error())
			}

			if _, ok := os.LookupEnv("OKTETO_AUTODEPLOY"); ok {
				autoDeploy = true
			}

			up := &upContext{
				Dev:            dev,
				Exit:           make(chan error, 1),
				resetSyncthing: resetSyncthing,
			}
			up.inFd, up.isTerm = term.GetFdInfo(os.Stdin)
			if up.isTerm {
				var err error
				up.stateTerm, err = term.SaveState(up.inFd)
				if err != nil {
					log.Infof("failed to save the state of the terminal: %s", err.Error())
					return fmt.Errorf("failed to save the state of the terminal")
				}
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
	cmd.Flags().BoolVarP(&resetSyncthing, "reset", "", false, "reset the file synchronization database")
	return cmd
}

func loadDevOrInit(namespace, k8sContext, devPath string) (*model.Dev, error) {
	dev, err := utils.LoadDev(devPath)

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
	return utils.LoadDev(devPath)
}

func loadDevOverrides(dev *model.Dev, namespace, k8sContext string, forcePull bool, remote int, autoDeploy bool) error {

	dev.LoadContext(namespace, k8sContext)

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

	return nil
}

func (up *upContext) start(autoDeploy, build bool) error {

	var namespace string
	var err error

	up.Client, up.RestConfig, namespace, err = k8Client.GetLocal(up.Dev.Context)
	if err != nil {
		kubecfg := config.GetKubeConfigFile()
		log.Infof("failed to load local Kubeconfig: %s", err)
		if up.Dev.Context == "" {
			return fmt.Errorf("failed to load your local Kubeconfig %q", kubecfg)
		}
		return fmt.Errorf("failed to load your local Kubeconfig: %q context not found in %q", up.Dev.Context, kubecfg)
	}

	if up.Dev.Namespace == "" {
		up.Dev.Namespace = namespace
	}

	ctx := context.Background()
	ns, err := namespaces.Get(ctx, up.Dev.Namespace, up.Client)
	if err != nil {
		log.Infof("failed to get namespace %s: %s", up.Dev.Namespace, err)
		return fmt.Errorf("couldn't get namespace/%s, please try again", up.Dev.Namespace)
	}

	if !namespaces.IsOktetoAllowed(ns) {
		return fmt.Errorf("'okteto up' is not allowed in the current namespace")
	}

	up.isOktetoNamespace = namespaces.IsOktetoNamespace(ns)

	if err := createPIDFile(up.Dev.Namespace, up.Dev.Name); err != nil {
		log.Infof("failed to create pid file for %s - %s: %s", up.Dev.Namespace, up.Dev.Name, err)
		return fmt.Errorf("couldn't create pid file for %s - %s", up.Dev.Namespace, up.Dev.Name)
	}

	defer cleanPIDFile(up.Dev.Namespace, up.Dev.Name)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	analytics.TrackUp(true, up.Dev.Name, up.getInteractive(), len(up.Dev.Services) == 0, up.isSwap, up.Dev.RemoteModeEnabled())

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

func (up *upContext) activate(autoDeploy, build bool) error {
	log.Infof("activating development container retry=%t", up.isRetry)
	// create a new context on every iteration
	ctx, cancel := context.WithCancel(context.Background())
	up.Cancel = cancel
	up.ShutdownCompleted = make(chan bool, 1)
	up.Sy = nil
	up.Forwarder = nil
	defer up.shutdown()

	up.Disconnect = make(chan error, 1)
	up.CommandResult = make(chan error, 1)
	up.cleaned = make(chan string, 1)
	up.hardTerminate = make(chan error, 1)

	d, create, err := up.getCurrentDeployment(ctx, autoDeploy)
	if err != nil {
		return err
	}

	if up.isRetry && !deployments.IsDevModeOn(d) {
		log.Information("Development container has been deactivated")
		return nil
	}

	if deployments.IsDevModeOn(d) && deployments.HasBeenChanged(d) {
		return errors.UserError{
			E: fmt.Errorf("Deployment '%s' has been modified while your development container was active", d.Name),
			Hint: `Follow these steps:
	  1. Execute 'okteto down'
	  2. Apply your manifest changes again: 'kubectl apply'
	  3. Execute 'okteto up' again
    More information is available here: https://okteto.com/docs/reference/known-issues/index.html#kubectl-apply-changes-are-undone-by-okteto-up`,
		}
	}

	if _, err := registry.GetImageTagWithDigest(ctx, up.Dev.Namespace, up.Dev.Image.Name); err == errors.ErrNotFound {
		log.Infof("image '%s' not found, building it: %s", up.Dev.Image.Name, err.Error())
		build = true
	}

	if !up.isRetry && build {
		if err := up.buildDevImage(ctx, d, create); err != nil {
			return fmt.Errorf("error building dev image: %s", err)
		}
	}

	go up.initializeSyncthing()

	if err := up.setDevContainer(d); err != nil {
		return err
	}

	if err := up.devMode(ctx, d, create); err != nil {
		return fmt.Errorf("couldn't activate your development container (%s): %s", up.Dev.Container, err.Error())
	}

	log.Success("Development container activated")
	up.isRetry = true

	if err := up.forwards(ctx); err != nil {
		if err == errors.ErrSSHConnectError {
			err := up.checkOktetoStartError(ctx, "Failed to connect to your development container")
			if err == errors.ErrLostSyncthing {
				if err := pods.Destroy(ctx, up.Pod, up.Dev.Namespace, up.Client); err != nil {
					return fmt.Errorf("error recreating development container: %s", err.Error())
				}
			}
			return err
		}
		return fmt.Errorf("couldn't connect to your development container: %s", err.Error())
	}
	log.Success("Connected to your development container")

	go up.cleanCommand(ctx)

	if err := up.sync(ctx); err != nil {
		if up.shouldRetry(ctx, err) {
			if pods.Exists(ctx, up.Pod, up.Dev.Namespace, up.Client) {
				up.resetSyncthing = true
			}
			return errors.ErrLostSyncthing
		}
		return err
	}

	up.success = true
	if up.isRetry {
		analytics.TrackReconnect(true, up.isSwap)
	}
	log.Success("Files synchronized")

	go func() {
		output := <-up.cleaned
		log.Debugf("clean command output: %s", output)

		if isWatchesConfigurationTooLow(output) {
			folder := config.GetNamespaceHome(up.Dev.Namespace)
			if utils.GetWarningState(folder, ".remotewatcher") == "" {
				log.Yellow("The value of /proc/sys/fs/inotify/max_user_watches in your cluster nodes is too low.")
				log.Yellow("This can affect file synchronization performance.")
				log.Yellow("Visit https://okteto.com/docs/reference/known-issues/index.html for more information.")
				if err := utils.SetWarningState(folder, ".remotewatcher", "true"); err != nil {
					log.Infof("failed to set warning remotewatcher state: %s", err.Error())
				}
			}
		}

		printDisplayContext(up.Dev)
		up.CommandResult <- up.runCommand(ctx)
	}()

	prevError := up.waitUntilExitOrInterrupt()

	if up.shouldRetry(ctx, prevError) {
		if !up.Dev.PersistentVolumeEnabled() {
			if err := pods.Destroy(ctx, up.Pod, up.Dev.Namespace, up.Client); err != nil {
				return err
			}
		}
		return errors.ErrLostSyncthing
	}

	return prevError
}

func (up *upContext) shouldRetry(ctx context.Context, err error) bool {
	switch err {
	case nil:
		return false
	case errors.ErrResetSyncthing:
		up.resetSyncthing = true
		return true
	case errors.ErrLostSyncthing:
		return true
	case errors.ErrCommandFailed:
		return !up.Sy.Ping(ctx, false)
	}

	return false
}

func (up *upContext) getCurrentDeployment(ctx context.Context, autoDeploy bool) (*appsv1.Deployment, bool, error) {
	d, err := deployments.Get(ctx, up.Dev, up.Dev.Namespace, up.Client)
	if err == nil {
		if d.Annotations[model.OktetoAutoCreateAnnotation] != model.OktetoUpCmd {
			up.isSwap = true
		}
		return d, false, nil
	}

	if !errors.IsNotFound(err) || up.isRetry {
		return nil, false, fmt.Errorf("couldn't get deployment %s/%s, please try again: %s", up.Dev.Namespace, up.Dev.Name, err)
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
    More information is available here: https://okteto.com/docs/reference/cli#up`,
		}
		return nil, false, err
	}

	return up.Dev.GevSandbox(), true, nil
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

func (up *upContext) buildDevImage(ctx context.Context, d *appsv1.Deployment, create bool) error {
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
		devContainer := deployments.GetDevContainer(&d.Spec.Template.Spec, up.Dev.Container)
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
	if err := buildCMD.Run(ctx, up.Dev.Namespace, buildKitHost, isOktetoCluster, up.Dev.Image.Context, up.Dev.Image.Dockerfile, imageTag, up.Dev.Image.Target, false, up.Dev.Image.CacheFrom, buildArgs, "tty"); err != nil {
		return fmt.Errorf("error building dev image '%s': %s", imageTag, err)
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

func (up *upContext) setDevContainer(d *appsv1.Deployment) error {
	devContainer := deployments.GetDevContainer(&d.Spec.Template.Spec, up.Dev.Container)
	if devContainer == nil {
		return fmt.Errorf("container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
	}

	up.Dev.Container = devContainer.Name

	if up.Dev.Image.Name == "" {
		up.Dev.Image.Name = devContainer.Image
	}

	return nil
}

func (up *upContext) devMode(ctx context.Context, d *appsv1.Deployment, create bool) error {
	spinner := utils.NewSpinner("Activating your development container...")
	if err := config.UpdateStateFile(up.Dev, config.Activating); err != nil {
		return err
	}
	spinner.Start()
	defer spinner.Stop()

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.Create(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	if err := config.UpdateStateFile(up.Dev, config.Starting); err != nil {
		return err
	}

	trList, err := deployments.GetTranslations(ctx, up.Dev, d, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.TranslateDevMode(trList, up.Client, up.isOktetoNamespace); err != nil {
		return err
	}

	initSyncErr := <-up.hardTerminate
	if initSyncErr != nil {
		return initSyncErr
	}

	log.Info("create deployment secrets")
	if err := secrets.Create(ctx, up.Dev, up.Client, up.Sy); err != nil {
		return err
	}

	for name := range trList {
		if name == d.Name {
			if err := deployments.Deploy(ctx, trList[name].Deployment, create, up.Client); err != nil {
				return err
			}
		} else {
			if err := deployments.Deploy(ctx, trList[name].Deployment, false, up.Client); err != nil {
				return err
			}
		}

		if trList[name].Deployment.Annotations[okLabels.DeploymentAnnotation] == "" {
			continue
		}

		if err := deployments.UpdateOktetoRevision(ctx, trList[name].Deployment, up.Client); err != nil {
			return err
		}

	}

	if create {
		if err := services.CreateDev(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	pod, err := pods.GetDevPodInLoop(ctx, up.Dev, up.Client, create)
	if err != nil {
		return err
	}

	reporter := make(chan string)
	defer close(reporter)
	go func() {
		message := "Activating your development container"
		if up.Dev.PersistentVolumeEnabled() {
			message = "Attaching persistent volume"
			if err := config.UpdateStateFile(up.Dev, config.Attaching); err != nil {
				log.Infof("error updating state: %s", err.Error())
			}
		}
		for {
			spinner.Update(fmt.Sprintf("%s...", message))
			message = <-reporter
			if message == "" {
				return
			}
			if strings.HasPrefix(message, "Pulling") {
				if err := config.UpdateStateFile(up.Dev, config.Pulling); err != nil {
					log.Infof("error updating state: %s", err.Error())
				}
			}
		}
	}()

	if err := pods.WaitUntilRunning(ctx, up.Dev, pod.Name, up.Client, reporter); err != nil {
		return err
	}

	up.Pod = pod.Name
	return nil
}

func (up *upContext) forwards(ctx context.Context) error {
	spinner := utils.NewSpinner("Connecting to your development container...")
	spinner.Start()
	defer spinner.Stop()

	if up.Dev.RemoteModeEnabled() {
		return up.sshForwards(ctx)
	}

	log.Infof("starting port forwards")
	up.Forwarder = forward.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client)

	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f); err != nil {
			return err
		}
	}

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	return up.Forwarder.Start(up.Pod, up.Dev.Namespace)
}

func (up *upContext) sshForwards(ctx context.Context) error {
	log.Infof("starting SSH port forwards")
	f := forward.NewPortForwardManager(ctx, up.Dev.Interface, up.RestConfig, up.Client)
	if err := f.Add(model.Forward{Local: up.Dev.RemotePort, Remote: up.Dev.SSHServerPort}); err != nil {
		return err
	}

	up.Forwarder = ssh.NewForwardManager(ctx, fmt.Sprintf(":%d", up.Dev.RemotePort), up.Dev.Interface, "0.0.0.0", f)

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemotePort, Remote: syncthing.ClusterPort}); err != nil {
		return err
	}

	if err := up.Forwarder.Add(model.Forward{Local: up.Sy.RemoteGUIPort, Remote: syncthing.GUIPort}); err != nil {
		return err
	}

	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f); err != nil {
			return err
		}
	}

	for _, r := range up.Dev.Reverse {
		if err := up.Forwarder.AddReverse(r); err != nil {
			return err
		}
	}

	if err := ssh.AddEntry(up.Dev.Name, up.Dev.Interface, up.Dev.RemotePort); err != nil {
		log.Infof("failed to add entry to your SSH config file: %s", err)
		return fmt.Errorf("failed to add entry to your SSH config file")
	}

	return up.Forwarder.Start(up.Pod, up.Dev.Namespace)
}

func (up *upContext) initializeSyncthing() error {
	sy, err := syncthing.New(up.Dev)
	if err != nil {
		return err
	}

	up.Sy = sy

	log.Infof("local syncthing intialized: gui -> %d, sync -> %d", up.Sy.LocalGUIPort, up.Sy.LocalPort)
	log.Infof("remote syncthing intialized: gui -> %d, sync -> %d", up.Sy.RemoteGUIPort, up.Sy.RemotePort)

	if err := up.Sy.SaveConfig(up.Dev); err != nil {
		log.Infof("error saving syncthing object: %s", err)
	}

	up.hardTerminate <- up.Sy.HardTerminate()

	return nil
}

func (up *upContext) sync(ctx context.Context) error {
	if err := up.startSyncthing(ctx); err != nil {
		return err
	}

	return up.synchronizeFiles(ctx)
}

func (up *upContext) startSyncthing(ctx context.Context) error {
	spinner := utils.NewSpinner("Starting the file synchronization service...")
	spinner.Start()
	if err := config.UpdateStateFile(up.Dev, config.StartingSync); err != nil {
		return err
	}
	defer spinner.Stop()

	if err := up.Sy.Run(ctx); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, true); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, false); err != nil {
		log.Infof("failed to ping syncthing: %s", err.Error())
		err = up.checkOktetoStartError(ctx, "Failed to connect to the synchronization service")
		if err == errors.ErrLostSyncthing {
			if err := pods.Destroy(ctx, up.Pod, up.Dev.Namespace, up.Client); err != nil {
				return fmt.Errorf("error recreating development container: %s", err.Error())
			}
		}
		return err
	}

	if up.resetSyncthing {
		spinner.Update("Resetting synchronization service database...")
		if err := up.Sy.ResetDatabase(ctx, up.Dev, false); err != nil {
			return err
		}
		if err := up.Sy.ResetDatabase(ctx, up.Dev, true); err != nil {
			return err
		}

		if err := up.Sy.WaitForPing(ctx, false); err != nil {
			return err
		}
		if err := up.Sy.WaitForPing(ctx, true); err != nil {
			return err
		}

		up.resetSyncthing = false
	}

	up.Sy.SendStignoreFile(ctx)
	spinner.Update("Scanning file system...")
	if err := up.Sy.WaitForScanning(ctx, up.Dev, true); err != nil {
		return err
	}

	if !up.Dev.PersistentVolumeEnabled() {
		if err := up.Sy.WaitForScanning(ctx, up.Dev, false); err != nil {
			log.Infof("failed to wait for syncthing scanning: %s", err.Error())
			return up.checkOktetoStartError(ctx, "Failed to connect to the synchronization service")
		}
	}

	return nil
}

func (up *upContext) synchronizeFiles(ctx context.Context) error {
	suffix := "Synchronizing your files..."
	spinner := utils.NewSpinner(suffix)
	pbScaling := 0.30

	if err := config.UpdateStateFile(up.Dev, config.Synchronizing); err != nil {
		return err
	}
	spinner.Start()
	defer spinner.Stop()
	reporter := make(chan float64)
	go func() {
		<-time.NewTicker(2 * time.Second).C
		var previous float64

		for c := range reporter {
			if c > previous {
				// todo: how to calculate how many characters can the line fit?
				pb := utils.RenderProgressBar(suffix, c, pbScaling)
				spinner.Update(pb)
				previous = c
			}
		}
	}()

	if err := up.Sy.WaitForCompletion(ctx, up.Dev, reporter); err != nil {
		analytics.TrackSyncError()
		switch err {
		case errors.ErrLostSyncthing, errors.ErrResetSyncthing:
			return err
		case errors.ErrInsufficientSpace:
			return up.getInsufficientSpaceError(err)
		default:
			return errors.UserError{
				E: err,
				Hint: `Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
    Please include the file generated by 'okteto doctor' if possible.
    Then, try to run 'okteto down -v' + 'okteto up' again`,
			}
		}
	}

	// render to 100
	spinner.Update(utils.RenderProgressBar(suffix, 100, pbScaling))

	up.Sy.Type = "sendreceive"
	up.Sy.IgnoreDelete = false
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}

	go up.Sy.Monitor(ctx, up.Disconnect)
	go up.Sy.MonitorStatus(ctx, up.Disconnect)
	log.Infof("restarting syncthing to update sync mode to sendreceive")
	return up.Sy.Restart(ctx)
}

func (up *upContext) cleanCommand(ctx context.Context) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	cmd := "cat /proc/sys/fs/inotify/max_user_watches; /var/okteto/bin/clean >/dev/null 2>&1"

	err := exec.Exec(
		ctx,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod,
		up.Dev.Container,
		false,
		in,
		&out,
		os.Stderr,
		[]string{"sh", "-c", cmd},
	)

	if err != nil {
		log.Infof("failed to clean session: %s", err)
	}

	up.cleaned <- out.String()
}

func (up *upContext) runCommand(ctx context.Context) error {
	log.Infof("starting remote command")
	if err := config.UpdateStateFile(up.Dev, config.Ready); err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		return ssh.Exec(ctx, up.Dev.Interface, up.Dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, up.Dev.Command.Values)
	}

	return exec.Exec(
		ctx,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod,
		up.Dev.Container,
		true,
		os.Stdin,
		os.Stdout,
		os.Stderr,
		up.Dev.Command.Values,
	)
}

func (up *upContext) checkOktetoStartError(ctx context.Context, msg string) error {
	userID := pods.GetDevPodUserID(ctx, up.Dev, up.Client)
	if up.Dev.PersistentVolumeEnabled() {
		if userID != -1 && userID != *up.Dev.SecurityContext.RunAsUser {
			return errors.UserError{
				E: fmt.Errorf("User %d doesn't have write permissions for the %s directory", userID, up.Dev.MountPath),
				Hint: fmt.Sprintf(`Set 'securityContext.runAsUser: %d' in your okteto manifest.
    After that, run 'okteto down -v' to reset your development container and run 'okteto up' again`, userID),
			}
		}
	} else {
		if pods.OktetoDevPodMustBeRecreated(ctx, up.Dev, up.Client) {
			return errors.ErrLostSyncthing
		}
	}

	if len(up.Dev.Secrets) > 0 {
		return errors.UserError{
			E: fmt.Errorf(msg),
			Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s',
    Check that your container can write to the destination path of your secrets.
    Run 'okteto down -v' to reset your development container and try again`, up.Pod),
		}
	}
	return errors.UserError{
		E: fmt.Errorf(msg),
		Hint: fmt.Sprintf(`Check your development container logs for errors: 'kubectl logs %s'.
    Run 'okteto down -v' to reset your development container and try again`, up.Pod),
	}
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
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest#persistentvolume-object-optional`,
		}
	}
	return errors.UserError{
		E: err,
		Hint: `The synchronization service is running out of space.
    Enable persistent volumes in your okteto manifest and try again.
    More information about configuring your persistent volume at https://okteto.com/docs/reference/manifest#persistentvolume-object-optional`,
	}

}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *upContext) shutdown() {
	if up.isTerm {
		if err := term.RestoreTerminal(up.inFd, up.stateTerm); err != nil {
			log.Infof("failed to restore terminal: %s", err.Error())
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
	if err := config.DeleteStateFile(up.Dev); err != nil {
		log.Infof("failed to delete state file: %s", err.Error())
	}

	log.Info("completed shutdown sequence")
	up.ShutdownCompleted <- true

}

func printDisplayContext(dev *model.Dev) {
	if dev.Context != "" {
		log.Println(fmt.Sprintf("    %s   %s", log.BlueString("Context:"), dev.Context))
	}
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Namespace:"), dev.Namespace))
	log.Println(fmt.Sprintf("    %s      %s", log.BlueString("Name:"), dev.Name))

	if len(dev.Forward) > 0 {
		log.Println(fmt.Sprintf("    %s   %d -> %d", log.BlueString("Forward:"), dev.Forward[0].Local, dev.Forward[0].Remote))
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
	fmt.Println()
}
