package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/cnd/pkg/storage"
	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/pkg/k8/deployments"
	"github.com/okteto/cnd/pkg/k8/exec"
	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeExec(args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	return cmd
}

func executeExec(args []string) error {
	namespace, deployment, devContainer, err := findDevEnvironment()
	if err != nil {
		return err
	}

	_, client, config, err := getKubernetesClient()
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(client, namespace, deployment, devContainer)
	if err != nil {
		return err
	}

	log.Debugf("running command `%s` on %s", strings.Join(args, " "), pod.Name)
	return exec.Exec(client, config, pod, devContainer, true, os.Stdin, os.Stdout, os.Stderr, args)
}

func findDevEnvironment() (string, string, string, error) {
	services := storage.All()
	candidates := []storage.Service{}
	deploymentFullName := ""
	devContainer := ""
	folder, _ := os.Getwd()

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			candidates = append(candidates, svc)
			if deploymentFullName == "" {
				deploymentFullName = name
				devContainer = svc.Container
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", "", fmt.Errorf("There aren't any cloud native development environments active in your current folder")
	}
	if len(candidates) > 1 {
		fmt.Printf("warning: there are %d cloud native development environments active in your current folder, using '%s'\n", len(candidates), deploymentFullName)
	}

	parts := strings.SplitN(deploymentFullName, "/", 2)
	namespace := parts[0]
	deploymentName := parts[1]

	return namespace, deploymentName, devContainer, nil
}
