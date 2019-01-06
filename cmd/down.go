package cmd

import (
	"fmt"

	"github.com/okteto/cnd/pkg/storage"
	"github.com/okteto/cnd/pkg/syncthing"

	"github.com/okteto/cnd/pkg/k8/deployments"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

//Down stops a cloud native environment
func Down() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Deactivate your cloud native development environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeDown()
		},
	}

	return cmd
}

func executeDown() error {
	fmt.Println("Deactivating your cloud native development environment...")

	namespace, deployment, devContainer, err := findDevEnvironment(false)

	if err != nil {
		if err == errNoCNDEnvironment {
			log.Debugf("No CND environment running")
			return nil
		}

		log.Error(err)
		return fmt.Errorf("failed to deactivate your cloud native environment")
	}

	_, client, _, err := getKubernetesClient(namespace)
	if err != nil {
		return err
	}

	_, err = deployments.DevModeOff(namespace, deployment, client)
	if err != nil {
		return err
	}

	syncthing, err := syncthing.NewSyncthing(deployment, namespace, devContainer, "")
	if err != nil {
		return err
	}

	storage.Delete(namespace, deployment, devContainer)

	err = syncthing.Stop()
	if err != nil {
		return err
	}

	err = syncthing.RemoveFolder()
	if err != nil {
		return err
	}

	fmt.Println("Cloud native development environment deactivated")
	return nil
}
