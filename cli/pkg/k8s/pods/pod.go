package pods

import (
	"context"
	"fmt"
	"time"

	"github.com/okteto/app/cli/pkg/log"
	"github.com/okteto/app/cli/pkg/model"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	oktetoLabel = "dev.okteto.com"
	maxRetries  = 300
)

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) (string, error) {
	tries := 0
	ticker := time.NewTicker(1 * time.Second)

	for tries < maxRetries {
		pods, err := c.CoreV1().Pods(dev.Namespace).List(
			metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=%s", oktetoLabel, dev.Name),
			},
		)
		if err != nil {
			log.Infof("error listing pods: %s", err)
			return "", fmt.Errorf("failed to retrieve dev environment information")
		}

		var pendingOrRunningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning {
				if pod.GetObjectMeta().GetDeletionTimestamp() == nil {
					pendingOrRunningPods = append(pendingOrRunningPods, pod)
				}
			} else {
				log.Debugf("pod %s/%s is on %s, waiting for it to be running", pod.Namespace, pod.Name, pod.Status.Phase)
			}
		}

		if len(pendingOrRunningPods) == 1 {
			log.Debugf("%s/pod/%s is %s", pendingOrRunningPods[0].Namespace, pendingOrRunningPods[0].Name, pendingOrRunningPods[0].Status.Phase)
			return pendingOrRunningPods[0].Name, nil
		}

		if len(pendingOrRunningPods) > 1 {
			podNames := make([]string, len(pendingOrRunningPods))
			for i, p := range pendingOrRunningPods {
				podNames[i] = p.Name
			}
			return "", fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
		}
		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get cnd pod")
			return "", ctx.Err()
		}
	}

	log.Debugf("cnd pod wasn't running after %d seconds", maxRetries)
	return "", fmt.Errorf("kubernetes is taking too long to create the cloud native environment. Please check for errors and try again")
}
