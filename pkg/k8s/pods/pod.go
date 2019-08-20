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
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"

	// OktetoDetachedDevLabel indicates the detached dev pods
	OktetoDetachedDevLabel = "detached.dev.okteto.com"

	// OktetoSyncLabel indicates a synthing pod
	OktetoSyncLabel = "syncthing.okteto.com"

	maxRetries = 300

	failedCreateReason = "FailedCreate"
)

var (
	devTerminationGracePeriodSeconds int64
)

// GetByLabel returns the dev pod for a deployment
func GetByLabel(ctx context.Context, dev *model.Dev, label string, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
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
			if err := isDeploymentFailed(dev, c); err != nil {
				if !errors.IsNotFound(err) {
					return nil, err
				}

				if !waitUntilDeployed {
					return nil, err
				}
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
					if status.Name == model.OktetoInitContainer {
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

// GetSyncNode returns the sync pod node
func GetSyncNode(dev *model.Dev, c *kubernetes.Clientset) string {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", OktetoSyncLabel, dev.Name),
		},
	)
	if err != nil {
		return ""
	}
	if len(pods.Items) > 0 {
		return pods.Items[0].Spec.NodeName
	}
	return ""
}

//Exists returns if the dev pod still exists
func Exists(podName, namespace string, c *kubernetes.Clientset) bool {
	pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.GetObjectMeta().GetDeletionTimestamp() == nil
}

func isDeploymentFailed(dev *model.Dev, c *kubernetes.Clientset) error {
	d, err := deployments.Get(dev, dev.Namespace, c)
	if err != nil {
		if errors.IsNotFound(err) {
			return err
		}

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
func Restart(dev *model.Dev, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", OktetoDetachedDevLabel, dev.Name),
		},
	)
	if err != nil {
		log.Infof("error listing pods to restart: %s", err)
		return fmt.Errorf("failed to retrieve dev environment information")
	}

	for _, pod := range pods.Items {
		log.Information("Restarting pod '%s'", pod.Name)
		err := c.CoreV1().Pods(dev.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				log.Infof("pod '%s' was already deleted.", pod.Name)
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %s", err)
		}
	}

	return nil
}
