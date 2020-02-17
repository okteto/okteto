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

package cmd

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
	"github.com/okteto/okteto/pkg/analytics"
	"github.com/okteto/okteto/pkg/errors"
	k8Client "github.com/okteto/okteto/pkg/k8s/client"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/namespaces"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/secrets"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/k8s/volumes"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/ssh"

	"github.com/okteto/okteto/pkg/k8s/forward"
	"github.com/okteto/okteto/pkg/syncthing"

	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ReconnectingMessage is the message shown when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

var (
	localClusters = []string{"127.", "172.", "192.", "169.", "localhost", "::1", "fe80::", "fc00::"}
)

// UpContext is the common context of all operations performed during
// the up command
type UpContext struct {
	Context    context.Context
	Cancel     context.CancelFunc
	Dev        *model.Dev
	Namespace  *apiv1.Namespace
	isSwap     bool
	retry      bool
	Client     *kubernetes.Clientset
	RestConfig *rest.Config
	Pod        string
	Forwarder  *forward.PortForwardManager
	Disconnect chan struct{}
	Running    chan error
	Exit       chan error
	Sy         *syncthing.Syncthing
	ErrChan    chan error
	cleaned    chan struct{}
	success    bool
}

//Up starts a cloud dev environment
func Up() *cobra.Command {
	var devPath string
	var namespace string
	var remote int
	var autoDeploy bool
	var forcePull bool
	var resetSyncthing bool
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activates your development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting up command")

			if k8Client.InCluster() {
				return errors.ErrNotInCluster
			}

			u := upgradeAvailable()
			if len(u) > 0 {
				log.Yellow("Okteto %s is available. To upgrade:", u)
				log.Yellow("    %s", getUpgradeCommand())
				fmt.Println()
			}

			if syncthingUpgradeAvailable() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					log.Infof("failed to upgrade syncthing: %s", err)

					if !syncthingExists() {
						return fmt.Errorf("couldn't download syncthing, please try again")
					}

					log.Yellow("couldn't download syncthing, will try again later")
					fmt.Println()
				}
			}

			checkLocalWatchesConfiguration()

			dev, err := loadDev(devPath)
			if err != nil {
				return err
			}
			if err := dev.UpdateNamespace(namespace); err != nil {
				return err
			}

			if remote > 0 {
				dev.RemotePort = remote
			}

			err = RunUp(dev, autoDeploy, forcePull, resetSyncthing)
			return err
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", defaultManifest, "path to the manifest file")
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "namespace where the up command is executed")
	cmd.Flags().IntVarP(&remote, "remote", "r", 0, "configures remote execution on the specified port")
	cmd.Flags().BoolVarP(&autoDeploy, "deploy", "d", false, "create deployment when it doesn't exist in a namespace")
	cmd.Flags().BoolVarP(&forcePull, "pull", "", false, "force dev image pull")
	cmd.Flags().BoolVarP(&resetSyncthing, "reset", "", false, "reset the file synchronization database")
	return cmd
}

//RunUp starts the up sequence
func RunUp(dev *model.Dev, autoDeploy bool, forcePull, resetSyncthing bool) error {
	up := &UpContext{
		Dev:  dev,
		Exit: make(chan error, 1),
	}

	defer up.shutdown()

	if up.Dev.RemoteModeEnabled() {
		dev.LoadRemote()
	}

	if forcePull {
		dev.LoadForcePull()
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go up.Activate(autoDeploy, resetSyncthing)
	select {
	case <-stop:
		log.Debugf("CTRL+C received, starting shutdown sequence")
		fmt.Println()
	case err := <-up.Exit:
		if err == nil {
			log.Debugf("exit signal received, starting shutdown sequence")
		} else {
			log.Infof("operation failed: %s", err)
			up.updateStateFile(failed)
			return err
		}
	}
	return nil
}

// Activate activates the dev environment
func (up *UpContext) Activate(autoDeploy, resetSyncthing bool) {
	var state *term.State
	inFd, isTerm := term.GetFdInfo(os.Stdin)
	if isTerm {
		var err error
		state, err = term.SaveState(inFd)
		if err != nil {
			up.Exit <- err
			return
		}
	}

	var namespace string
	var err error
	up.Client, up.RestConfig, namespace, err = k8Client.GetLocal()
	if err != nil {
		up.Exit <- err
		return
	}

	if up.Dev.Namespace == "" {
		up.Dev.Namespace = namespace
	}

	up.Namespace, err = namespaces.Get(up.Dev.Namespace, up.Client)
	if err != nil {
		log.Infof("failed to get namespace %s: %s", up.Dev.Namespace, err)
		up.Exit <- fmt.Errorf("couldn't get namespace/%s, please try again: %s", up.Dev.Namespace, err)
		return
	}

	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		up.Disconnect = make(chan struct{}, 1)
		up.Running = make(chan error, 1)
		up.ErrChan = make(chan error, 1)
		up.cleaned = make(chan struct{}, 1)

		d, create, err := up.getCurrentDeployment(autoDeploy)
		if err != nil {
			log.Infof("failed to get deployment %s/%s: %s", up.Dev.Namespace, up.Dev.Name, err)
			up.Exit <- err
			return
		}

		if !up.retry {
			analytics.TrackUp(true, up.Dev.Name, up.getClusterType(), len(up.Dev.Services) == 0, up.isSwap, up.Dev.RemoteModeEnabled())
		}

		err = up.devMode(d, create)
		if err != nil {
			up.Exit <- fmt.Errorf("couldn't activate your development environment: %s", err)
			return
		}

		log.Success("Development environment activated")

		err = up.sync(resetSyncthing && !up.retry)
		if err != nil {
			if !pods.Exists(up.Pod, up.Dev.Namespace, up.Client) {
				log.Yellow("\nConnection lost to your development environment, reconnecting...\n")
				up.shutdown()
				continue
			}
			up.Exit <- err
			return
		}

		up.success = true
		if up.retry {
			analytics.TrackReconnect(true, up.getClusterType(), up.isSwap)
		}
		up.retry = true

		printDisplayContext("Files synchronized", up.Dev)

		go func() {
			<-up.cleaned
			up.Running <- up.runCommand()
		}()

		prevError := up.WaitUntilExitOrInterrupt()
		if isTerm {
			log.Debug("Restoring terminal")
			if err := term.RestoreTerminal(inFd, state); err != nil {
				log.Infof("failed to restore terminal: %s", err)
			}
		}

		if prevError != nil {
			if prevError == errors.ErrLostConnection || (prevError == errors.ErrCommandFailed && !pods.Exists(up.Pod, up.Dev.Namespace, up.Client)) {
				log.Yellow("\nConnection lost to your development environment, reconnecting...\n")
				up.shutdown()
				continue
			}
		}

		up.Exit <- prevError
		return
	}
}

func (up *UpContext) getCurrentDeployment(autoDeploy bool) (*appsv1.Deployment, bool, error) {
	d, err := deployments.Get(up.Dev, up.Dev.Namespace, up.Client)
	if err == nil {
		if _, ok := d.Annotations[model.OktetoAutoCreateAnnotation]; !ok {
			up.isSwap = true
		}
		return d, false, nil
	}

	if !errors.IsNotFound(err) || up.retry {
		return nil, false, fmt.Errorf("couldn't get deployment %s/%s, please try again; %s", up.Dev.Namespace, up.Dev.Name, err)
	}

	if len(up.Dev.Labels) > 0 {
		if err == errors.ErrNotFound {
			err = errors.UserError{
				E:    fmt.Errorf("Didn't find a deployment in namespace %s that matches the labels in your Okteto manifest", up.Dev.Namespace),
				Hint: "Update your labels or use `okteto namespace` to select a different namespace and try again"}
		}
		return nil, false, err
	}

	_, deploy := os.LookupEnv("OKTETO_AUTODEPLOY")
	deploy = deploy || autoDeploy
	if !deploy {
		deploy, err = askYesNo(fmt.Sprintf("Deployment %s doesn't exist in namespace %s. Do you want to create a new one? [y/n]: ", up.Dev.Name, up.Dev.Namespace))
		if err != nil {
			return nil, false, fmt.Errorf("couldn't read your response")
		}
	}

	if !deploy {
		return nil, false, errors.UserError{
			E:    fmt.Errorf("Deployment %s doesn't exist in namespace %s", up.Dev.Name, up.Dev.Namespace),
			Hint: "Deploy your application first or use `okteto namespace` to select a different namespace and try again",
		}
	}

	return up.Dev.GevSandbox(), true, nil
}

// WaitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExitOrInterrupt() error {
	for {
		select {
		case err := <-up.Running:
			fmt.Println()
			if err != nil {
				log.Infof("Command execution error: %s", err)
				return errors.ErrCommandFailed
			}

			log.Info("Command finished execution without any errors")
			return nil

		case err := <-up.ErrChan:
			log.Yellow(err.Error())

		case <-up.Disconnect:
			return errors.ErrLostConnection
		}
	}
}

func (up *UpContext) devMode(d *appsv1.Deployment, create bool) error {
	spinner := newSpinner("Activating your development environment...")
	up.updateStateFile(activating)
	spinner.start()
	defer spinner.stop()

	if !namespaces.IsOktetoAllowed(up.Namespace) {
		return fmt.Errorf("`okteto up` is not allowed in this namespace")
	}

	if up.Dev.PersistentVolumeEnabled() {
		if err := volumes.Create(up.Context, up.Dev, up.Client); err != nil {
			return err
		}
	}

	devContainer := deployments.GetDevContainer(&d.Spec.Template.Spec, up.Dev.Container)
	if devContainer == nil {
		return fmt.Errorf("Container '%s' does not exist in deployment '%s'", up.Dev.Container, up.Dev.Name)
	}
	up.Dev.Container = devContainer.Name
	if up.Dev.Image == "" {
		up.Dev.Image = devContainer.Image
	}

	if up.retry && !deployments.IsDevModeOn(d) {
		return fmt.Errorf("Development environment has been deactivated")
	}

	up.updateStateFile(starting)

	var err error
	up.Sy, err = syncthing.New(up.Dev)
	if err != nil {
		return err
	}

	if err := up.Sy.Stop(true); err != nil {
		log.Infof("failed to stop existing syncthing: %s", err)
	}

	log.Info("create deployment secrets")
	if err := secrets.Create(up.Dev, up.Client, up.Sy.GUIPasswordHash); err != nil {
		return err
	}

	tr, err := deployments.GetTranslations(up.Dev, d, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.TranslateDevMode(tr, up.Namespace, up.Client); err != nil {
		return err
	}

	for name := range tr {
		if name == d.Name {
			if err := deployments.Deploy(tr[name].Deployment, create, up.Client); err != nil {
				return err
			}
		} else {
			if err := deployments.Deploy(tr[name].Deployment, false, up.Client); err != nil {
				return err
			}
		}
	}
	if create {
		if err := services.CreateDev(up.Dev, up.Client); err != nil {
			return err
		}
	}

	pod, err := pods.GetDevPod(up.Context, up.Dev, up.Client, create)
	if err != nil {
		return err
	}

	reporter := make(chan string)
	defer close(reporter)
	go func() {
		message := "Attaching persistent volume"
		up.updateStateFile(attaching)
		for {
			if up.Dev.PersistentVolumeEnabled() {
				spinner.update(fmt.Sprintf("%s...", message))
			}

			message = <-reporter
			if message == "" {
				return
			}
			if strings.HasPrefix(message, "Pulling") {
				up.updateStateFile(pulling)
			}
		}
	}()

	pod, err = pods.MonitorDevPod(up.Context, up.Dev, pod, up.Client, reporter)
	if err != nil {
		return err
	}

	up.Pod = pod.Name
	go up.cleanCommand()

	up.Forwarder = forward.NewPortForwardManager(up.Context, up.RestConfig, up.Client)
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

	if err := up.Forwarder.Start(up.Pod, up.Dev.Namespace); err != nil {
		return err
	}

	if up.Dev.RemoteModeEnabled() {
		if err := ssh.AddEntry(up.Dev.Name, up.Dev.RemotePort); err != nil {
			return err
		}

		reverseManager := ssh.NewReverseManager(up.Context, up.Dev.RemotePort)
		for _, f := range up.Dev.Reverse {
			if err := reverseManager.Add(&f); err != nil {
				return err
			}
		}

		if err := reverseManager.Start(); err != nil {
			return err
		}
	}

	return nil
}

func (up *UpContext) sync(resetSyncthing bool) error {
	if err := up.startSyncthing(resetSyncthing); err != nil {
		return err
	}

	if err := up.synchronizeFiles(); err != nil {
		return err
	}

	return nil
}

func (up *UpContext) startSyncthing(resetSyncthing bool) error {
	spinner := newSpinner("Starting the file synchronization service...")
	spinner.start()
	up.updateStateFile(startingSync)
	defer spinner.stop()

	if err := up.Sy.Run(up.Context); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(up.Context, true); err != nil {
		return err
	}

	if err := up.Sy.WaitForPing(up.Context, false); err != nil {
		return errors.UserError{
			E:    fmt.Errorf("Failed to connect to the synchronization service"),
			Hint: fmt.Sprintf("Check your development environment logs for errors: 'kubectl logs %s'.\n    If you are using a non-root container, set the securityContext.runAsUser, securityContext.runAsGroup and securityContext.fsGroup fields in your Okteto manifest (https://okteto.com/docs/reference/manifest/index.html#securityContext-object-optional).\n    If you are using secrets, check that your container can write to the destination path of your secrets.\n    Finally, run 'okteto down -v' to reset the synchronization service and try again.", up.Pod),
		}
	}

	if resetSyncthing {
		spinner.update("Resetting synchronization service database...")
		if err := up.Sy.ResetDatabase(up.Context, up.Dev, true); err != nil {
			return err
		}
		if err := up.Sy.ResetDatabase(up.Context, up.Dev, false); err != nil {
			return err
		}
	}

	up.Sy.SendStignoreFile(up.Context, up.Dev)

	if err := up.Sy.WaitForScanning(up.Context, up.Dev, true); err != nil {
		return err
	}
	if err := up.Sy.WaitForScanning(up.Context, up.Dev, false); err != nil {
		return err
	}
	return nil
}

func (up *UpContext) synchronizeFiles() error {
	postfix := "Synchronizing your files..."
	spinner := newSpinner(postfix)
	pbScaling := 0.30

	up.updateStateFile(synchronizing)
	spinner.start()
	defer spinner.stop()
	reporter := make(chan float64)
	go func() {
		<-time.NewTicker(2 * time.Second).C
		var previous float64

		for c := range reporter {
			if c > previous {
				// todo: how to calculate how many characters can the line fit?
				pb := renderProgressBar(postfix, c, pbScaling)
				spinner.update(pb)
				previous = c
			}
		}
	}()

	err := up.Sy.WaitForCompletion(up.Context, up.Dev, reporter)
	if err != nil {
		if err == errors.ErrSyncFrozen {
			analytics.TrackSyncError()
			return errors.UserError{
				E: err,
				Hint: fmt.Sprintf(`Help us improve okteto by filing an issue in https://github.com/okteto/okteto/issues/new.
    Please include your syncthing log (%s) if possible.`, up.Sy.LogPath),
			}
		}

		return err
	}

	// render to 100
	spinner.update(renderProgressBar(postfix, 100, pbScaling))

	up.Sy.Type = "sendreceive"
	up.Sy.IgnoreDelete = false
	if err := up.Sy.UpdateConfig(); err != nil {
		return err
	}

	go up.Sy.Monitor(up.Context, up.Disconnect)
	return up.Sy.Restart(up.Context)
}

func (up *UpContext) cleanCommand() {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	err := exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod,
		up.Dev.Container,
		false,
		in,
		&out,
		os.Stderr,
		[]string{"sh", "-c", "(((cp /var/okteto/bin/* /usr/local/bin); (ps -ef | grep -v -E '/var/okteto/bin/syncthing|/var/okteto/bin/remote|PPID' | awk '{print $2}' | xargs -r kill -9)) >/dev/null 2>&1); cat /proc/sys/fs/inotify/max_user_watches"},
	)

	if err != nil {
		log.Infof("first session to the remote container: %s", err)
	}

	if isWatchesConfigurationTooLow(out.String()) {
		log.Yellow("\nThe value of /proc/sys/fs/inotify/max_user_watches in your cluster nodes is too low.")
		log.Yellow("This can affect Okteto's file synchronization performance.")
		log.Yellow("Visit https://okteto.com/docs/reference/known-issues/index.html for more information.")
	}
	up.cleaned <- struct{}{}
}

func (up *UpContext) runCommand() error {
	log.Infof("starting remote command")
	up.updateStateFile(ready)

	if up.Dev.ExecuteOverSSHEnabled() {
		return ssh.Exec(up.Context, up.Dev.RemotePort, true, os.Stdin, os.Stdout, os.Stderr, up.Dev.Command)
	}

	return exec.Exec(
		up.Context,
		up.Client,
		up.RestConfig,
		up.Dev.Namespace,
		up.Pod,
		up.Dev.Container,
		true,
		os.Stdin,
		os.Stdout,
		os.Stderr,
		up.Dev.Command,
	)
}

func (up *UpContext) getClusterType() string {
	if up.Namespace != nil && namespaces.IsOktetoNamespace(up.Namespace) {
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

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) shutdown() {
	log.Debugf("up shutdown")
	if !up.success {
		analytics.TrackUpError(true, up.isSwap)
	}

	if up.Cancel != nil {
		up.Cancel()
		log.Info("sent cancellation signal")
	}

	if up.Dev.RemoteModeEnabled() {
		if err := ssh.RemoveEntry(up.Dev.Name); err != nil {
			log.Infof("failed to remove ssh entry: %s", err)
		}
	}

	if up.Sy != nil {
		log.Infof("stopping syncthing")
		if err := up.Sy.Stop(false); err != nil {
			log.Infof("failed to stop syncthing during shutdown: %s", err)
		}
	}

	log.Infof("stopping forwarder")
	if up.Forwarder != nil {
		up.Forwarder.Stop()
	}

	log.Info("completed shutdown sequence")
}

func printDisplayContext(message string, dev *model.Dev) {
	log.Success(message)
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
