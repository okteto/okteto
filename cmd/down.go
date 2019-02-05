package cmd

import (
	"fmt"
	"strings"

	k8Client "github.com/cloudnativedevelopment/cnd/pkg/k8/client"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/deployments"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"
	"github.com/cloudnativedevelopment/cnd/pkg/syncthing"

	"github.com/spf13/cobra"
)

//Down stops a cloud native environment
func Down() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			log.Debug("starting down command")
			return executeDown()
		},
	}

	return cmd
}

func executeDown() error {
	fmt.Println("Deactivating your cloud native development environment...")

	namespace, deployment, container, err := findDevEnvironment(false)

	if err != nil {
		if err == errNoCNDEnvironment {
			log.Debugf("No CND environment running")
			return nil
		}

		log.Info(err)
		return fmt.Errorf("failed to deactivate your cloud native environment")
	}

	_, client, _, k8sContext, err := k8Client.Get(namespace)
	if err != nil {
		return err
	}

	d, err := deployments.Get(namespace, deployment, client)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			fullname := deployments.GetFullName(namespace, deployment)
			return fmt.Errorf("deployment %s not found [current context: %s]", fullname, k8sContext)
		}

		return err
	}
	if err := deployments.DevModeOff(d, client); err != nil {
		return err
	}

	sy, err := syncthing.NewSyncthing(namespace, d.Name, nil)
	if err != nil {
		return err
	}

	dev := &model.Dev{Swap: model.Swap{Deployment: model.Deployment{Name: deployment, Container: container}}}
	if err := storage.Delete(namespace, dev); err != nil {
		return err
	}

	err = sy.Stop()
	if err != nil {
		return err
	}

	err = sy.RemoveFolder()
	if err != nil {
		return err
	}

	fmt.Println("Cloud native development environment deactivated")
	return nil
}
