package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"sync"

	"github.com/okteto/cnd/pkg/analytics"

	log "github.com/sirupsen/logrus"

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

	_, deploymentName, _, err := findDevEnvironment(true)

	if err != errNoCNDEnvironment {
		log.Info(err)
		return fmt.Errorf("there is already an entry for %s. Are you running 'cnd up' somewhere else?", deployments.GetFullName(namespace, deploymentName))
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

	if err := deployments.DevModeOn(dev, d, client); err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(d, client)
	if err != nil {
		return err
	}

	if err := deployments.InitVolumeWithTarball(client, restConfig, namespace, pod.Name, dev.Mount.Source); err != nil {
		return err
	}

	sy, err := syncthing.NewSyncthing(dev, namespace)
	if err != nil {
		return err
	}

	fullname := deployments.GetFullName(namespace, d.Name)

	pf, err := forward.NewCNDPortForward(dev.Mount.Source, sy.RemoteAddress, fullname)
	if err != nil {
		return err
	}

	if err := sy.Run(); err != nil {
		return err
	}

	err = storage.Insert(namespace, dev, sy.GUIAddress)
	if err != nil {
		if err == storage.ErrAlreadyRunning {
			return fmt.Errorf("there is already an entry for %s. Are you running 'cnd up' somewhere else?", fullname)
		}

		return err
	}

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	go func() {
		<-channel
		stop(sy, pf)
		return
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	ready := make(chan bool)
	go pf.Start(client, restConfig, pod, ready, &wg)
	<-ready

	fmt.Printf("Linking '%s' to %s/%s...", dev.Mount.Source, fullname, dev.Swap.Deployment.Container)
	fmt.Println()
	fmt.Printf("Ready! Go to your local IDE and continue coding!")
	fmt.Println()

	go logs.StreamLogs(d, dev.Swap.Deployment.Container, client, restConfig, &wg)
	wg.Wait()
	return nil
}

func stop(sy *syncthing.Syncthing, pf *forward.CNDPortForward) {
	fmt.Println()
	log.Debugf("stopping syncthing and port forwarding")
	if err := sy.Stop(); err != nil {
		log.Error(err)
	}

	storage.Stop(sy.Namespace, sy.Dev)
	pf.Stop()
	log.Debugf("stopped syncthing and port forwarding")
}
