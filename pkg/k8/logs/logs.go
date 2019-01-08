package logs

import (
	"context"
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

	cancellationCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	channel := make(chan os.Signal, 1)
	signal.Notify(channel, os.Interrupt)
	go func() {
		<-channel
		if readCloser != nil {
			readCloser.Close()
		}
		cancellationCtx.Done()
		end = true
		return
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
		if err := writeLogs(cancellationCtx, os.Stdout, readCloser); err != nil {
			log.Infof("couldn't retrieve logs for %s/%s/%s: %s", pod.Name, d.Name, container, err)
		}

		readCloser = nil
	}
	return
}

type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

func writeLogs(ctx context.Context, out io.Writer, in io.Reader) error {
	// Based on http://ixday.github.io/post/golang-cancel-copy/
	_, err := io.Copy(out, readerFunc(func(p []byte) (int, error) {

		select {

		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			return in.Read(p)
		}
	}))

	return err
}
