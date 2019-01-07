package logs

import (
	"io"
	"os"
	"os/signal"
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
func StreamLogs(d *appsv1.Deployment, container string, c *kubernetes.Clientset, config *rest.Config, wg *sync.WaitGroup) {
	defer wg.Done()
	var readCloser io.ReadCloser
	end := false

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	go func() {
		<-channel
		if readCloser != nil {
			readCloser.Close()
		}
		end = true
	}()

	var wait time.Duration
	for !end {
		time.Sleep(wait * time.Second)
		pod, err := deployments.GetCNDPod(d, c)
		if err != nil {
			if wait < 10 {
				wait++
			}
			continue
		}
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
		readCloser, err = req.Stream()
		if err != nil {
			if wait < 10 {
				wait++
			}
			continue
		}
		wait = 0
		if _, err = io.Copy(os.Stdout, readCloser); err != nil {
			log.Errorf("couldn't retrieve logs for %s/%s: %s", pod.Namespace, container, err)
		}
		readCloser = nil
	}
	return
}
