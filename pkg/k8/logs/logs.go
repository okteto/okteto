package logs

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/okteto/cnd/pkg/k8/deployments"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//StreamLogs stremas logs from a container
func StreamLogs(
	ctx context.Context, wg *sync.WaitGroup,
	d *appsv1.Deployment, container string, c *kubernetes.Clientset, config *rest.Config,
) {
	defer wg.Done()
	for {
		if err := streamLogs(ctx, d, container, c, config); err != nil {
			log.Infof("couldn't stream logs for %s/%s: %s", d.Name, container, err)
		}
		select {
		case <-ctx.Done():
			log.Debug("stream logs clean shutdown")
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

func streamLogs(ctx context.Context, d *appsv1.Deployment, container string, c *kubernetes.Clientset, config *rest.Config) error {
	pod, err := deployments.GetCNDPod(d, c)
	if err != nil {
		return err
	}
	var tailLines int64
	tailLines = 100
	req := c.CoreV1().Pods(pod.Namespace).GetLogs(
		pod.Name,
		&apiv1.PodLogOptions{
			Container:  container,
			Timestamps: false,
			Follow:     true,
			TailLines:  &tailLines,
		},
	)
	req = req.Context(ctx)
	readCloser, err := req.Stream()
	if err != nil {
		return err
	}

	_, err = io.Copy(os.Stdout, readCloser)
	return err
}
