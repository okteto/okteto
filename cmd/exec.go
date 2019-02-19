package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/cloudnativedevelopment/cnd/pkg/analytics"
	"github.com/cloudnativedevelopment/cnd/pkg/config"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	"github.com/cloudnativedevelopment/cnd/pkg/model"
	"github.com/cloudnativedevelopment/cnd/pkg/storage"

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
	var namespace string
	var devPath string

	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			analytics.Send(analytics.EventExec, GetActionID())
			defer analytics.Send(analytics.EventExecEnd, GetActionID())
			return executeExec(namespace, devPath, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	addDevPathFlag(cmd, &devPath, "")
	addNamespaceFlag(cmd, &namespace)
	return cmd
}

func executeExec(searchNamespace, searchDevPath string, args []string) error {
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

		namespace, deployment, devContainer, podName, err = getDevEnvironment(searchNamespace, searchDevPath, true)
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

func findDevEnvironment(mustBeRunning, checkForStale bool) (string, string, string, string, error) {
	services := storage.All()
	candidates := []storage.Service{}
	deploymentFullName := ""
	podName := ""
	folder, _ := os.Getwd()

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			if mustBeRunning {
				if svc.Syncthing == "" {
					continue
				}
			}

			if checkForStale {
				if storage.RemoveIfStale(&svc, name) {
					log.Debugf("found stale entry for %s", name)
					continue
				}
			}

			candidates = append(candidates, svc)
			if deploymentFullName == "" {
				deploymentFullName = name
			}

			if podName == "" {
				podName = svc.Pod
			}
		}
	}

	if len(candidates) == 0 {
		return "", "", "", "", errNoCNDEnvironment
	}

	if len(candidates) > 1 {
		return "", "", "", "", errMultipleCNDEnvironment
	}

	parts := strings.SplitN(deploymentFullName, "/", 3)
	if len(parts) < 3 {
		return "", "", "", "", fmt.Errorf("unable to parse the cnd local state. Remove '%s' and try again", config.GetCNDHome())
	}
	namespace := parts[0]
	deploymentName := parts[1]
	devContainer := parts[2]

	return namespace, deploymentName, devContainer, podName, nil
}

func getDevEnvironment(namespace, devPath string, mustBeRunning bool) (string, string, string, string, error) {
	dev, err := model.ReadDev(devPath)
	if err != nil {
		return "", "", "", "", err
	}

	if namespace == "" {
		namespace, _, _, _, err = k8Client.Get(namespace)
		if err != nil {
			return "", "", "", "", err
		}
	}

	services := storage.All()
	folder, _ := os.Getwd()

	expectedName := fmt.Sprintf("%s/%s/%s", namespace, dev.Swap.Deployment.Name, dev.Swap.Deployment.Container)
	log.Debugf("searching for: %s", expectedName)

	for name, svc := range services {
		if strings.HasPrefix(folder, svc.Folder) {
			if mustBeRunning {
				if svc.Syncthing == "" {
					continue
				}
			}

			if name == expectedName {
				parts := strings.SplitN(name, "/", 3)
				if len(parts) < 3 {
					return "", "", "", "", fmt.Errorf("unable to parse the cnd local state. Remove '%s' and try again", config.GetCNDHome())
				}
				namespace := parts[0]
				deploymentName := parts[1]
				devContainer := parts[2]

				return namespace, deploymentName, devContainer, svc.Pod, nil
			}
		}
	}

	return "", "", "", "", errNoCNDEnvironment
}
