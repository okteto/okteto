package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/okteto/cnd/pkg/analytics"

	"github.com/okteto/cnd/pkg/k8/deployments"
	"github.com/okteto/cnd/pkg/k8/forward"
	"github.com/okteto/cnd/pkg/k8/logs"
	"github.com/okteto/cnd/pkg/storage"
	"github.com/okteto/cnd/pkg/syncthing"

	"github.com/okteto/cnd/pkg/model"
	"github.com/spf13/cobra"
)

//Up starts a cloud native environment
func Up() *cobra.Command {
	var namespace string
	var devPath string
	cmd := &cobra.Command{
		Use:   "up",
		Short: "Activate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.Send(analytics.EventUp, c.actionID)
			defer analytics.Send(analytics.EventUpEnd, c.actionID)
			return executeUp(devPath, namespace)
		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

func executeUp(devPath, namespace string) error {
	fmt.Println("Activating your cloud native development environment...")
	ctx, cancel := context.WithCancel(context.Background())

	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return fmt.Errorf("there is already an entry for %s/%s Are you running 'cnd up' somewhere else?", deployments.GetFullName(n, deploymentName), c)
	}

	namespace, client, restConfig, err := getKubernetesClient(namespace)
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}
	d, err := deployments.Get(namespace, dev.Swap.Deployment.Name, client)
	if err != nil {
		return err
	}
	dev.Swap.Deployment.Container = deployments.GetDevContainerOrFirst(
		dev.Swap.Deployment.Container,
		d.Spec.Template.Spec.Containers,
	)
	devList, err := deployments.GetAndUpdateDevListFromAnnotation(d.GetObjectMeta(), dev)
	if err != nil {
		return err
	}

	if err := deployments.DevModeOn(d, devList, client); err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup
	defer shutdown(cancel, &wg)

	go deployments.GetPodEvents(ctx, pod, client)

	if err := deployments.InitVolumeWithTarball(ctx, client, restConfig, namespace, pod.Name, devList); err != nil {
		return err
	}

	sy, err := syncthing.NewSyncthing(namespace, d.Name, devList)
	if err != nil {
		return err
	}

	fullname := deployments.GetFullName(namespace, d.Name)

	pf, err := forward.NewCNDPortForward(sy.RemoteAddress)
	if err != nil {
		return err
	}

	wg.Add(1)
	if err := sy.Run(ctx, &wg); err != nil {
		return err
	}

	wg.Add(1)
	err = storage.Insert(ctx, &wg, namespace, dev, sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			return fmt.Errorf("there is already an entry for %s. Are you running 'cnd up' somewhere else?", fullname)
		}
		return err
	}

	ready := make(chan bool)
	wg.Add(1)
	go pf.Start(ctx, &wg, client, restConfig, pod, ready)
	<-ready

	fmt.Printf("Linking '%s' to %s/%s...", dev.Mount.Source, fullname, dev.Swap.Deployment.Container)
	fmt.Println()
	fmt.Printf("Ready! Go to your local IDE and continue coding!")
	fmt.Println()

	wg.Add(1)
	go logs.StreamLogs(ctx, &wg, d, dev.Swap.Deployment.Container, client, restConfig)

	stopChannel := make(chan os.Signal, 1)
	signal.Notify(stopChannel, os.Interrupt)
	<-stopChannel
	fmt.Println()
	return nil
}

func shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
}
