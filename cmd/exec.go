package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/okteto/cnd/k8/client"
	"github.com/okteto/cnd/k8/exec"
	"github.com/okteto/cnd/model"
	"github.com/spf13/cobra"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Exec executes a command on the CND container
func Exec() *cobra.Command {
	var devPath string
	cmd := &cobra.Command{
		Use:   "exec COMMAND",
		Short: "Execute a command in the cloud native environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			return executeExec(devPath, args)
		},
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return errors.New("exec requires the COMMAND argument")
			}

			return nil
		},
	}

	cmd.Flags().StringVarP(&devPath, "file", "f", "cnd.yml", "manifest file")
	return cmd
}

func executeExec(devPath string, args []string) error {
	namespace, client, config, err := client.Get()
	if err != nil {
		return err
	}

	dev, err := model.ReadDev(devPath)
	if err != nil {
		return err
	}

	pod, err := getCNDPod(client, namespace, dev)
	if err != nil {
		return err
	}

	return exec.Exec(client, config, pod, dev.Swap.Deployment.Container, os.Stdin, os.Stdout, os.Stderr, args)
}

func containerExists(pod *apiv1.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}

	return false
}

func getCNDPod(c *kubernetes.Clientset, namespace string, dev *model.Dev) (*apiv1.Pod, error) {
	pods, err := c.CoreV1().Pods(namespace).List(v1.ListOptions{
		LabelSelector: fmt.Sprintf("cnd=%s", dev.Name),
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

	if len(pods.Items) > 1 {
		podNames := make([]string, len(pods.Items))
		for i, p := range pods.Items {
			podNames[i] = p.Name
		}

		return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)

	}

	if dev.Swap.Deployment.Container != "" {
		if !containerExists(&pod, dev.Swap.Deployment.Container) {
			return nil, fmt.Errorf("container %s doesn't exist in the pod", dev.Swap.Deployment.Container)
		}
	}

	return &pod, nil
}
