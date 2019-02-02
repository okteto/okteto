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
)

//Up starts a cloud native environment
func Up() *cobra.Command {
	var namespace string
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debugf("starting up command")
			dev, err := model.ReadDev(devPath)
			if err != nil {
				return err
			}

			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())
			fmt.Println("Activating your cloud native development environment...")

			ctx, cancel := context.WithCancel(context.Background())
			var wg sync.WaitGroup
			defer Shutdown(cancel, &wg)

			disconnectChannel := make(chan struct{}, 1)
			reconnectChannel := make(chan struct{}, 1)
			d, pf, err := ExecuteUp(ctx, &wg, dev, namespace, disconnectChannel, reconnectChannel)
			if err != nil {
				log.Debugf("failed to execute up: %s", err)
				return err
			}

			fullname := deployments.GetFullName(d.Namespace, d.Name)
			fmt.Printf("Linking '%s' to %s/%s...", dev.Mount.Source, fullname, dev.Swap.Deployment.Container)
			fmt.Println()
			log.Green("Ready. Go to your IDE and start coding 😎!")

			log.Debugf("%s ready, waiting for stop signal to shut down", fullname)

			return WaitUntilExit(ctx, reconnectChannel, disconnectChannel, &wg, d, pf)
		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

// WaitUntilExit blocks execution until a stop signal is sent or a disconnect event
func WaitUntilExit(ctx context.Context, reconnect, disconnect chan struct{}, wg *sync.WaitGroup, d *appsv1.Deployment, pf *forward.CNDPortForward) error {
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
		case <-reconnect:
			reconnectionAttempts = 0
			if displayReconnectionNotification {
				log.Green("Reconnected to your cluster.")
				displayDisconnectionNotification = true
				displayReconnectionNotification = false
			}
		case <-disconnect:
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

			if err := ReconnectPortForward(ctx, wg, d, pf); err != nil {
				log.Infof("failed to reconnect port forward. will retry: %s", err)
			}
		case <-resetAttempts.C:
			log.Debug("resetting reconnection attempts counter")
			reconnectionAttempts = 0
		}
	}
}

// ExecuteUp runs all the logic for the up command
func ExecuteUp(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev, namespace string, disconnect, reconnect chan struct{}) (*appsv1.Deployment, *forward.CNDPortForward, error) {

	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return nil, nil, fmt.Errorf("there is already an entry for %s/%s Are you running '%s up' somewhere else?", config.GetBinaryName(), deployments.GetFullName(n, deploymentName), c)
	}

	namespace, client, restConfig, k8sContext, err := GetKubernetesClient(namespace)
	if err != nil {
		return nil, nil, err
	}

	fullname := deployments.GetFullName(namespace, dev.Swap.Deployment.Name)

	log.Debugf("getting deployment %s", fullname)
	d, err := deployments.Get(namespace, dev.Swap.Deployment.Name, client)
	if err != nil {
		log.Debug(err)
		if strings.Contains(err.Error(), "not found") {
			return nil, nil, fmt.Errorf("deployment %s not found [current context: %s]", fullname, k8sContext)
		}

		return nil, nil, fmt.Errorf("couldn't get deployment %s from your cluster. Please try again", fullname)
	}

	dev.Swap.Deployment.Container = deployments.GetDevContainerOrFirst(
		dev.Swap.Deployment.Container,
		d.Spec.Template.Spec.Containers,
	)

	devList, err := deployments.GetAndUpdateDevListFromAnnotation(d.GetObjectMeta(), dev)
	if err != nil {
		return nil, nil, err
	}

	sy, err := syncthing.NewSyncthing(namespace, d.Name, devList)
	if err != nil {
		return nil, nil, err
	}

	log.Debugf("enabling dev mode on %s", fullname)
	if err := deployments.DevModeOn(d, devList, client); err != nil {
		return nil, nil, err
	}

	log.Debugf("enabled dev mode on %s", fullname)

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return nil, nil, err
	}

	go deployments.GetPodEvents(ctx, pod, client)

	if err := deployments.InitVolumeWithTarball(ctx, client, restConfig, namespace, pod.Name, devList); err != nil {
		return nil, nil, err
	}

	if err := sy.Run(ctx, wg); err != nil {
		return nil, nil, err
	}

	err = storage.Insert(ctx, wg, namespace, dev, sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			log.Infof("failed to insert new state value for %s", fullname)
			return nil, nil, fmt.Errorf("there is already an entry for %s. Are you running '%s up' somewhere else?", config.GetBinaryName(), fullname)
		}
		return nil, nil, err
	}

	pf, err := forward.NewCNDPortForward(sy.RemoteAddress)
	if err != nil {
		return nil, nil, err
	}

	if err := pf.Start(ctx, wg, client, restConfig, pod); err != nil {
		return nil, nil, fmt.Errorf("couldn't connect to your cluster: %s", err)
	}

	go logs.StreamLogs(ctx, wg, d, dev.Swap.Deployment.Container, client)

	go sy.Monitor(ctx, disconnect, reconnect)
	return d, pf, nil
}

func ReconnectPortForward(ctx context.Context, wg *sync.WaitGroup, d *appsv1.Deployment, pf *forward.CNDPortForward) error {

	pf.Stop()

	_, client, restConfig, _, err := GetKubernetesClient(d.Namespace)
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return err
	}

	if err := pf.Start(ctx, wg, client, restConfig, pod); err != nil {
		return fmt.Errorf("couldn't connect to your cluster: %s", err)
	}

	log.Infof("reconnected port-forwarder-%d:%d", pf.LocalPort, pf.RemotePort)

	return nil
}

// Shutdown runs the cancellation sequence. It will wait for all tasks to finish for up to 500 milliseconds
func Shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	log.Debugf("cancelling context")
	cancel()

	log.Debugf("waiting for tasks for be done")
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		log.Debugf("completed shutdown sequence")
		return
	case <-time.After(500 * time.Millisecond):
		log.Debugf("tasks didn't finish, terminating")
		return
	}
}
