package logs

import (
	"io"
	"os"
	"os/signal"
	"time"

	"github.com/okteto/cnd/pkg/k8/deployments"
	log "github.com/sirupsen/logrus"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

//StreamLogs stremas logs from a container
func StreamLogs(d *appsv1.Deployment, container string, c *kubernetes.Clientset, config *rest.Config) {
	var readCloser io.ReadCloser
	end := false

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	go func() {
		<-channel
		if readCloser != nil {
			readCloser.Close()
			log.Debugf("closed stream reader")
		}
		end = true
		return
	}()

	for !end {
		pod, err := deployments.GetCNDPod(d, c)
		if err != nil {
			time.Sleep(time.Second)
			continue
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

		readCloser, err = req.Stream()
		if err != nil {
			time.Sleep(time.Second)
			continue
		}

		if _, err := io.Copy(os.Stdout, readCloser); err != nil {
			if err.Error() != "http2: response body closed" {
				log.Infof("couldn't retrieve logs for %s/%s/%s: %s", pod.Name, d.Name, container, err)
			}
		}

		readCloser = nil
	}
	return
}
