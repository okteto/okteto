package deployments

import (
	"context"
	"fmt"
	"strings"

	"github.com/okteto/app/backend/log"
	"github.com/okteto/app/backend/model"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy creates or updates the dev environment
func Deploy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	d := translate(dev, s)

	if exists(dev, s, c) {
		if err := update(d, c); err != nil {
			return err
		}
	} else {
		if err := create(d, c); err != nil {
			return err
		}
	}
	return nil
}

func exists(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) bool {
	d, err := c.AppsV1().Deployments(s.Name).Get(dev.Name, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return d.Name != ""
}

func create(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Infof("creating deployment '%s' in '%s'...", d.Name, d.Namespace)
	dClient := c.AppsV1().Deployments(d.Namespace)
	_, err := dClient.Create(d)
	if err != nil {
		return fmt.Errorf("error creating kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' created", d.Name)
	return nil
}

func update(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Infof("updating deployment '%s' in '%s' ...", d.Name, d.Namespace)
	dClient := c.AppsV1().Deployments(d.Namespace)
	if _, err := dClient.Update(d); err != nil {
		return fmt.Errorf("error updating kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' updated", d.Name)
	return nil
}

// Destroy destroys a deployment
func Destroy(dev *model.Dev, s *model.Space, c *kubernetes.Clientset) error {
	log.Infof("destroying deployment '%s' in '%s' ...", dev.Name, s.Name)
	dClient := c.AppsV1().Deployments(s.Name)
	if err := dClient.Delete(dev.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds}); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("couldn't destroy deployment: %s", err)
		}
	}
	log.Infof("deployment '%s' destroyed", dev.Name)
	return nil
}

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, s *model.Space, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	pods, err := c.CoreV1().Pods(s.Name).List(metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", oktetoLabel, dev.Name),
	})
	if err != nil {
		return nil, fmt.Errorf("error listing pods: %s", err)
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
		return &pendingOrRunningPods[0], nil
	}

	if len(pendingOrRunningPods) > 1 {
		podNames := make([]string, len(pendingOrRunningPods))
		for i, p := range pendingOrRunningPods {
			podNames[i] = p.Name
		}
		return nil, fmt.Errorf("more than one cloud native environment have the same name: %+v. Please restart your environment", podNames)
	}

	return nil, nil
}
