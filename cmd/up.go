package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/errors"
	k8Client "github.com/cloudnativedevelopment/cnd/pkg/k8/client"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"

	"github.com/cloudnativedevelopment/cnd/pkg/k8/deployments"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/forward"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/logs"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	runtime "k8s.io/apimachinery/pkg/util/runtime"
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
	Reconnect      chan struct{}
	Sy             *syncthing.Syncthing
	ErrChan        chan error
}

//Up starts a cloud native environment
func Up() *cobra.Command {
	var namespace string
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debugf("starting up command")
			up := &UpContext{
				WG:         &sync.WaitGroup{},
				Disconnect: make(chan struct{}, 1),
				Reconnect:  make(chan struct{}, 1),
			}

			var err error
			up.Dev, err = model.ReadDev(devPath)
			if err != nil {
				return err
			}

			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())
			fmt.Println("Activating your cloud native development environment...")

			up.Namespace = namespace
			defer up.Shutdown()

			runtime.ErrorHandlers = []func(error){up.handleRuntimeError}

			isRetry := false
			for {
				up.Context, up.Cancel = context.WithCancel(context.Background())
				up.ErrChan = make(chan error, 1)

				err = up.Execute(isRetry)
				if err != nil {
					return err
				}

				up.StreamLogsAndEvents()

				disp := up.getDisplayContext()

				if !isRetry {
					fmt.Println(disp)
					log.Debugf(up.String())
				} else {
					log.Green("Reconnected to your cluster.")
					fmt.Println()
				}

				err = up.WaitUntilExit()
				close(up.ErrChan)

				if err == errors.ErrPodIsGone {
					log.Yellow("Detected change in the dev environment, reconnecting.")
					up.Shutdown()
					isRetry = true
					continue
				}

				return err
			}

		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

func (up *UpContext) handleRuntimeError(err error) {
	if strings.Contains(err.Error(), "container not running") || strings.Contains(err.Error(), "No such container") {
		up.ErrChan <- errors.ErrPodIsGone
		return
	}

	log.Debugf("unknown unhandled error: %s", err)
}

// WaitUntilExit blocks execution until a stop signal is sent or a disconnect event or an error
func (up *UpContext) WaitUntilExit() error {
	maxReconnectionAttempts := 6 // this will cause the command to exit after 3 minutes of disconnection
	resetAttempts := time.NewTimer(5 * time.Minute)
	displayDisconnectionNotification := true
	displayReconnectionNotification := false
	reconnectionAttempts := 0

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	for {
		select {
		case <-stop:
			log.Debugf("CTRL+C received, starting shutdown sequence")
			fmt.Println()
			return nil
		case <-up.Reconnect:
			reconnectionAttempts = 0
			if displayReconnectionNotification {
				log.Green("Reconnected to your cluster.")
				displayDisconnectionNotification = true
				displayReconnectionNotification = false
			}
		case <-up.Disconnect:
			log.Infof("cluster connection lost, reconnecting %d/%d", reconnectionAttempts, maxReconnectionAttempts)
			reconnectionAttempts++
			if reconnectionAttempts > maxReconnectionAttempts {
				return errors.ErrLostConnection
			}

			if displayDisconnectionNotification {
				log.Yellow(ReconnectingMessage)
				displayReconnectionNotification = true
				displayDisconnectionNotification = false
			}

			up.ErrChan <- errors.ErrPodIsGone

		case err := <-up.ErrChan:
			if err == errors.ErrPodIsGone {
				return err
			}

			log.Yellow(err.Error())

		case <-resetAttempts.C:
			log.Debug("resetting reconnection attempts counter")
			reconnectionAttempts = 0
		}
	}
}

// Execute runs all the logic for the up command
func (up *UpContext) Execute(isRetry bool) error {

	if !syncthing.IsInstalled() {
		fmt.Println("Installing dependencies...")
		if err := downloadSyncthing(up.Context); err != nil {
			return fmt.Errorf("couldn't download syncthing, please try again")
		}

	}
	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return fmt.Errorf("there is already an entry for %s/%s. Are you running '%s up' somewhere else?", deployments.GetFullName(n, deploymentName), c, config.GetBinaryName())
	}

	up.Namespace, up.Client, up.RestConfig, up.CurrentContext, err = k8Client.Get(up.Namespace)
	if err != nil {
		return err
	}

	up.DeploymentName = deployments.GetFullName(up.Namespace, up.Dev.Swap.Deployment.Name)
	log.Debugf("getting deployment %s", up.DeploymentName)

	up.Deployment, err = deployments.Get(up.Namespace, up.Dev.Swap.Deployment.Name, up.Client)
	if err != nil {
		log.Debug(err)
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("deployment %s not found [current context: %s]", up.DeploymentName, up.CurrentContext)
		}

		return fmt.Errorf("couldn't get deployment %s from your cluster. Please try again", up.DeploymentName)
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

	up.Dev.Swap.Deployment.Container = deployments.GetDevContainerOrFirst(
		up.Dev.Swap.Deployment.Container,
		up.Deployment.Spec.Template.Spec.Containers,
	)

	devList, err := deployments.GetAndUpdateDevListFromAnnotation(up.Deployment.GetObjectMeta(), up.Dev)
	if err != nil {
		return err
	}

	up.Sy, err = syncthing.NewSyncthing(up.Namespace, up.Deployment.Name, devList)
	if err != nil {
		return err
	}

	log.Debugf("enabling dev mode on %s", up.DeploymentName)
	if err := deployments.DevModeOn(up.Deployment, devList, up.Client); err != nil {
		return err
	}

	log.Debugf("enabled dev mode on %s", up.DeploymentName)

	up.Pod, err = deployments.GetCNDPod(up.Context, up.Deployment, up.Client)
	if err != nil {
		return err
	}

	if err := deployments.WaitForDevPodToBeRunning(up.Context, up.Client, up.Namespace, up.Pod.Name); err != nil {
		return err
	}

	if err := up.Sy.Run(up.Context, up.WG); err != nil {
		return err
	}

	err = storage.Insert(up.Context, up.WG, up.Namespace, up.Dev, up.Sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			log.Infof("failed to insert new state value for %s", up.DeploymentName)
			return fmt.Errorf("there is already an entry for %s. Are you running '%s up' somewhere else?", config.GetBinaryName(), up.DeploymentName)
		}
		return err
	}

	up.Forwarder = forward.NewCNDPortForwardManager(up.Context, up.RestConfig, up.Client, up.ErrChan)
	if err := up.Forwarder.Add(up.Sy.RemotePort, syncthing.ClusterPort, up.Sy.IsConnected); err != nil {
		return err
	}

	for _, f := range up.Dev.Forward {
		if err := up.Forwarder.Add(f.Local, f.Remote, up.Sy.IsConnected); err != nil {
			return err
		}
	}

	up.Forwarder.Start(up.Pod)
	return nil
}

// StreamLogsAndEvents starts go routines to:
// - get pod events
// - get container logs
func (up *UpContext) StreamLogsAndEvents() {
	go deployments.GetPodEvents(up.Context, up.Pod, up.Client)
	go logs.StreamLogs(up.Context, up.WG, up.Pod, up.Dev.Swap.Deployment.Container, up.Client, up.ErrChan)
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
		up.WG.Wait()
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
	buf.WriteString(log.BlueString("    Deployment:"))
	buf.WriteString(fmt.Sprintf("  %s\n", up.DeploymentName))
	return buf.String()
}

func (up *UpContext) String() string {
	buf := bytes.NewBufferString("")
	buf.WriteString(fmt.Sprintf("context: %s\n", up.CurrentContext))
	buf.WriteString(fmt.Sprintf("deployment: %s\n", up.DeploymentName))
	buf.WriteString(fmt.Sprintf("container: %s\n", up.Dev.Swap.Deployment.Container))

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
