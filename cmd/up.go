package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

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
			up := &UpContext{}
			var err error
			up.Dev, err = model.ReadDev(devPath)
			if err != nil {
				return err
			}

			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())
			fmt.Println("Activating your cloud native development environment...")

			up.Namespace = namespace
			up.WG = &sync.WaitGroup{}
			up.Context, up.Cancel = context.WithCancel(context.Background())
			defer up.Shutdown()

			up.Disconnect = make(chan struct{}, 1)
			up.Reconnect = make(chan struct{}, 1)

			err = up.Execute()
			if err != nil {
				return err
			}

			up.StreamLogsAndEvents()

			fmt.Printf("Linking '%s' to %s/%s...", up.Dev.Mount.Source, up.DeploymentName, up.Dev.Swap.Deployment.Container)
			fmt.Println()
			log.Green("Ready. Go to your IDE and start coding 😎!")

			log.Debugf("%s ready, waiting for stop signal to shut down", up.DeploymentName)

			return up.WaitUntilExit()
		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

// WaitUntilExit blocks execution until a stop signal is sent or a disconnect event
func (up *UpContext) WaitUntilExit() error {
	maxReconnectionAttempts := 6 // this will cause the command to exit after 3 minutes of disconnection
	resetAttempts := time.NewTimer(5 * time.Minute)
	displayDisconnectionNotification := true
	displayReconnectionNotification := false
	reconnectionAttempts := 0
	errLostConnection := fmt.Errorf("Lost connection to your cluster. Plase check your network connection and run '%s up' again", config.GetBinaryName())

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	for {
		select {
		case <-up.Context.Done():
			log.Debug("stopping due to cancellation context")
			fmt.Println()
			return nil
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
				return errLostConnection
			}

			if displayDisconnectionNotification {
				log.Yellow("Trying to reconnect to your cluster. File synchronization will automatically resume when the connection improves.")
				displayReconnectionNotification = true
				displayDisconnectionNotification = false
			}

			if err := up.ReconnectPortForward(); err != nil {
				log.Infof("failed to reconnect port forward. will retry: %s", err)
			}
		case err := <-up.Forwarder.ErrChan:
			log.Yellow(err.Error())
		case <-resetAttempts.C:
			log.Debug("resetting reconnection attempts counter")
			reconnectionAttempts = 0
		}
	}
}

// Execute runs all the logic for the up command
func (up *UpContext) Execute() error {

	if !syncthing.IsInstalled() {
		fmt.Println("Installing dependencies...")
		if err := downloadSyncthing(up.Context); err != nil {
			return fmt.Errorf("couldn't download syncthing, please try again")
		}

	}
	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return fmt.Errorf("there is already an entry for %s/%s Are you running '%s up' somewhere else?", config.GetBinaryName(), deployments.GetFullName(n, deploymentName), c)
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

	up.Forwarder = forward.NewCNDPortForwardManager(up.Context, up.RestConfig, up.Client)
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
	go logs.StreamLogs(up.Context, up.WG, up.Deployment, up.Dev.Swap.Deployment.Container, up.Client)
}

// ReconnectPortForward stops pf and starts it again with the same ports
func (up *UpContext) ReconnectPortForward() error {

	up.Forwarder.Stop()

	pod, err := deployments.GetCNDPod(up.Context, up.Deployment, up.Client)
	if err != nil {
		return err
	}

	up.Forwarder.Start(pod)
	return nil
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) Shutdown() {
	log.Debugf("cancelling context")
	up.Cancel()

	log.Debugf("waiting for tasks for be done")
	done := make(chan struct{})
	go func() {
		up.WG.Wait()
		close(done)
	}()

	go func() {
		up.Forwarder.Stop()
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
