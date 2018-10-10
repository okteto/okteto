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

	pod, err := validateSwapConfiguration(client, namespace, dev)
	if err != nil {
		return err
	}

	return exec.Exec(client, config, pod, dev.Swap.Deployment.Container, os.Stdin, os.Stdout, os.Stderr, args)
}

func getPod(c *kubernetes.Clientset, namespace, name string) (*apiv1.Pod, error) {

	pods, err := c.CoreV1().Pods(namespace).List(v1.ListOptions{
		LabelSelector: fmt.Sprintf("cnd=%s", name),
	})

	if err != nil {
		return nil, err
	}

	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("cloud native environment is not initialized. Please run 'cnd up' first")
	}

	if len(pods.Items) > 1 {
		return nil, fmt.Errorf("more than one cloud native environment have the same name. Please restart your environment")
	}

	return &pods.Items[0], nil
}

func containerExists(pod *apiv1.Pod, container string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == container {
			return true
		}
	}

	return false
}

func validateSwapConfiguration(c *kubernetes.Clientset, namespace string, dev *model.Dev) (*apiv1.Pod, error) {
	p, err := getPod(c, namespace, dev.Name)
	if err != nil {
		return nil, err
	}

	if !containerExists(p, dev.Swap.Deployment.Container) {
		return nil, fmt.Errorf("container %s doesn't exist in the pod", dev.Swap.Deployment.Container)
	}

	return p, nil
}
