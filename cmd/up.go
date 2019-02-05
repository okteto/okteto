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
	wg             *sync.WaitGroup
	dev            *model.Dev
	client         *kubernetes.Clientset
	restConfig     *rest.Config
	currentContext string
	namespace      string
	deployment     *appsv1.Deployment
	deploymentName string
	pod            *apiv1.Pod
	forwarder      *forward.CNDPortForwardManager
	disconnect     chan struct{}
	reconnect      chan struct{}
	sy             *syncthing.Syncthing
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
			up.dev, err = model.ReadDev(devPath)
			if err != nil {
				return err
			}

			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())
			fmt.Println("Activating your cloud native development environment...")

			up.namespace = namespace
			up.wg = &sync.WaitGroup{}
			up.Context, up.Cancel = context.WithCancel(context.Background())
			defer up.Shutdown()

			up.disconnect = make(chan struct{}, 1)
			up.reconnect = make(chan struct{}, 1)

			err = up.Execute()
			if err != nil {
				return err
			}

			up.StreamLogsAndEvents()

			fmt.Printf("Linking '%s' to %s/%s...", up.dev.Mount.Source, up.deploymentName, up.dev.Swap.Deployment.Container)
			fmt.Println()
			log.Green("Ready. Go to your IDE and start coding 😎!")

			log.Debugf("%s ready, waiting for stop signal to shut down", up.deploymentName)

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
		case <-stop:
			log.Debugf("CTRL+C received, starting shutdown sequence")
			fmt.Println()
			return nil
		case <-up.reconnect:
			reconnectionAttempts = 0
			if displayReconnectionNotification {
				log.Green("Reconnected to your cluster.")
				displayDisconnectionNotification = true
				displayReconnectionNotification = false
			}
		case <-up.disconnect:
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

			if err := ReconnectPortForward(up.Context, up.client, up.deployment, up.forwarder); err != nil {
				log.Infof("failed to reconnect port forward. will retry: %s", err)
			}
		case err := <-up.forwarder.ErrChan:
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

	up.namespace, up.client, up.restConfig, up.currentContext, err = GetKubernetesClient(up.namespace)
	if err != nil {
		return err
	}

	up.deploymentName = deployments.GetFullName(up.namespace, up.dev.Swap.Deployment.Name)
	log.Debugf("getting deployment %s", up.deploymentName)

	up.deployment, err = deployments.Get(up.namespace, up.dev.Swap.Deployment.Name, up.client)
	if err != nil {
		log.Debug(err)
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("deployment %s not found [current context: %s]", up.deploymentName, up.currentContext)
		}

		return fmt.Errorf("couldn't get deployment %s from your cluster. Please try again", up.deploymentName)
	}

	up.dev.Swap.Deployment.Container = deployments.GetDevContainerOrFirst(
		up.dev.Swap.Deployment.Container,
		up.deployment.Spec.Template.Spec.Containers,
	)

	devList, err := deployments.GetAndUpdateDevListFromAnnotation(up.deployment.GetObjectMeta(), up.dev)
	if err != nil {
		return err
	}

	up.sy, err = syncthing.NewSyncthing(up.namespace, up.deployment.Name, devList)
	if err != nil {
		return err
	}

	log.Debugf("enabling dev mode on %s", up.deploymentName)
	if err := deployments.DevModeOn(up.deployment, devList, up.client); err != nil {
		return err
	}

	log.Debugf("enabled dev mode on %s", up.deploymentName)

	up.pod, err = deployments.GetCNDPod(up.Context, up.deployment, up.client)
	if err != nil {
		return err
	}

	if err := deployments.WaitForDevPodToBeRunning(up.Context, up.client, up.namespace, up.pod.Name); err != nil {
		return err
	}

	if err := up.sy.Run(up.Context, up.wg); err != nil {
		return err
	}

	err = storage.Insert(up.Context, up.wg, up.namespace, up.dev, up.sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			log.Infof("failed to insert new state value for %s", up.deploymentName)
			return fmt.Errorf("there is already an entry for %s. Are you running '%s up' somewhere else?", config.GetBinaryName(), up.deploymentName)
		}
		return err
	}

	up.forwarder = forward.NewCNDPortForwardManager(up.Context, up.restConfig, up.client)
	if err := up.forwarder.Add(up.sy.RemotePort, syncthing.ClusterPort, up.sy.IsConnected); err != nil {
		return err
	}

	for _, f := range up.dev.Forward {
		if err := up.forwarder.Add(f.Local, f.Remote, up.sy.IsConnected); err != nil {
			return err
		}
	}

	up.forwarder.Start(up.pod)
	return nil
}

// StreamLogsAndEvents starts go routines to:
// - get pod events
// - get container logs
func (up *UpContext) StreamLogsAndEvents() {
	go deployments.GetPodEvents(up.Context, up.pod, up.client)
	go logs.StreamLogs(up.Context, up.wg, up.deployment, up.dev.Swap.Deployment.Container, up.client)
}

// ReconnectPortForward stops pf and starts it again with the same ports
func ReconnectPortForward(ctx context.Context, c *kubernetes.Clientset, d *appsv1.Deployment, pf *forward.CNDPortForwardManager) error {

	pf.Stop()

	pod, err := deployments.GetCNDPod(ctx, d, c)
	if err != nil {
		return err
	}

	pf.Start(pod)
	return nil
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func (up *UpContext) Shutdown() {
	log.Debugf("cancelling context")
	up.Cancel()

	log.Debugf("waiting for tasks for be done")
	done := make(chan struct{})
	go func() {
		up.wg.Wait()
		close(done)
	}()

	go func() {
		up.forwarder.Stop()
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
