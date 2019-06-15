package pods

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// OktetoDevLabel indicates a dev pod
	OktetoDevLabel = "dev.okteto.com"
	// OktetoSyncLabel indicates a synthing pod
	OktetoSyncLabel = "syncthing.okteto.com"
	maxRetries      = 300
)

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, label string, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	tries := 0
	ticker := time.NewTicker(1 * time.Second)

	for tries < maxRetries {
		pods, err := c.CoreV1().Pods(dev.Namespace).List(
			metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", label, dev.Name),
			},
		)
		if err != nil {
			log.Infof("error listing pods: %s", err)
			return nil, fmt.Errorf("failed to retrieve dev environment information")
		}

		var runningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning {
				if pod.GetObjectMeta().GetDeletionTimestamp() == nil {
					runningPods = append(runningPods, pod)
				}
			} else {
				log.Debugf("pod %s/%s is on %s, waiting for it to be running", pod.Namespace, pod.Name, pod.Status.Phase)
			}
		}

		if len(runningPods) == 1 {
			log.Debugf("%s/pod/%s is %s", runningPods[0].Namespace, runningPods[0].Name, runningPods[0].Status.Phase)
			return &runningPods[0], nil
		}

		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get cnd pod")
			return nil, ctx.Err()
		}
	}

	log.Debugf("dev pod wasn't running after %d seconds", maxRetries)
	return nil, fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
}
