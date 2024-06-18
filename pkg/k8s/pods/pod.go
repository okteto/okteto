// Copyright 2023 The Okteto Authors
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
	"strconv"
	"strings"
	"time"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/events"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

var (
	limitBytes int64 = 5 * 1024 * 1024 // 5Mb
)

// GetBySelector returns the first pod that matches the selector or error if not found
func GetBySelector(ctx context.Context, namespace string, selector map[string]string, c kubernetes.Interface) (*apiv1.Pod, error) {
	ps, err := ListBySelector(ctx, namespace, selector, c)
	if err != nil {
		return nil, err
	}

	if len(ps) == 0 {
		return nil, oktetoErrors.ErrNotFound
	}

	r := ps[0]
	return &r, nil
}

// ListBySelector returns all the pods that matches the selector or error if not found
func ListBySelector(ctx context.Context, namespace string, selector map[string]string, c kubernetes.Interface) ([]apiv1.Pod, error) {
	if len(selector) == 0 {
		return nil, fmt.Errorf("empty selector")
	}

	b := new(bytes.Buffer)
	for key, value := range selector {
		fmt.Fprintf(b, "%s=%s,", key, value)
	}

	s := strings.TrimRight(b.String(), ",")

	p, err := c.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: s,
		},
	)

	if err != nil {
		return nil, err
	}

	return p.Items, nil
}

// GetPodByReplicaSet returns a pod of a given replicaset
func GetPodByReplicaSet(ctx context.Context, rs *appsv1.ReplicaSet, c kubernetes.Interface) (*apiv1.Pod, error) {
	podList, err := c.CoreV1().Pods(rs.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		if podList.Items[i].DeletionTimestamp != nil {
			continue
		}
		if podList.Items[i].Status.Phase == apiv1.PodFailed && podList.Items[i].Status.Reason == "Shutdown" {
			continue
		}
		if podList.Items[i].Status.Phase == apiv1.PodFailed && podList.Items[i].Status.Reason == "Evicted" {
			continue
		}
		for _, or := range podList.Items[i].OwnerReferences {
			if or.UID == rs.UID {
				return &podList.Items[i], nil
			}
		}
	}
	return nil, oktetoErrors.ErrNotFound
}

// GetPodByStatefulSet returns a pod of a given replicaset
func GetPodByStatefulSet(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) (*apiv1.Pod, error) {
	podList, err := c.CoreV1().Pods(sfs.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		if podList.Items[i].DeletionTimestamp != nil {
			continue
		}
		if podList.Items[i].Status.Phase == apiv1.PodFailed && podList.Items[i].Status.Reason == "Shutdown" {
			continue
		}
		if podList.Items[i].Status.Phase == apiv1.PodFailed && podList.Items[i].Status.Reason == "Evicted" {
			continue
		}
		if sfs.Status.UpdateRevision == podList.Items[i].Labels[appsv1.StatefulSetRevisionLabel] {
			for _, or := range podList.Items[i].OwnerReferences {
				if or.UID == sfs.UID {
					return &podList.Items[i], nil
				}
			}
		}
	}
	return nil, oktetoErrors.ErrNotFound
}

// Exists returns true if pod still exists and is not being deleted
func Exists(ctx context.Context, podName, namespace string, c kubernetes.Interface) bool {
	pod, err := c.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.GetObjectMeta().GetDeletionTimestamp() == nil
}

// Destroy destroys a pod by name
func Destroy(ctx context.Context, podName, namespace string, c kubernetes.Interface) error {
	err := c.CoreV1().Pods(namespace).Delete(
		ctx,
		podName,
		metav1.DeleteOptions{
			GracePeriodSeconds: pointer.Int64(0),
		},
	)
	if err != nil && !oktetoErrors.IsNotFound(err) {
		return err
	}
	return nil
}

// GetPodUserID returns the user id running the dev pod
func GetPodUserID(ctx context.Context, podName, containerName, namespace string, c kubernetes.Interface) int64 {
	podLogs, err := ContainerLogs(ctx, containerName, podName, namespace, false, c)
	if err != nil {
		oktetoLog.Infof("failed to access development container logs: %s", err)
		return -1
	}
	return parseUserID(podLogs)
}

func parseUserID(output string) int64 {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		oktetoLog.Info("development container logs not generated. USER cannot be inferred")
		return -1
	}

	if lines[0] == "" {
		oktetoLog.Info("development container logs are empty. USER cannot be inferred")
		return -1
	}

	if !strings.HasPrefix(lines[0], "USER:") {
		oktetoLog.Infof("USER is not the first log line: %s", lines[0])
		return -1
	}

	validUserLineParts := 2
	parts := strings.Split(lines[0], ":")
	if len(parts) != validUserLineParts {
		oktetoLog.Infof("failed to parse USER entry: %s", lines[0])
		return -1
	}

	result, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		oktetoLog.Infof("failed to parse USER entry: %s", lines[0])
		return -1
	}

	return result
}

// ContainerLogs retrieves the logs of a container in a pod
func ContainerLogs(ctx context.Context, containerName, podName, namespace string, timestamps bool, c kubernetes.Interface) (string, error) {
	podLogOpts := apiv1.PodLogOptions{
		Container:  containerName,
		LimitBytes: &limitBytes,
		Timestamps: timestamps,
	}
	req := c.CoreV1().Pods(namespace).GetLogs(podName, &podLogOpts)
	logsStream, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}
	defer func() {
		if err := logsStream.Close(); err != nil {
			oktetoLog.Debugf("Error closing logStream: %s", err)
		}
	}()

	buf := new(bytes.Buffer)

	_, err = io.Copy(buf, logsStream)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

// Restart restarts the pods of a deployment
func Restart(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, sn string) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", model.DetachedDevLabel, dev.Name),
		},
	)
	if err != nil {
		oktetoLog.Infof("error listing pods to restart: %s", err)
		return fmt.Errorf("failed to retrieve development container information")
	}

	found := false
	prefix := fmt.Sprintf("%s-", sn)
	for i := range pods.Items {

		if sn != "" && !strings.HasPrefix(pods.Items[i].Name, prefix) {
			continue
		}
		found = true
		err := c.CoreV1().Pods(dev.Namespace).Delete(ctx, pods.Items[i].Name, metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %w", err)
		}
	}

	if !found {
		return fmt.Errorf("no pods running in development mode")
	}
	return waitUntilRunning(ctx, dev.Namespace, fmt.Sprintf("%s=%s", model.DetachedDevLabel, dev.Name), c)
}

func waitUntilRunning(ctx context.Context, namespace, selector string, c *kubernetes.Clientset) error {
	t := time.NewTicker(1 * time.Second)
	notready := map[string]bool{}

	for i := 0; i < 60; i++ {
		if i%5 == 0 {
			oktetoLog.Infof("checking if pods are ready")
		}

		pods, err := c.CoreV1().Pods(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: selector,
			},
		)

		if err != nil {
			oktetoLog.Infof("error listing pods to check status after restart: %s", err)
			return fmt.Errorf("failed to retrieve development container information")
		}

		allRunning := true
		for i := range pods.Items {
			phase := pods.Items[i].Status.Phase
			if phase == apiv1.PodPending {
				allRunning = false
				notready[pods.Items[i].GetName()] = true
			} else if phase == apiv1.PodFailed {
				return fmt.Errorf("pod %s failed to start", pods.Items[i].Name)
			} else if phase == apiv1.PodRunning {
				if isRunning(&pods.Items[i]) {
					if _, ok := notready[pods.Items[i].GetName()]; ok {
						oktetoLog.Infof("pod/%s is ready", pods.Items[i].GetName())
						delete(notready, pods.Items[i].GetName())
					}
				} else {
					allRunning = false
					notready[pods.Items[i].GetName()] = true
					if i%5 == 0 {
						oktetoLog.Infof("pod/%s is not ready", pods.Items[i].GetName())
					}
				}
			}
		}

		if allRunning {
			oktetoLog.Infof("pods are ready")
			return nil
		}

		<-t.C
	}

	pods := make([]string, 0, len(notready))
	for k := range notready {
		pods = append(pods, k)
	}

	return fmt.Errorf("pod(s) %s didn't restart after 60 seconds", strings.Join(pods, ","))
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

func GetHealthcheckFailure(ctx context.Context, namespace, svcName, stackName string, c kubernetes.Interface) string {
	selector := fmt.Sprintf("%s=%s,%s=%s", model.StackNameLabel, format.ResourceK8sMetaString(stackName), model.StackServiceNameLabel, svcName)
	pods, err := c.CoreV1().Pods(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: selector,
		},
	)
	if err != nil {
		return ""
	}
	for _, pod := range pods.Items {
		for _, containerStatus := range pod.Status.ContainerStatuses {
			if containerStatus.RestartCount > 0 {
				if failureReason := events.GetUnhealthyEventFailure(ctx, namespace, pod.Name, c); failureReason != "" {
					return failureReason
				}
			}
		}

	}
	return ""
}
