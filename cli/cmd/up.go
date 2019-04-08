package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"sync"
	"time"

	"cli/cnd/pkg/analytics"
	"cli/cnd/pkg/config"
	"cli/cnd/pkg/errors"
	k8Client "cli/cnd/pkg/k8/client"
	"cli/cnd/pkg/log"
	"cli/cnd/pkg/model"
	"cli/cnd/pkg/utils"

	"cli/cnd/pkg/k8/deployments"
	"cli/cnd/pkg/k8/forward"
	"cli/cnd/pkg/storage"
	"cli/cnd/pkg/syncthing"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ReconnectingMessage is the messaged show when we are trying to reconnect
const ReconnectingMessage = "Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves."

// UpContext is the common context of all operations performed during
// the up command
type UpContext struct {
	Context        context.Context
	Cancel         context.CancelFunc
	WG             *sync.WaitGroup
	Dev            *model.Dev
	Client         *kubernetes.Clientset
	RestConfig     *rest.Config
	CurrentContext string
	Namespace      string
	Deployment     *appsv1.Deployment
	DeploymentName string
	Pod            *apiv1.Pod
	Forwarder      *forward.CNDPortForwardManager
	Disconnect     chan struct{}
	Running        chan error
	Exit           chan error
	Sy             *syncthing.Syncthing
	ErrChan        chan error
	progress       *spinner.Spinner
}

//Up starts a cloud native environment
func Up() *cobra.Command {
	var namespace string
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting up command")
			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())

			u := upgradeAvailable()
			if len(u) > 0 {
				log.Yellow("Okteto %s is available, please upgrade.", u)
			}

			if !syncthing.IsInstalled() {
				fmt.Println("Installing dependencies...")
				if err := downloadSyncthing(); err != nil {
					return fmt.Errorf("couldn't download syncthing, please try again")
				}
			}

			devPath = getFullPath(devPath)
			dev, err := model.Get(devPath)
			if err != nil {
				return err
			}
			return RunUp(dev, devPath, namespace)
		},
	}

	addDevPathFlag(cmd, &devPath)
	addNamespaceFlag(cmd, &namespace)
	return cmd
}

//RunUp starts the up sequence
func RunUp(dev *model.Dev, devPath, namespace string) error {
	up := &UpContext{
		WG:         &sync.WaitGroup{},
		Dev:        dev,
		Disconnect: make(chan struct{}, 1),
		Running:    make(chan error, 1),
		Exit:       make(chan error, 1),
		ErrChan:    make(chan error, 1),
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}
	if len(up.Dev.WorkDir.Source) > 0 && up.Dev.WorkDir.Source != wd {
		return fmt.Errorf("The use of 'workdir.source' has been deprecated. Please remove the value from 'okteto.yml' and try again")
	}
	up.Dev.WorkDir.Source = wd

	up.progress = spinner.New(spinner.CharSets[14], 100*time.Millisecond)
	up.progress.Suffix = " Activating your cloud native development environment..."

	up.Namespace = namespace
	defer up.Shutdown()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	go up.Activate(devPath)
	select {
	case <-stop:
		log.Debugf("CTRL+C received, starting shutdown sequence")
		fmt.Println()
	case err = <-up.Exit:
		if err == nil {
			log.Debugf("finished channel received, starting shutdown sequence")
		} else {
			return err
		}
	}
	return nil
}

// Activate activates the dev environment
func (up *UpContext) Activate(devPath string) {
	up.WG.Add(1)
	defer up.WG.Done()
	var prevError error
	for {
		up.Context, up.Cancel = context.WithCancel(context.Background())
		err := up.Execute(prevError != nil, devPath)
		up.progress.Stop()
		if err != nil {
			up.Exit <- err
			return
		}

		disp := up.getDisplayContext()

		switch prevError {
		case nil:
			fmt.Println(disp)
			log.Debugf(up.String())
		case errors.ErrLostConnection:
			log.Green("Reconnected to your cluster.")
		}

		args := []string{"exec", "--file", devPath, "--"}
		args = append(args, up.Dev.Command...)
		args = append(args, up.Dev.Args...)
		cmd := exec.Command(config.GetBinaryFullPath(), args...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Start(); err != nil {
			log.Infof("Failed to execute okteto exec: %s", err)
			up.Exit <- err
			return
		}

		go func() {
			up.WG.Add(1)
			defer up.WG.Done()
			up.Running <- cmd.Wait()
			return
		}()

		prevError = up.WaitUntilExitOrInterrupt(cmd)
		if prevError != nil {
			if prevError == errors.ErrLostConnection {
				log.Yellow("Connection lost to the cloud native development environment, reconnecting...")
				fmt.Println()
			}
			if prevError == errors.ErrCommandFailed {
				log.Yellow("Restarting...")
				fmt.Println()
			}
			up.Shutdown()
			continue
		}

		up.Exit <- nil
		return
	}
}

// WaitUntilExitOrInterrupt blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExitOrInterrupt(cmd *exec.Cmd) error {
	for {
		select {
		case <-up.Context.Done():
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Infof("Failed to kill process: %s", err)
			}
			return nil

		case err := <-up.Running:
			if err != nil {
				log.Infof("Command execution error: %s\n", err)
				return errors.ErrCommandFailed
			}
			return nil

		case err := <-up.ErrChan:
			log.Yellow(err.Error())
		case <-up.Disconnect:
			if err := cmd.Process.Signal(os.Interrupt); err != nil {
				log.Infof("Failed to kill process: %s", err)
			}
			return errors.ErrLostConnection
		}
	}
}

// Execute runs all the logic for the up command
func (up *UpContext) Execute(isRetry bool, devPath string) error {

	var err error
	up.Namespace, up.Client, up.RestConfig, up.CurrentContext, err = k8Client.Get(up.Namespace)
	if err != nil {
		return err
	}

	devEnvironments, err := getDevEnvironments(true, true)
	if err != nil {
		return err
	}

	up.DeploymentName = deployments.GetFullName(up.Namespace, up.Dev.Name)
	log.Debugf("getting deployment %s", up.DeploymentName)

	up.Deployment, err = deployments.Get(up.Namespace, up.Dev.Name, up.Client)
	if err != nil {
		log.Debug(err)
		if strings.Contains(err.Error(), "not found") {
			if !utils.AskYesNo(fmt.Sprintf("Deployment '%s' doesn't exist. Do you want to create a new one? [y/n]: ", up.Dev.Name)) {
				return fmt.Errorf("deployment %s not found [current context: %s]", up.DeploymentName, up.CurrentContext)
			}
		} else {
			return fmt.Errorf("couldn't get deployment %s from your cluster. Please try again", up.DeploymentName)
		}
		up.Deployment = deployments.GetSandBoxManifest(up.Dev, up.Namespace)
		if err := deployments.Create(up.Deployment, up.Client); err != nil {
			return fmt.Errorf("couldn't create deployment %s: %s", up.DeploymentName, err)
		}
	}
	up.progress.Start()

	up.Dev.Container = deployments.GetDevContainerOrFirst(up.Dev.Container, up.Deployment.Spec.Template.Spec.Containers)
	if len(devEnvironments) > 0 {
		for _, d := range devEnvironments {
			if d.Deployment == up.Dev.Name && d.Container == up.Dev.Container && devPath == d.Manifest {
				return fmt.Errorf("there is already an active cloud native development environment on the current folder. Are you running '%s up' somewhere else?", config.GetBinaryName())
			}
		}
	}

	if isRetry {
		// check if is dev deployment, if not, bail out
		enabled, err := deployments.IsDevModeEnabled(up.Deployment.GetObjectMeta())
		if err != nil {
			log.Infof("couldn't determine if the deployment has the dev mode enabled: %s", err)
			return fmt.Errorf("couldn't get deployment %s from your cluster. Please try again", up.DeploymentName)
		}

		if !enabled {
			log.Infof("deployment is no longer in dev mode, shutting down")
			return errors.ErrNotDevDeployment
		}
	}

	devList, err := deployments.GetAndUpdateDevListFromAnnotation(up.Deployment.GetObjectMeta(), up.Dev)
	if err != nil {
		return err
	}

	primary := up.Dev.Container == devList[0].Container
	up.Sy, err = syncthing.NewSyncthing(up.Namespace, up.Deployment.Name, devList, primary)
	if err != nil {
		return err
	}

	log.Debugf("enabling dev mode on %s", up.DeploymentName)
	if err := deployments.DevModeOn(up.Deployment, devList, up.Client); err != nil {
		return err
	}

	log.Debugf("enabled dev mode on %s", up.DeploymentName)

	up.Pod, err = deployments.GetCNDPod(up.Context, up.Deployment.Namespace, up.Deployment.Name, up.Client)
	if err != nil {
		return err
	}

	go deployments.GetPodEvents(up.Context, up.Pod, up.Client)

	if err := up.Sy.Run(up.Context, up.WG); err != nil {
		return err
	}

	err = storage.Insert(up.Context, up.WG, up.Namespace, up.Dev, up.Sy.GUIAddress, up.Pod.Name, devPath)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			log.Infof("failed to insert new state value for %s", up.DeploymentName)
			return fmt.Errorf("there is already an entry for %s/%s. Are you running '%s up' somewhere else?", up.DeploymentName, up.Dev.Container, config.GetBinaryName())
		}
		return err
	}

	up.Forwarder = forward.NewCNDPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	if up.Sy.Primary {
		if err := up.Forwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort); err != nil {
			return err
		}
		if err := up.Forwarder.Add(up.Sy.RemoteGUIPort, syncthing.GUIPort); err != nil {
			return err
		}
	}

	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f.Local, f.Remote); err != nil {
			return err
		}
	}

	up.Forwarder.Start(up.Pod)
	go up.Sy.Monitor(up.Context, up.WG, up.Disconnect)

	if err := up.Sy.WaitForPing(up.Context, up.WG); err != nil {
		return err
	}
	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev); err != nil {
		return err
	}
	if err := up.Sy.OverrideChanges(up.Context, up.WG, up.Dev); err != nil {
		return err
	}
	if err := up.Sy.WaitForCompletion(up.Context, up.WG, up.Dev); err != nil {
		return err
	}

	if !up.Dev.WorkDir.SendOnly {
		up.Sy.ForceSendOnly = false
		if err := up.Sy.UpdateConfig(); err != nil {
			return err
		}
		if err := up.Sy.Restart(up.Context, up.WG); err != nil {
			return err
		}
	}

	return nil
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) Shutdown() {
	log.Debugf("cancelling context")
	if up.Cancel != nil {
		up.Cancel()
	}

	log.Debugf("waiting for tasks for be done")
	done := make(chan struct{})
	go func() {
		if up.WG != nil {
			up.WG.Wait()
		}
		close(done)
	}()

	go func() {
		if up.Forwarder != nil {
			up.Forwarder.Stop()
		}

		return
	}()

	select {
	case <-done:
		log.Debugf("completed shutdown sequence")
		return
	case <-time.After(1 * time.Second):
		log.Debugf("tasks didn't finish, terminating")
		return
	}
}

func (up *UpContext) getDisplayContext() string {
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("%s %s\n", log.SuccessSymbol, log.GreenString("Environment activated!")))

	if len(up.Dev.Forward) > 0 {
		buf.WriteString(log.BlueString("    Ports:\n"))
		for _, f := range up.Dev.Forward {
			buf.WriteString(fmt.Sprintf("       %d -> %d\n", f.Local, f.Remote))
		}
	}

	buf.WriteString(log.BlueString("    Cluster:"))
	buf.WriteString(fmt.Sprintf("     %s\n", up.CurrentContext))
	buf.WriteString(log.BlueString("    Namespace:"))
	buf.WriteString(fmt.Sprintf("   %s\n", up.Deployment.Namespace))
	buf.WriteString(log.BlueString("    Deployment:"))
	buf.WriteString(fmt.Sprintf("  %s\n", up.Deployment.Name))
	return buf.String()
}

func (up *UpContext) String() string {
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("context: %s\n", up.CurrentContext))
	buf.WriteString(fmt.Sprintf("deployment: %s\n", up.DeploymentName))
	buf.WriteString(fmt.Sprintf("container: %s\n", up.Dev.Container))

	buf.WriteString("forward:\n")
	for _, p := range up.Dev.Forward {
		buf.WriteString(fmt.Sprintf("  %d->%d\n", p.Local, p.Remote))
	}

	buf.WriteString("environment:\n")
	for _, e := range up.Dev.Environment {
		buf.WriteString(fmt.Sprintf("  %s=?\n", e.Name))
	}

	return buf.String()
}
