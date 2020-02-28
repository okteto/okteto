// Copyright 2020 The Okteto Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package pods

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
	maxRetriesPodRunning         = 300 //1min pod is created
)

var (
	devTerminationGracePeriodSeconds int64
	tailLines                        int64 = 1200
)

// GetBySelector returns the first pod that matches the selector or error if not found
func GetBySelector(namespace string, selector map[string]string, c kubernetes.Interface) (*apiv1.Pod, error) {
	ps, err := ListBySelector(namespace, selector, c)
	if err != nil {
		return nil, err
	}

	if len(ps) == 0 {
		return nil, errors.ErrNotFound
	}

	r := ps[0]
	return &r, nil
}

// ListBySelector returns all the pods that matches the selector or error if not found
func ListBySelector(namespace string, selector map[string]string, c kubernetes.Interface) ([]apiv1.Pod, error) {
	if len(selector) == 0 {
		return nil, fmt.Errorf("empty selector")
	}

	b := new(bytes.Buffer)
	for key, value := range selector {
		fmt.Fprintf(b, "%s=%s,", key, value)
	}

	s := strings.TrimRight(b.String(), ",")

	p, err := c.CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: s,
	})

	if err != nil {
		return nil, err
	}

	return p.Items, nil
}

// GetDevPodInLoop returns the dev pod for a deployment and loops until it success
func GetDevPodInLoop(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	tries := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	for tries < maxRetriesPodRunning {
		pod, err := GetDevPod(ctx, dev, c, waitUntilDeployed)
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

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	d, err := deployments.GetRevisionAnnotatedDeploymentOrFailed(dev, c, waitUntilDeployed)
	if d == nil {
		return nil, err
	}

	log.Infof("deployment %s with revision %v is progressing", d.Name, d.Annotations[deploymentRevisionAnnotation])

	rs, err := replicasets.GetReplicaSetByDeployment(dev, d, c)
	if rs == nil {
		log.Infof("failed to get replicaset with revision %v: %s ", d.Annotations[deploymentRevisionAnnotation], err)
		return nil, err
	}

	log.Infof("replicaset %s with revison %s is progressing", rs.Name, d.Annotations[deploymentRevisionAnnotation])

	return getPodByReplicaSet(dev, rs, c)
}

func getPodByReplicaSet(dev *model.Dev, rs *appsv1.ReplicaSet, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	opts := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", okLabels.InteractiveDevLabel, dev.Name),
	}
	podList, err := c.CoreV1().Pods(dev.Namespace).List(opts)
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		for _, or := range podList.Items[i].OwnerReferences {
			if or.UID == rs.UID {
				return &podList.Items[i], nil
			}
		}
	}
	return nil, nil
}

//MonitorDevPod monitores the state of the pod
func MonitorDevPod(ctx context.Context, dev *model.Dev, pod *apiv1.Pod, c *kubernetes.Clientset, reporter chan string) (*apiv1.Pod, error) {
	opts := metav1.ListOptions{
		FieldSelector: fmt.Sprintf("metadata.name=%s", pod.Name),
	}

	watchPod, err := c.CoreV1().Pods(dev.Namespace).Watch(opts)
	if err != nil {
		return nil, err
	}
	opts = metav1.ListOptions{
		FieldSelector: fmt.Sprintf("involvedObject.kind=Pod,involvedObject.name=%s", pod.Name),
	}
	watchPodEvents, err := c.CoreV1().Events(dev.Namespace).Watch(opts)
	if err != nil {
		return nil, err
	}
	for {
		select {
		case event := <-watchPod.ResultChan():
			pod, ok := event.Object.(*v1.Pod)
			if !ok {
				log.Errorf("type error getting pod event")
				continue
			}
			log.Infof("pod %s updated", pod.Name)
			if pod.Status.Phase == apiv1.PodRunning {
				return pod, nil
			}
			if pod.DeletionTimestamp != nil {
				return nil, fmt.Errorf("development environment has been removed")
			}
		case event := <-watchPodEvents.ResultChan():
			e, ok := event.Object.(*v1.Event)
			if !ok {
				log.Errorf("type error getting pod event")
				continue
			}
			log.Infof("pod %s event: %s", pod.Name, e.Message)
			switch e.Reason {
			case "Failed", "FailedScheduling", "FailedCreatePodSandBox", "ErrImageNeverPull", "InspectFailed", "FailedCreatePodContainer":
				if !strings.HasPrefix(e.Message, "pod has unbound immediate PersistentVolumeClaims") {
					return nil, fmt.Errorf(e.Message)
				}
			case "FailedAttachVolume", "FailedMount":
				reporter <- fmt.Sprintf("%s: retrying", e.Message)
			default:
				if e.Reason == "Pulling" {
					reporter <- strings.Replace(e.Message, "pulling", "Pulling", 1)
				}
			}
		case <-ctx.Done():
			log.Debug("cancelling call to monitor dev pod")
			return nil, ctx.Err()
		}
	}
}

//Exists returns true if pod still exists and is not being deleted
func Exists(podName, namespace string, c kubernetes.Interface) bool {
	pod, err := c.CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.GetObjectMeta().GetDeletionTimestamp() == nil
}

//ContainerLogs returns the logs of a given container in a given pod
func ContainerLogs(container string, pod *apiv1.Pod, namespace string, c kubernetes.Interface) (string, error) {
	podLogOpts := apiv1.PodLogOptions{
		Container:  container,
		Timestamps: true,
		TailLines:  &tailLines,
	}
	req := c.CoreV1().Pods(namespace).GetLogs(pod.Name, &podLogOpts)
	logsStream, err := req.Stream()
	if err != nil {
		return "", err
	}
	defer logsStream.Close()

	buf := new(bytes.Buffer)

	_, err = io.Copy(buf, logsStream)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Restart restarts the pods of a deployment
func Restart(dev *model.Dev, c *kubernetes.Clientset) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", okLabels.DetachedDevLabel, dev.Name),
		},
	)
	if err != nil {
		log.Infof("error listing pods to restart: %s", err)
		return fmt.Errorf("failed to retrieve dev environment information")
	}

	for i := range pods.Items {
		err := c.CoreV1().Pods(dev.Namespace).Delete(pods.Items[i].Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %s", err)
		}
	}

	return waitUntilRunning(dev.Namespace, fmt.Sprintf("%s=%s", okLabels.DetachedDevLabel, dev.Name), c)
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
		for i := range pods.Items {
			switch pods.Items[i].Status.Phase {
			case apiv1.PodPending:
				allRunning = false
				notready[pods.Items[i].GetName()] = true
			case apiv1.PodFailed:
				return fmt.Errorf("Pod %s failed to start", pods.Items[i].Name)
			case apiv1.PodRunning:
				if isRunning(&pods.Items[i]) {
					if _, ok := notready[pods.Items[i].GetName()]; ok {
						log.Infof("pod/%s is ready", pods.Items[i].GetName())
						delete(notready, pods.Items[i].GetName())
					}
				} else {
					allRunning = false
					notready[pods.Items[i].GetName()] = true
					if i%5 == 0 {
						log.Infof("pod/%s is not ready", pods.Items[i].GetName())
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
