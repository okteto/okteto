package logs

import (
	"io"
	"os"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//Logs stremas logs from a container
func Logs(c *kubernetes.Clientset, config *rest.Config, pod *apiv1.Pod, container string) error {
	var tailLines int64
	tailLines = 100
	req := c.CoreV1().Pods(pod.Namespace).GetLogs(
		pod.Name,
		&apiv1.PodLogOptions{
			Container:  container,
			Timestamps: true,
			Follow:     true,
			TailLines:  &tailLines,
		},
	)
	readCloser, err := req.Stream()
	if err != nil {
		return err
	}
	defer readCloser.Close()
	_, err = io.Copy(os.Stdout, readCloser)
	return err
}
