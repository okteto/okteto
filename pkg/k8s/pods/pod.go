package pods

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	sync "github.com/okteto/okteto/pkg/syncthing/k8s"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// OktetoDevLabel indicates a dev pod
	OktetoDevLabel = "dev.okteto.com"

	// OktetoSyncLabel indicates a synthing pod
	OktetoSyncLabel = "syncthing.okteto.com"

	maxRetries = 300

	failedCreateReason = "FailedCreate"
)

var (
	devTerminationGracePeriodSeconds int64
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

		if tries%10 == 0 && len(pods.Items) == 0 {
			// every 30s check if the deployment failed
			if err := isDeploymentFailed(dev.Namespace, dev.Name, c); err != nil {
				return nil, err
			}
		}

		var runningPods []apiv1.Pod
		for _, pod := range pods.Items {
			if pod.Status.Phase == apiv1.PodRunning {
				if pod.GetObjectMeta().GetDeletionTimestamp() == nil {
					runningPods = append(runningPods, pod)
				}
			} else {
				log.Debugf("pod %s/%s is on %s, waiting for it to be running", pod.Namespace, pod.Name, pod.Status.Phase)
				for _, status := range pod.Status.InitContainerStatuses {
					if status.Name == sync.OktetoInitContainer {
						if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
							return nil, fmt.Errorf("Error initializing okteto volume. This is probably because your development image is not root. Please, add securityContext.runAsUser and securityContext.fsGroup to your deployment manifest")
						}
						if status.State.Waiting != nil {
							if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
								return nil, fmt.Errorf(status.State.Waiting.Message)
							}
						}
					}
				}
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

func isDeploymentFailed(namespace, name string, c *kubernetes.Clientset) error {
	d, err := deployments.Get(name, namespace, c)
	if err != nil {
		log.Infof("failed to get deployment information: %s", err)
		return nil
	}

	log.Debugf("%s/%s conditions", d.Namespace, d.Name)
	for _, c := range d.Status.Conditions {
		log.Debugf("status=%s, type=%s, transition=%s, lastUpdate=%s, reason=%s, message=%s", c.Status, c.Type, c.LastTransitionTime.String(), c.LastUpdateTime.String(), c.Reason, c.Message)
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == failedCreateReason && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
				return errors.ErrQuota
			}
		}
	}

	return nil
}

// Restart restarts the pods of a deployment
func Restart(name, namespace string, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", OktetoDevLabel, name),
		},
	)
	if err != nil {
		log.Infof("error listing pods: %s", err)
		return fmt.Errorf("failed to retrieve dev environment information")
	}

	for _, pod := range pods.Items {
		log.Information("Restarting pod '%s'", pod.Name)
		err := c.CoreV1().Pods(namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("service '%s' was already deleted.", name)
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %s", err)
		}
	}

	return nil
}
