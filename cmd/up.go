package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

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
			defer shutdown(cancel, &wg)

			disconnectChannel := make(chan struct{}, 1)

			d, pf, err := ExecuteUp(ctx, &wg, dev, namespace, disconnectChannel)
			if err != nil {
				return err
			}

			fullname := deployments.GetFullName(d.Namespace, d.Name)
			fmt.Printf("Linking '%s' to %s/%s...", dev.Mount.Source, fullname, dev.Swap.Deployment.Container)
			fmt.Println()
			fmt.Printf("Ready! Go to your local IDE and continue coding!")
			fmt.Println()

			stopChannel := make(chan os.Signal, 1)
			signal.Notify(stopChannel, os.Interrupt)

			debugChannel := make(chan os.Signal, 1)
			signal.Notify(debugChannel, syscall.SIGUSR2)

			log.Debugf("%s ready, waiting for stop signal to shut down", fullname)
			for {
				select {
				case <-stopChannel:
					log.Debugf("CTRL+C received, starting shutdown sequence")
					fmt.Println()
					return nil
				case <-debugChannel:
					log.Debugf("SIGUSR2 received, reconnecting port forward")
					disconnectChannel <- struct{}{}
				case <-disconnectChannel:
					log.Debug("Cluster connection lost, reconnecting...")
					reconnectPortForward(ctx, &wg, d, pf)
				}
			}
		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

// ExecuteUp runs all the logic for the up command
func ExecuteUp(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev, namespace string, monitor chan struct{}) (*appsv1.Deployment, *forward.CNDPortForward, error) {

	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return nil, nil, fmt.Errorf("there is already an entry for %s/%s Are you running '%s up' somewhere else?", config.GetBinaryName(), deployments.GetFullName(n, deploymentName), c)
	}

	namespace, client, restConfig, err := GetKubernetesClient(namespace)
	if err != nil {
		return nil, nil, err
	}

	d, err := deployments.Get(namespace, dev.Swap.Deployment.Name, client)
	if err != nil {
		return nil, nil, err
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

	if err := deployments.DevModeOn(d, devList, client); err != nil {
		return nil, nil, err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return nil, nil, err
	}

	go deployments.GetPodEvents(ctx, pod, client)

	if err := deployments.InitVolumeWithTarball(ctx, client, restConfig, namespace, pod.Name, devList); err != nil {
		return nil, nil, err
	}

	fullname := deployments.GetFullName(namespace, d.Name)

	pf, err := forward.NewCNDPortForward(sy.RemoteAddress)
	if err != nil {
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

	if err := pf.Start(ctx, wg, client, restConfig, pod); err != nil {
		return nil, nil, fmt.Errorf("couldn't start the connection to your cluster: %s", err)
	}

	wg.Add(1)
	go logs.StreamLogs(ctx, wg, d, dev.Swap.Deployment.Container, client)

	go sy.Monitor(ctx, monitor)
	return d, pf, nil
}

func reconnectPortForward(ctx context.Context, wg *sync.WaitGroup, d *appsv1.Deployment, pf *forward.CNDPortForward) error {

	pf.Stop()

	_, client, restConfig, err := GetKubernetesClient(d.Namespace)
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return err
	}

	if err := pf.Start(ctx, wg, client, restConfig, pod); err != nil {
		return fmt.Errorf("couldn't start the connection to your cluster: %s", err)
	}

	log.Infof("reconnected port-forwarder-%d:%d", pf.LocalPort, pf.RemotePort)

	return nil
}

func shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
}
