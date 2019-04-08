package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"cli/cnd/pkg/analytics"
	"cli/cnd/pkg/config"
	"cli/cnd/pkg/log"

	k8Client "cli/cnd/pkg/k8/client"
	"cli/cnd/pkg/k8/deployments"
	"cli/cnd/pkg/k8/exec"

	"github.com/spf13/cobra"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string

	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.Send(analytics.EventExec, GetActionID())
			defer analytics.Send(analytics.EventExecEnd, GetActionID())
			err := executeExec(devPath, args)
			return err
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	addDevPathFlag(cmd, &devPath)
	return cmd
}

func executeExec(searchDevPath string, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	devEnvironments, err := getDevEnvironments(true, false)
	if err != nil {
		return err
	}

	if len(devEnvironments) == 0 {
		return errNoCNDEnvironment
	}

	var devEnvironment *devenv
	if len(devEnvironments) == 1 {
		devEnvironment = &devEnvironments[0]
	} else {
		devEnvironments = getDevEnvironmentByManifest(devEnvironments, searchDevPath)
		switch len(devEnvironments) {
		case 0:
			return fmt.Errorf("no active environment available. Please run `%s up` and try again", config.GetBinaryName())
		case 1:
			devEnvironment = &devEnvironments[0]
		default:
			log.Infof("more than one environments were created with %s: %+v", searchDevPath, devEnvironments)
			return fmt.Errorf("malformed configuration. Please run `%s down` and try again", config.GetBinaryName())
		}
	}

	_, client, cfg, _, err := k8Client.Get(devEnvironment.Namespace)
	if err != nil {
		return err
	}

	if devEnvironment.Pod != "" {
		log.Debugf("running command on %s/pod/%s/%s", devEnvironment.Namespace, devEnvironment.Pod, devEnvironment.Container)
		err = exec.Exec(ctx, client, cfg, devEnvironment.Namespace, devEnvironment.Pod, devEnvironment.Container, true, os.Stdin, os.Stdout, os.Stderr, args)
		if err == nil {
			return nil
		}

		if !strings.Contains(err.Error(), "not found") {
			return err
		}

		log.Debugf("error running command on %s/pod/%s/%s: %s", devEnvironment.Namespace, devEnvironment.Pod, devEnvironment.Container, err)
	}

	log.Debugf("retrieving the new pod name for %s/%s and running command", devEnvironment.Namespace, devEnvironment.Deployment)
	pod, err := deployments.GetCNDPod(ctx, devEnvironment.Namespace, devEnvironment.Deployment, client)
	if err != nil {
		return err
	}

	err = exec.Exec(ctx, client, cfg, devEnvironment.Namespace, pod.Name, devEnvironment.Container, true, os.Stdin, os.Stdout, os.Stderr, args)

	return err
}
