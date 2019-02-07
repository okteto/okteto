package logs

import (
	"context"
	"io"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/cloudnativedevelopment/cnd/pkg/errors"
	"github.com/cloudnativedevelopment/cnd/pkg/log"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
)

//StreamLogs stremas logs from a container
func StreamLogs(ctx context.Context, wg *sync.WaitGroup, pod *apiv1.Pod, container string, c *kubernetes.Clientset, errChan chan error) {
	wg.Add(1)
	defer wg.Done()
	log.Debugf("streaming logs for %s/%s/%s", pod.Namespace, pod.Name, container)

	for {
		if err := streamLogs(ctx, pod, container, c); err != nil {
			if err == context.Canceled {
				return
			}

			if strings.Contains(err.Error(), "not found") {
				log.Debugf("pod is gone, stopping streaming logs")
				errChan <- errors.ErrPodIsGone
				return
			}

			log.Infof("error when streaming logs for %s/%s/%s: %s", pod.Namespace, pod.Name, container, err)

		}

		select {
		case <-ctx.Done():
			log.Debug("stream logs clean shutdown")
			return
		default:
			time.Sleep(5 * time.Second)
		}
	}
}

func streamLogs(ctx context.Context, pod *apiv1.Pod, container string, c *kubernetes.Clientset) error {
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
