package pods

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
	// OktetoInteractiveDevLabel indicates the interactive dev pod
	OktetoInteractiveDevLabel = "interactive.dev.okteto.com"

	// OktetoDetachedDevLabel indicates the detached dev pods
	OktetoDetachedDevLabel = "detached.dev.okteto.com"

	// OktetoSyncLabel indicates a synthing pod
	OktetoSyncLabel = "syncthing.okteto.com"

	maxRetriesPodRunning = 1500 //5min pod is running
)

var (
	devTerminationGracePeriodSeconds int64
)

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	tries := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	for tries < maxRetriesPodRunning {
		pod, err := loopGetDevPod(dev, c, waitUntilDeployed)
		if err != nil {
			return nil, err
		}
		if pod != nil {
			return pod, nil
		}
		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to get dev pod")
			return nil, ctx.Err()
		}
	}
	return nil, fmt.Errorf("kubernetes is taking too long to create the pod of your development environment. Please check for errors and try again")
}

func loopGetDevPod(dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	d, err := deployments.GetRevionAnnotatedDeploymentOrFailed(dev, c, waitUntilDeployed)
	if d == nil {
		return nil, err
	}
	log.Infof("deployment %s with revision %v is progressing", d.Name, d.Annotations[deploymentRevisionAnnotation])

	rs, err := replicasets.GetReplicaSetByDeployment(dev, d, c)
	if rs == nil {
		return nil, err
	}
	log.Infof("replicaset %s with revison %s is progressing", rs.Name, d.Annotations[deploymentRevisionAnnotation])

	pod, err := getPodByReplicaSet(dev, rs, c)
	if pod == nil {
		return nil, err
	}

	log.Infof("pod %s with revison %s is progressing", pod.Name, d.Annotations[deploymentRevisionAnnotation])
	if pod.Status.Phase == apiv1.PodRunning {
		log.Infof("pod %s with revison %s is running", pod.Name, d.Annotations[deploymentRevisionAnnotation])
		return pod, nil
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			if status.State.Waiting.Reason == "ErrImagePull" || status.State.Waiting.Reason == "ImagePullBackOff" {
				return nil, fmt.Errorf(status.State.Waiting.Message)
			}
		}
	}
	return nil, nil
}

func getPodByReplicaSet(dev *model.Dev, rs *appsv1.ReplicaSet, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", OktetoInteractiveDevLabel, dev.Name),
	}
	podList, err := c.CoreV1().Pods(dev.Namespace).List(opts)
	if err != nil {
		return nil, err
	}
	for _, pod := range podList.Items {
		for _, or := range pod.OwnerReferences {
			if or.UID == rs.UID {
				return &pod, nil
			}
		}
	}
	return nil, nil
}

//Exists returns if the dev pod still exists
func Exists(podName, namespace string, c *kubernetes.Clientset) bool {
	pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.GetObjectMeta().GetDeletionTimestamp() == nil
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
		err := c.CoreV1().Pods(dev.Namespace).Delete(pod.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %s", err)
		}
	}

	return waitUntilRunning(dev.Namespace, fmt.Sprintf("%s=%s", OktetoDetachedDevLabel, dev.Name), c)
}

func waitUntilRunning(namespace, selector string, c *kubernetes.Clientset) error {
	t := time.NewTicker(1 * time.Second)
	notready := map[string]bool{}

	for i := 0; i < 60; i++ {
		if i%5 == 0 {
			log.Infof("checking if pods are ready")
		}

		pods, err := c.CoreV1().Pods(namespace).List(
			metav1.ListOptions{
				LabelSelector: selector,
			},
		)

		if err != nil {
			log.Infof("error listing pods to check status after restart: %s", err)
			return fmt.Errorf("failed to retrieve dev environment information")
		}

		allRunning := true
		for _, pod := range pods.Items {
			switch pod.Status.Phase {
			case apiv1.PodPending:
				allRunning = false
				notready[pod.GetName()] = true
			case apiv1.PodFailed:
				return fmt.Errorf("Pod %s failed to start", pod.Name)
			case apiv1.PodRunning:
				if isRunning(&pod) {
					if _, ok := notready[pod.GetName()]; ok {
						log.Infof("pod/%s is ready", pod.GetName())
						delete(notready, pod.GetName())
					}
				} else {
					allRunning = false
					notready[pod.GetName()] = true
					if i%5 == 0 {
						log.Infof("pod/%s is not ready", pod.GetName())
					}
				}
			}
		}

		if allRunning {
			log.Infof("pods are ready")
			return nil
		}

		<-t.C
	}

	pods := make([]string, 0, len(notready))
	for k := range notready {
		pods = append(pods, k)
	}

	return fmt.Errorf("Pod(s) %s didn't restart after 60 seconds", strings.Join(pods, ","))
}

func isRunning(p *apiv1.Pod) bool {
	if p.Status.Phase != apiv1.PodRunning {
		return false
	}

	if p.GetObjectMeta().GetDeletionTimestamp() != nil {
		return false
	}

	for _, c := range p.Status.Conditions {
		if c.Type == apiv1.PodReady {
			if c.Status == apiv1.ConditionTrue {
				return true
			}
		}
	}

	return false
}
