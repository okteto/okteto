package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/log"

	k8Client "github.com/cloudnativedevelopment/cnd/pkg/k8/client"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/deployments"
	"github.com/cloudnativedevelopment/cnd/pkg/k8/exec"
	"github.com/spf13/cobra"
)

var (
	errNoCNDEnvironment       = fmt.Errorf("There aren't any cloud native development environments active in your current folder")
	errMultipleCNDEnvironment = fmt.Errorf("There are multiple cloud native development environments active in your current folder")
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
			return executeExec(devPath, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	addDevPathFlag(cmd, &devPath, "")
	return cmd
}

func executeExec(searchDevPath string, args []string) error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	namespace, deployment, devContainer, podName, err := findDevEnvironment(true, false)
	if err != nil {
		if err != errMultipleCNDEnvironment {
			return err
		}

		if searchDevPath == "" {
			return fmt.Errorf("%s: Please specify which environment to use with the --file flag", err)
		}

		namespace, deployment, devContainer, podName, err = getDevEnvironment(searchDevPath, true)
		if err != nil {
			return err
		}
	}

	_, client, cfg, _, err := k8Client.Get(namespace)
	if err != nil {
		return err
	}

	if podName != "" {
		log.Debugf("running command on %s/pod/%s", namespace, podName)
		err = exec.Exec(ctx, client, cfg, namespace, podName, devContainer, true, os.Stdin, os.Stdout, os.Stderr, args)
		if err == nil {
			return nil
		}

		if !strings.Contains(err.Error(), "not found") {
			return err
		}

		log.Debugf("error running command on %s/pod/%s: %s", namespace, podName, err)
	}

	log.Debugf("retrieving the new pod name for %s/%s and running command", namespace, deployment)
	pod, err := deployments.GetCNDPod(ctx, namespace, deployment, client)
	if err != nil {
		return err
	}

	err = exec.Exec(ctx, client, cfg, namespace, pod.Name, devContainer, true, os.Stdin, os.Stdout, os.Stderr, args)

	return err
}
