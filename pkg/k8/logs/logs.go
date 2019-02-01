package logs

import (
	"context"
	"io"
	"os"
	"sync"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/k8/deployments"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

//StreamLogs stremas logs from a container
func StreamLogs(ctx context.Context, wg *sync.WaitGroup, d *appsv1.Deployment, container string, c *kubernetes.Clientset) {
	defer wg.Done()
	for {
		log.Debugf("streaming logs for %s/%s", d.Name, container)
		if err := streamLogs(ctx, d, container, c); err != nil {
			if err != context.Canceled {
				log.Infof("error when streaming logs for %s/%s: %s", d.Name, container, err)
			}
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

func streamLogs(ctx context.Context, d *appsv1.Deployment, container string, c *kubernetes.Clientset) error {
	pod, err := deployments.GetCNDPod(ctx, d, c)
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
