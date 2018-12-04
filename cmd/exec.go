package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/okteto/cnd/storage"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/exec"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
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
		return fmt.Errorf("There is not a dev mode service in your current folder")
	}
	if len(candidates) > 1 {
		fmt.Printf("warning: there are %d dev mode services in your current folder, taking '%s'\n", len(candidates), deploymentFullName)
	}

	parts := strings.SplitN(deploymentFullName, "/", 2)
	namespace := parts[0]
	deploymentName := parts[1]

	namespace, client, config, err := client.Get()
	if err != nil {
		return err
	}

	pod, err := getCNDPod(client, namespace, deploymentName, devContainer)
	if err != nil {
		return err
	}

	return exec.Exec(client, config, pod, devContainer, os.Stdin, os.Stdout, os.Stderr, args)
}

func containerExists(pod *apiv1.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}

	return false
}

func getCNDPod(c *kubernetes.Clientset, namespace, deploymentName, devContainer string) (*apiv1.Pod, error) {
	tries := 0
	for tries < 30 {

		pods, err := c.CoreV1().Pods(namespace).List(v1.ListOptions{
			LabelSelector: fmt.Sprintf("cnd=%s", deploymentName),
		})

		if err != nil {
			return nil, err
		}

		if len(pods.Items) == 0 {
			return nil, fmt.Errorf("cloud native environment is not initialized. Please run 'cnd up' first")
		}

		pod := pods.Items[0]
		if pod.Status.Phase == apiv1.PodSucceeded || pod.Status.Phase == apiv1.PodFailed {
			return nil, fmt.Errorf("cannot exec in your cloud native environment; current state is %s", pod.Status.Phase)
		}

		var runningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning && pod.GetObjectMeta().GetDeletionTimestamp() == nil {
				runningPods = append(runningPods, pod)
			}
		}

		if len(runningPods) == 1 {
			if devContainer != "" {
				if !containerExists(&pod, devContainer) {
					return nil, fmt.Errorf("container %s doesn't exist in the pod", devContainer)
				}
			}

			return &runningPods[0], nil
		}

		if len(runningPods) > 1 {
			podNames := make([]string, len(runningPods))
			for i, p := range runningPods {
				podNames[i] = p.Name
			}
			return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
		}

		tries++
		time.Sleep(1 * time.Second)
	}

	return nil, fmt.Errorf("kubernetes is taking long to create the dev mode container. Please, check for erros or retry in about 1 minute")
}
