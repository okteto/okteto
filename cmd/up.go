package cmd

import (
	"fmt"
	"os"
	"os/signal"

	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/pkg/k8/deployments"
	"github.com/okteto/cnd/pkg/k8/forward"
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

	_, client, restConfig, err := getKubernetesClient(namespace)
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	name, err := deployments.DevModeOn(dev, namespace, client)
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(client, namespace, name, dev.Swap.Deployment.Container)
	if err != nil {
		return err
	}

	if err := deployments.InitVolumeWithTarball(client, restConfig, namespace, pod.Name, dev.Mount.Source); err != nil {
		return err
	}

	sy, err := syncthing.NewSyncthing(name, namespace, dev.Mount.Source)
	if err != nil {
		return err
	}

	fullname := deployments.GetFullName(namespace, name)

	pf, err := forward.NewCNDPortForward(dev.Mount.Source, sy.RemoteAddress, fullname)
	if err != nil {
		return err
	}

	if err := sy.Run(); err != nil {
		return err
	}

	err = storage.Insert(namespace, name, dev.Swap.Deployment.Container, sy.LocalPath, sy.GUIAddress)
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

	return pf.Start(client, restConfig, pod, dev.Swap.Deployment.Container)
}

func stop(sy *syncthing.Syncthing, pf *forward.CNDPortForward) {
	fmt.Println()
	log.Debugf("stopping syncthing and port forwarding")
	if err := sy.Stop(); err != nil {
		log.Error(err)
	}

	storage.Stop(sy.Namespace, sy.Name)
	pf.Stop()
	log.Debugf("stopped syncthing and port forwarding")
}
