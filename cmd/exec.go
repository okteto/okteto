package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/okteto/cnd/pkg/analytics"
	"github.com/okteto/cnd/pkg/storage"
	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/pkg/k8/deployments"
	"github.com/okteto/cnd/pkg/k8/exec"
	"github.com/spf13/cobra"
)

var (
	errNoCNDEnvironment = fmt.Errorf("There aren't any cloud native development environments active in your current folder")
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.Send(analytics.EventExec, c.actionID)
			defer analytics.Send(analytics.EventExecEnd, c.actionID)
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
	namespace, deployment, devContainer, err := findDevEnvironment(true)
	if err != nil {
		return err
	}

	_, client, config, err := getKubernetesClient(namespace)
	if err != nil {
		return err
	}

	d, err := deployments.Get(namespace, deployment, client)
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(d, client)
	if err != nil {
		return err
	}

	log.Debugf("running command `%s` on %s", strings.Join(args, " "), pod.Name)
	return exec.Exec(client, config, pod, devContainer, true, os.Stdin, os.Stdout, os.Stderr, args)
}

func findDevEnvironment(mustBeRunning bool) (string, string, string, error) {
	services := storage.All()
	candidates := []storage.Service{}
	deploymentFullName := ""
	folder, _ := os.Getwd()

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			if mustBeRunning && svc.Syncthing == "" {
				continue
			}

			candidates = append(candidates, svc)
			if deploymentFullName == "" {
				deploymentFullName = name
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", "", errNoCNDEnvironment
	}

	if len(candidates) > 1 {
		fmt.Printf("warning: there are %d cloud native development environments active in your current folder, using '%s'\n", len(candidates), deploymentFullName)
	}

	parts := strings.SplitN(deploymentFullName, "/", 3)
	namespace := parts[0]
	deploymentName := parts[1]
	devContainer := parts[2]

	return namespace, deploymentName, devContainer, nil
}
