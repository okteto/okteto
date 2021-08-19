// Copyright 2021 The Okteto Authors
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
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/apps"
	"github.com/okteto/okteto/pkg/k8s/events"
	"github.com/okteto/okteto/pkg/k8s/exec"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
	sfsRevisionLabel             = "controller-revision-hash"
	maxRetriesPodRunning         = 300 //1min pod is created
)

var (
	devTerminationGracePeriodSeconds int64
	limitBytes                       int64 = 5 * 1024 * 1024 // 5Mb
)

// GetBySelector returns the first pod that matches the selector or error if not found
func GetBySelector(ctx context.Context, namespace string, selector map[string]string, c kubernetes.Interface) (*apiv1.Pod, error) {
	ps, err := ListBySelector(ctx, namespace, selector, c)
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

// GetDevPodInLoop returns the dev pod for a deployment and loops until it success
func GetDevPodInLoop(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	start := time.Now()
	to := start.Add(dev.Timeout.Resources)

	for retries := 0; ; retries++ {
		pod, err := GetDevPod(ctx, dev, c, waitUntilDeployed)
		if err != nil {
			return nil, err
		}
		if pod != nil {
			return pod, nil
		}

		if time.Now().After(to) && retries > 10 {
			return nil, fmt.Errorf("kubernetes is taking too long to create your development container. Please check for errors and try again")
		}

		select {
		case <-ticker.C:
			if retries%5 == 0 {
				log.Info("development container is not ready yet, will retry")
			}

			continue
		case <-ctx.Done():
			log.Debug("call to pod.GetDevPodInLoop cancelled")
			return nil, ctx.Err()
		}
	}

}

// GetDevPod returns the dev pod for a deployment
func GetDevPod(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*apiv1.Pod, error) {
	k8sObject, err := apps.GetRevisionAnnotatedK8sObjectOrFailed(ctx, dev, c, waitUntilDeployed)

	if k8sObject == nil {
		return nil, err
	}

	labels := fmt.Sprintf("%s=%s", model.InteractiveDevLabel, dev.Name)

	if k8sObject.ObjectType == model.DeploymentObjectType {
		rs, err := replicasets.GetReplicaSetByDeployment(ctx, k8sObject.Deployment, labels, c)
		if rs == nil {
			if err == nil {
				log.Infof("didn't find replicaset with revision %v", k8sObject.GetAnnotation(deploymentRevisionAnnotation))
			} else {
				log.Infof("failed to get replicaset with revision %v: %s ", k8sObject.GetAnnotation(deploymentRevisionAnnotation), err)
			}
			return nil, err
		}
		return GetPodByReplicaSet(ctx, rs, labels, c)
	} else {
		return GetPodByStatefulSet(ctx, k8sObject.StatefulSet, labels, c)
	}
}

//GetPodByReplicaSet returns a pod of a given replicaset
func GetPodByReplicaSet(ctx context.Context, rs *appsv1.ReplicaSet, labels string, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	podList, err := c.CoreV1().Pods(rs.Namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labels},
	)
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

//GetPodByReplicaSet returns a pod of a given replicaset
func GetPodByStatefulSet(ctx context.Context, sfs *appsv1.StatefulSet, labels string, c *kubernetes.Clientset) (*apiv1.Pod, error) {
	podList, err := c.CoreV1().Pods(sfs.Namespace).List(
		ctx,
		metav1.ListOptions{LabelSelector: labels},
	)
	if err != nil {
		return nil, err
	}
	for i := range podList.Items {
		if podList.Items[i].DeletionTimestamp != nil {
			continue
		}
		if sfs.Status.UpdateRevision == podList.Items[i].Labels[sfsRevisionLabel] {
			for _, or := range podList.Items[i].OwnerReferences {
				if or.UID == sfs.UID {
					return &podList.Items[i], nil
				}
			}
		}
	}
	return nil, nil
}

//GetUserByPod returns the current user of a running pod
func GetUserByPod(ctx context.Context, p *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) (int64, error) {
	cmd := []string{"sh", "-c", "id -u"}
	userIDString, err := execCommandInPod(ctx, p, container, cmd, config, c)
	if err != nil {
		return 0, err
	}
	userID, err := strconv.ParseInt(userIDString, 10, 64)
	if err != nil {
		return 0, err
	}
	return userID, nil
}

//HasPackageJson returns if the container has node_modules
func HasPackageJson(ctx context.Context, p *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) bool {
	cmd := []string{"sh", "-c", "[ -f 'package.json' ] && echo 'package.json exists'"}
	out, err := execCommandInPod(ctx, p, container, cmd, config, c)
	if err != nil {
		return false
	}
	return strings.Contains(out, "package.json exists")
}

//GetWorkdirByPod returns the workdir of a running pod
func GetWorkdirByPod(ctx context.Context, p *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) (string, error) {
	cmd := []string{"sh", "-c", "echo $PWD"}
	return execCommandInPod(ctx, p, container, cmd, config, c)
}

//CheckIfBashIsAvailable returns if bash is available in the given container
func CheckIfBashIsAvailable(ctx context.Context, p *apiv1.Pod, container string, config *rest.Config, c *kubernetes.Clientset) bool {
	cmd := []string{"bash", "--version"}
	_, err := execCommandInPod(ctx, p, container, cmd, config, c)
	return err == nil
}

func execCommandInPod(ctx context.Context, p *apiv1.Pod, container string, cmd []string, config *rest.Config, c *kubernetes.Clientset) (string, error) {
	in := strings.NewReader("\n")
	var out bytes.Buffer

	err := exec.Exec(
		ctx,
		c,
		config,
		p.Namespace,
		p.Name,
		container,
		false,
		in,
		&out,
		os.Stderr,
		cmd,
	)

	if err != nil {
		log.Infof("failed to execute command: %s - %s", err, out.String())
		return "", err
	}
	result := strings.TrimSuffix(out.String(), "\n")
	return result, nil
}

//Exists returns true if pod still exists and is not being deleted
func Exists(ctx context.Context, podName, namespace string, c kubernetes.Interface) bool {
	pod, err := c.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return pod.GetObjectMeta().GetDeletionTimestamp() == nil
}

//Destroy destroys a pod by name
func Destroy(ctx context.Context, podName, namespace string, c kubernetes.Interface) error {
	err := c.CoreV1().Pods(namespace).Delete(
		ctx,
		podName,
		metav1.DeleteOptions{
			GracePeriodSeconds: &devTerminationGracePeriodSeconds,
		},
	)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	return nil
}

//GetDevPodUserID returns the user id running the dev pod
func GetDevPodUserID(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) int64 {
	devPodLogs, err := GetDevPodLogs(ctx, dev, false, c)
	if err != nil {
		log.Infof("failed to access development container logs: %s", err)
		return -1
	}
	return parseUserID(devPodLogs)
}

func parseUserID(output string) int64 {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		log.Info("development container logs not generated. USER cannot be inferred")
		return -1
	}

	if lines[0] == "" {
		log.Info("development container logs are empty. USER cannot be inferred")
		return -1
	}

	if !strings.HasPrefix(lines[0], "USER:") {
		log.Infof("USER is not the first log line: %s", lines[0])
		return -1
	}

	parts := strings.Split(lines[0], ":")
	if len(parts) != 2 {
		log.Infof("failed to parse USER entry: %s", lines[0])
		return -1
	}

	result, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		log.Infof("failed to parse USER entry: %s", lines[0])
		return -1
	}

	return result
}

//GetDevPodLogs returns the logs of the dev pod
func GetDevPodLogs(ctx context.Context, dev *model.Dev, timestamps bool, c *kubernetes.Clientset) (string, error) {
	p, err := GetDevPod(ctx, dev, c, false)
	if err != nil {
		return "", err
	}
	if p == nil {
		return "", errors.ErrNotFound
	}
	if dev.Container == "" {
		dev.Container = p.Spec.Containers[0].Name
	}
	return containerLogs(ctx, dev.Container, p, dev.Namespace, timestamps, c)
}

func containerLogs(ctx context.Context, container string, pod *apiv1.Pod, namespace string, timestamps bool, c kubernetes.Interface) (string, error) {
	podLogOpts := apiv1.PodLogOptions{
		Container:  container,
		LimitBytes: &limitBytes,
		Timestamps: timestamps,
	}
	req := c.CoreV1().Pods(namespace).GetLogs(pod.Name, &podLogOpts)
	logsStream, err := req.Stream(ctx)
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
func Restart(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, sn string) error {
	pods, err := c.CoreV1().Pods(dev.Namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: fmt.Sprintf("%s=%s", model.DetachedDevLabel, dev.Name),
		},
	)
	if err != nil {
		log.Infof("error listing pods to restart: %s", err)
		return fmt.Errorf("failed to retrieve development container information")
	}

	found := false
	prefix := fmt.Sprintf("%s-", sn)
	for i := range pods.Items {

		if sn != "" && !strings.HasPrefix(pods.Items[i].Name, prefix) {
			continue
		}
		found = true
		err := c.CoreV1().Pods(dev.Namespace).Delete(ctx, pods.Items[i].Name, metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return nil
			}
			return fmt.Errorf("error deleting kubernetes service: %s", err)
		}
	}

	if !found {
		return fmt.Errorf("Unable to find any service with the provided name")
	}
	return waitUntilRunning(ctx, dev.Namespace, fmt.Sprintf("%s=%s", model.DetachedDevLabel, dev.Name), c)
}

func waitUntilRunning(ctx context.Context, namespace, selector string, c *kubernetes.Clientset) error {
	t := time.NewTicker(1 * time.Second)
	notready := map[string]bool{}

	for i := 0; i < 60; i++ {
		if i%5 == 0 {
			log.Infof("checking if pods are ready")
		}

		pods, err := c.CoreV1().Pods(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: selector,
			},
		)

		if err != nil {
			log.Infof("error listing pods to check status after restart: %s", err)
			return fmt.Errorf("failed to retrieve development container information")
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

func GetHealthcheckFailure(ctx context.Context, namespace, svcName, stackName string, c kubernetes.Interface) string {
	selector := fmt.Sprintf("%s=%s,%s=%s", model.StackNameLabel, stackName, model.StackServiceNameLabel, svcName)
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
