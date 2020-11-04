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
	"net/url"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/docker/docker/pkg/term"
	initCMD "github.com/okteto/okteto/cmd/init"
	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/analytics"
	buildCMD "github.com/okteto/okteto/pkg/cmd/build"
	"github.com/okteto/okteto/pkg/config"
	okErrors "github.com/okteto/okteto/pkg/errors"
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
	"github.com/okteto/okteto/pkg/ssh"

	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

var (
	localClusters = []string{"127.", "172.", "192.", "169.", model.Localhost, "::1", "fe80::", "fc00::"}
)

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
			log.Info("starting up command")

			if okteto.InDevContainer() {
				return okErrors.ErrNotInDevContainer
			}

			u := upgradeAvailable()
			if len(u) > 0 {
				log.Yellow("Okteto %s is available. To upgrade:", u)
				log.Yellow("    %s", getUpgradeCommand())
				fmt.Println()
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

			dev, err := loadDevOrInit(namespace, k8sContext, devPath)
			if err != nil {
				return err
			}

			if err := loadDevOverrides(dev, namespace, k8sContext, forcePull, remote); err != nil {
				return err
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
			log.Debug("completed up command")
			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", utils.DefaultDevManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().StringVarP(&k8sContext, "context", "c", "", "context where the up command is executed")
	cmd.Flags().IntVarP(&remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&autoDeploy, "deploy", "d", false, "create deployment when it doesn't exist in a namespace")
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

func loadDevOverrides(dev *model.Dev, namespace, k8sContext string, forcePull bool, remote int) error {

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

	analytics.TrackUp(true, up.Dev.Name, up.getClusterType(), up.getInteractive(), len(up.Dev.Services) == 0, up.isSwap, up.Dev.RemoteModeEnabled())

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
	isRetry := false
	t := time.NewTicker(3 * time.Second)
	for {
		err := up.activate(isRetry, autoDeploy, build)
		if err != nil {
			log.Infof("activate failed with %s", err)

			if err == okErrors.ErrLostSyncthing {
				isRetry = true
				continue
			}

			if okErrors.IsTransient(err) {
				log.Yellow("Connection lost to your development container, reconnecting...")
				<-t.C
				continue
			}

			up.Exit <- err
			return
		}
		up.Exit <- nil
		return
	}
}

func (up *upContext) activate(isRetry, autoDeploy, build bool) error {
	log.Infof("activating development container retry=%t", isRetry)
	// create a new context on every iteration
	ctx, cancel := context.WithCancel(context.Background())
	up.Cancel = cancel
	up.Canceled = false

	up.Disconnect = make(chan error, 1)
	up.CommandResult = make(chan error, 1)
	up.cleaned = make(chan string, 1)

	d, create, err := up.getCurrentDeployment(ctx, autoDeploy, isRetry)
	if err != nil {
		return err
	}

	if isRetry && !deployments.IsDevModeOn(d) {
		log.Information("Development container has been deactivated")
		return nil
	}

	if deployments.IsDevModeOn(d) && deployments.HasBeenChanged(d) {
		return okErrors.UserError{
			E:    fmt.Errorf("Deployment '%s' has been modified while your development container was active", d.Name),
			Hint: "Follow these steps:\n      1. Execute 'okteto down'\n      2. Apply your manifest changes again: 'kubectl apply'\n      3. Execute 'okteto up' again\n    More information is available here: https://okteto.com/docs/reference/known-issues/index.html#kubectl-apply-changes-are-undone-by-okteto-up",
		}
	}

	if !isRetry && build {
		if err := up.buildDevImage(ctx, d, create); err != nil {
			return fmt.Errorf("error building dev image: %s", err)
		}
	}

	defer up.shutdown()

	if err := up.initializeSyncthing(); err != nil {
		return err
	}

	if err := up.setDevContainer(d); err != nil {
		return err
	}

	if err := up.devMode(ctx, d, create); err != nil {
		return fmt.Errorf("couldn't activate your development container (%s): %s", up.Dev.Container, err.Error())
	}

	if err := up.forwards(ctx); err != nil {
		return fmt.Errorf("couldn't forward traffic to your development container: %s", err.Error())
	}

	go up.cleanCommand(ctx)

	log.Success("Development container activated")

	if err := up.sync(ctx); err != nil {
		if up.shouldRetry(ctx, err) {
			if pods.Exists(ctx, up.Pod, up.Dev.Namespace, up.Client) {
				up.resetSyncthing = true
			}
			log.Yellow("\nConnection lost to your development container, reconnecting...\n")
			return okErrors.ErrLostSyncthing
		}
		return err
	}

	up.success = true
	if isRetry {
		analytics.TrackReconnect(true, up.getClusterType(), up.isSwap)
	}
	log.Success("Files synchronized")

	go func() {
		output := <-up.cleaned
		log.Debugf("clean command output: %s", output)

		if isWatchesConfigurationTooLow(output) {
			log.Yellow("\nThe value of /proc/sys/fs/inotify/max_user_watches in your cluster nodes is too low.")
			log.Yellow("This can affect file synchronization performance.")
			log.Yellow("Visit https://okteto.com/docs/reference/known-issues/index.html for more information.")
		}

		printDisplayContext(up.Dev)
		up.CommandResult <- up.runCommand(ctx)
	}()

	prevError := up.waitUntilExitOrInterrupt()

	if up.shouldRetry(ctx, prevError) {
		log.Yellow("\nConnection lost to your development container, reconnecting...\n")
		if !up.Dev.PersistentVolumeEnabled() {
			if err := pods.Destroy(ctx, up.Pod, up.Dev.Namespace, up.Client); err != nil {
				return err
			}
		}
		return okErrors.ErrLostSyncthing
	}

	return prevError
}

func (up *upContext) shouldRetry(ctx context.Context, err error) bool {
	switch err {
	case nil:
		return false
	case okErrors.ErrResetSyncthing:
		up.resetSyncthing = true
		return true
	case okErrors.ErrLostSyncthing:
		return true
	case okErrors.ErrCommandFailed:
		if up.Sy.Ping(ctx, false) {
			return false
		}
		return true
	}

	return false
}

func (up *upContext) getCurrentDeployment(ctx context.Context, autoDeploy, isRetry bool) (*appsv1.Deployment, bool, error) {
	d, err := deployments.Get(ctx, up.Dev, up.Dev.Namespace, up.Client)
	if err == nil {
		if d.Annotations[model.OktetoAutoCreateAnnotation] != model.OktetoUpCmd {
			up.isSwap = true
		}
		return d, false, nil
	}

	if !okErrors.IsNotFound(err) || isRetry {
		return nil, false, fmt.Errorf("couldn't get deployment %s/%s, please try again: %s", up.Dev.Namespace, up.Dev.Name, err)
	}

	if len(up.Dev.Labels) > 0 {
		if err == okErrors.ErrNotFound {
			err = okErrors.UserError{
				E:    fmt.Errorf("Didn't find a deployment in namespace %s that matches the labels in your Okteto manifest", up.Dev.Namespace),
				Hint: "Update your labels or use `okteto namespace` to select a different namespace and try again"}
		}
		return nil, false, err
	}

	if !autoDeploy {
		if err := utils.AskIfDeploy(up.Dev.Name, up.Dev.Namespace); err != nil {
			return nil, false, err
		}
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
				return okErrors.ErrCommandFailed
			}

			log.Info("command completed")
			return nil

		case err := <-up.Disconnect:
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

	imageTag := buildCMD.GetImageTag(up.Dev.Image.Name, up.Dev.Name, up.Dev.Namespace, oktetoRegistryURL)
	log.Infof("building dev image tag %s", imageTag)

	var imageDigest string
	buildArgs := model.SerializeBuildArgs(up.Dev.Image.Args)
	imageDigest, err = buildCMD.Run(ctx, buildKitHost, isOktetoCluster, up.Dev.Image.Context, up.Dev.Image.Dockerfile, imageTag, up.Dev.Image.Target, false, up.Dev.Image.CacheFrom, buildArgs, "tty")
	if err != nil {
		return fmt.Errorf("error building dev image '%s': %s", imageTag, err)
	}
	if imageDigest != "" {
		imageWithoutTag := buildCMD.GetRepoNameWithoutTag(imageTag)
		imageTag = fmt.Sprintf("%s@%s", imageWithoutTag, imageDigest)
	}
	for _, s := range up.Dev.Services {
		if s.Image.Name == up.Dev.Image.Name {
			s.Image.Name = imageTag
		}
	}
	up.Dev.Image.Name = imageTag
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
	up.updateStateFile(activating)
	spinner.Start()
	defer spinner.Stop()

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.Create(ctx, up.Dev, up.Client); err != nil {
			return err
		}
	}

	up.updateStateFile(starting)

	log.Info("create deployment secrets")
	if err := secrets.Create(ctx, up.Dev, up.Client, up.Sy); err != nil {
		return err
	}

	trList, err := deployments.GetTranslations(ctx, up.Dev, d, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.TranslateDevMode(trList, up.Client, up.isOktetoNamespace); err != nil {
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
			up.updateStateFile(attaching)
		}
		for {
			spinner.Update(fmt.Sprintf("%s...", message))
			message = <-reporter
			if message == "" {
				return
			}
			if strings.HasPrefix(message, "Pulling") {
				up.updateStateFile(pulling)
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

	if err := up.Sy.Stop(true); err != nil {
		log.Infof("failed to stop existing syncthing processes: %s", err)
	}

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
	up.updateStateFile(startingSync)
	defer spinner.Stop()

	if err := up.Sy.Run(ctx); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, true); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(ctx, false); err != nil {
		userID := pods.GetDevPodUserID(ctx, up.Dev, up.Client)
		if up.Dev.PersistentVolumeEnabled() {
			if userID != -1 && userID != *up.Dev.SecurityContext.RunAsUser {
				return okErrors.UserError{
					E:    fmt.Errorf("User %d doesn't have write permissions for the %s directory", userID, up.Dev.MountPath),
					Hint: fmt.Sprintf("Set 'securityContext.runAsUser: %d' in your okteto manifest\n    After that, run 'okteto down -v' to reset the synchronization service and run 'okteto up' again", userID),
				}
			}
		} else {
			if pods.OktetoDevPodMustBeRecreated(ctx, up.Dev, up.Client) {
				if err := pods.Destroy(ctx, up.Pod, up.Dev.Namespace, up.Client); err == nil {
					return okErrors.ErrLostSyncthing
				}
			}
		}

		if len(up.Dev.Secrets) > 0 {
			return okErrors.UserError{
				E:    fmt.Errorf("Failed to connect to the synchronization service"),
				Hint: fmt.Sprintf("Check your development container logs for okErrors: 'kubectl logs %s'\n    Check that your container can write to the destination path of your secrets\n    Run 'okteto down -v' to reset the synchronization service and try again.", up.Pod),
			}
		}
		return okErrors.UserError{
			E:    fmt.Errorf("Failed to connect to the synchronization service"),
			Hint: fmt.Sprintf("Check your development container logs for okErrors: 'kubectl logs %s'\n    Run 'okteto down -v' to reset the synchronization service and try again.", up.Pod),
		}
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

	if err := up.Sy.SendStignoreFile(ctx, up.Dev); err != nil {
		return err
	}

	spinner.Update("Scanning file system...")
	if err := up.Sy.WaitForScanning(ctx, up.Dev, true); err != nil {
		return err
	}

	if !up.Dev.PersistentVolumeEnabled() {
		if err := up.Sy.WaitForScanning(ctx, up.Dev, false); err != nil {
			return err
		}
	}

	return nil
}

func (up *upContext) synchronizeFiles(ctx context.Context) error {
	suffix := "Synchronizing your files..."
	spinner := utils.NewSpinner(suffix)
	pbScaling := 0.30

	up.updateStateFile(synchronizing)
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
		if err == okErrors.ErrUnknownSyncError {
			analytics.TrackSyncError()
			return okErrors.UserError{
				E: err,
				Hint: `Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
Please include the file generated by 'okteto doctor' if possible.
Then, try to run 'okteto down -v' + 'okteto up'  again`,
			}
		}
		return err
	}

	// render to 100
	spinner.Update(utils.RenderProgressBar(suffix, 100, pbScaling))

	if err := up.Sy.SendStignoreFile(ctx, up.Dev); err != nil {
		return err
	}

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

	cmd := "cat /proc/sys/fs/inotify/max_user_watches; (cp /var/okteto/bin/* /usr/local/bin; cp /var/okteto/cloudbin/* /usr/local/bin; /var/okteto/bin/clean) >/dev/null 2>&1"

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
		up.cleaned <- out.String()
		return
	}

	up.cleaned <- out.String()
}

func (up *upContext) runCommand(ctx context.Context) error {
	log.Infof("starting remote command")
	up.updateStateFile(ready)

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

func (up *upContext) getClusterType() string {
	if up.isOktetoNamespace {
		return "okteto"
	}

	u, err := url.Parse(up.RestConfig.Host)
	host := ""
	if err == nil {
		host = u.Hostname()
	} else {
		host = up.RestConfig.Host
	}
	for _, l := range localClusters {
		if strings.HasPrefix(host, l) {
			return "local"
		}
	}
	return "remote"
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

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *upContext) shutdown() {
	if up.Canceled {
		return
	}
	up.Canceled = true

	if up.isTerm {
		if err := term.RestoreTerminal(up.inFd, up.stateTerm); err != nil {
			log.Infof("failed to restore terminal: %s", err)
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
		if err := up.Sy.Stop(false); err != nil {
			log.Infof("failed to stop syncthing during shutdown: %s", err)
		}
	}

	log.Infof("stopping forwarders")
	if up.Forwarder != nil {
		up.Forwarder.Stop()
	}

	log.Info("completed shutdown sequence")
}

func printDisplayContext(dev *model.Dev) {
	if dev.Context != "" {
		log.Println(fmt.Sprintf("    %s   %s", log.BlueString("Context:"), dev.Context))
	}
	log.Println(fmt.Sprintf("    %s %s", log.BlueString("Namespace:"), dev.Namespace))
	log.Println(fmt.Sprintf("    %s      %s", log.BlueString("Name:"), dev.Name))
	if dev.RemoteModeEnabled() {
		log.Println(fmt.Sprintf("    %s       %d -> %d", log.BlueString("SSH:"), dev.RemotePort, dev.SSHServerPort))
	}

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
