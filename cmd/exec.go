package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"
	log "github.com/sirupsen/logrus"

	"github.com/cloudnativedevelopment/cnd/pkg/k8/deployments"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/exec"
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
			analytics.Send(analytics.EventExec, GetActionID())
			defer analytics.Send(analytics.EventExecEnd, GetActionID())
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	namespace, deployment, devContainer, err := findDevEnvironment(true)
	if err != nil {
		return err
	}

	_, client, cfg, err := GetKubernetesClient(namespace)
	if err != nil {
		return err
	}

	d, err := deployments.Get(namespace, deployment, client)
	if err != nil {
		return err
	}

	pod, err := deployments.GetCNDPod(ctx, d, client)
	if err != nil {
		return err
	}

	log.Debugf("running command `%s` on %s", strings.Join(args, " "), pod.Name)
	return exec.Exec(client, cfg, pod, devContainer, true, os.Stdin, os.Stdout, os.Stderr, args)
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
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("unable to parse the cnd local state. Remove '%s' and try again", config.GetCNDHome())
	}
	namespace := parts[0]
	deploymentName := parts[1]
	devContainer := parts[2]

	return namespace, deploymentName, devContainer, nil
}
