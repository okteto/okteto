package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/okteto/cnd/pkg/analytics"
	"github.com/okteto/cnd/pkg/config"
	"github.com/okteto/cnd/pkg/model"

	"github.com/okteto/cnd/pkg/k8/deployments"
	"github.com/okteto/cnd/pkg/k8/forward"
	"github.com/okteto/cnd/pkg/k8/logs"
	"github.com/okteto/cnd/pkg/storage"
	"github.com/okteto/cnd/pkg/syncthing"
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
			dev, err := model.ReadDev(devPath)
			if err != nil {
				return err
			}

			analytics.Send(analytics.EventUp, GetActionID())
			defer analytics.Send(analytics.EventUpEnd, GetActionID())
			fmt.Println("Activating your cloud native development environment...")

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var wg sync.WaitGroup
			defer shutdown(cancel, &wg)

			d, err := ExecuteUp(ctx, &wg, dev, namespace)
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
			<-stopChannel
			fmt.Println()
			return nil
		},
	}

	addDevPathFlag(cmd, &devPath)
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "", "kubernetes namespace to use (defaults to the current kube config namespace)")
	return cmd
}

// ExecuteUp runs all the logic for the up command
func ExecuteUp(ctx context.Context, wg *sync.WaitGroup, dev *model.Dev, namespace string) (*appsv1.Deployment, error) {

	n, deploymentName, c, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		return nil, fmt.Errorf("there is already an entry for %s/%s Are you running '%s up' somewhere else?", config.GetBinaryName(), deployments.GetFullName(n, deploymentName), c)
	}

	namespace, client, restConfig, err := GetKubernetesClient(namespace)
	if err != nil {
		return nil, err
	}

	d, err := deployments.Get(namespace, dev.Swap.Deployment.Name, client)
	if err != nil {
		return nil, err
	}
	dev.Swap.Deployment.Container = deployments.GetDevContainerOrFirst(
		dev.Swap.Deployment.Container,
		d.Spec.Template.Spec.Containers,
	)
	devList, err := deployments.GetAndUpdateDevListFromAnnotation(d.GetObjectMeta(), dev)
	if err != nil {
		return nil, err
	}

	if err := deployments.DevModeOn(d, devList, client); err != nil {
		return nil, err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return nil, err
	}

	go deployments.GetPodEvents(ctx, pod, client)

	if err := deployments.InitVolumeWithTarball(ctx, client, restConfig, namespace, pod.Name, devList); err != nil {
		return nil, err
	}

	sy, err := syncthing.NewSyncthing(namespace, d.Name, devList)
	if err != nil {
		return nil, err
	}

	fullname := deployments.GetFullName(namespace, d.Name)

	pf, err := forward.NewCNDPortForward(sy.RemoteAddress)
	if err != nil {
		return nil, err
	}

	wg.Add(1)
	if err := sy.Run(ctx, wg); err != nil {
		return nil, err
	}

	wg.Add(1)
	err = storage.Insert(ctx, wg, namespace, dev, sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			return nil, fmt.Errorf("there is already an entry for %s. Are you running '%s up' somewhere else?", config.GetBinaryName(), fullname)
		}
		return nil, err
	}

	ready := make(chan bool)
	wg.Add(1)
	go pf.Start(ctx, wg, client, restConfig, pod, ready)
	<-ready

	wg.Add(1)
	go logs.StreamLogs(ctx, wg, d, dev.Swap.Deployment.Container, client)

	return d, nil
}

func shutdown(cancel context.CancelFunc, wg *sync.WaitGroup) {
	cancel()
	wg.Wait()
}
