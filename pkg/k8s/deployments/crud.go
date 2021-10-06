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

package deployments

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

//GSandbox returns a base deployment for a dev
func Sandbox(dev *model.Dev) *appsv1.Deployment {
	image := dev.Image.Name
	if image == "" {
		image = model.DefaultImage
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Labels:    model.Labels{},
			Annotations: model.Annotations{
				model.OktetoAutoCreateAnnotation: model.OktetoUpCmd,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32Ptr(1),
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RecreateDeploymentStrategyType,
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": dev.Name,
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": dev.Name,
					},
					Annotations: map[string]string{},
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName:            dev.ServiceAccount,
					TerminationGracePeriodSeconds: pointer.Int64Ptr(0),
					Containers: []apiv1.Container{
						{
							Name:            "dev",
							Image:           image,
							ImagePullPolicy: apiv1.PullAlways,
						},
					},
				},
			},
		},
	}
}

//List returns the list of deployments
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]appsv1.Deployment, error) {
	dList, err := c.AppsV1().Deployments(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return dList.Items, nil
}

//Get returns a deployment object by name
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	return c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

//GetByDev returns a deployment object given a dev struct (by name or by label)
func GetByDev(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	if len(dev.Labels) == 0 {
		return Get(ctx, dev.Name, namespace, c)
	}

	dList, err := c.AppsV1().Deployments(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: dev.LabelsSelector(),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(dList.Items) == 0 {
		return nil, errors.ErrNotFound
	}
	validDeployments := []*appsv1.Deployment{}
	for _, d := range dList.Items {
		if d.Labels[model.DevCloneLabel] == "" {
			validDeployments = append(validDeployments, &d)
		}
	}
	if len(validDeployments) > 1 {
		return nil, fmt.Errorf("found '%d' deployments for labels '%s' instead of 1", len(dList.Items), dev.LabelsSelector())
	}
	return validDeployments[0], nil
}

//CheckConditionErrors checks errors in conditions
func CheckConditionErrors(deployment *appsv1.Deployment, dev *model.Dev) error {
	for _, c := range deployment.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
				log.Infof("%s: %s", errors.ErrQuota, c.Message)
				if strings.Contains(c.Message, "requested: pods=") {
					return fmt.Errorf("quota exceeded, you have reached the maximum number of pods per namespace")
				}
				if strings.Contains(c.Message, "requested: requests.storage=") {
					return fmt.Errorf("quota exceeded, you have reached the maximum storage per namespace")
				}
				return errors.ErrQuota
			} else if isResourcesRelatedError(c.Message) {
				return getResourceLimitError(c.Message, dev)
			}
			return fmt.Errorf(c.Message)
		}
	}
	return nil
}

func isResourcesRelatedError(errorMessage string) bool {
	if strings.Contains(errorMessage, "maximum cpu usage") || strings.Contains(errorMessage, "maximum memory usage") {
		return true
	}
	return false
}

func getResourceLimitError(errorMessage string, dev *model.Dev) error {
	var errorToReturn string
	if strings.Contains(errorMessage, "maximum cpu usage") {
		cpuMaximumRegex, _ := regexp.Compile(`cpu usage per Pod is (\d*\w*)`)
		maximumCpuPerPod := cpuMaximumRegex.FindStringSubmatch(errorMessage)[1]
		var manifestCpu string
		if limitCpu, ok := dev.Resources.Limits[apiv1.ResourceCPU]; ok {
			manifestCpu = limitCpu.String()
		}
		errorToReturn += fmt.Sprintf("The value of resources.limits.cpu in your okteto manifest (%s) exceeds the maximum CPU limit per pod (%s). ", manifestCpu, maximumCpuPerPod)
	}
	if strings.Contains(errorMessage, "maximum memory usage") {
		memoryMaximumRegex, _ := regexp.Compile(`memory usage per Pod is (\d*\w*)`)
		maximumMemoryPerPod := memoryMaximumRegex.FindStringSubmatch(errorMessage)[1]
		var manifestMemory string
		if limitMemory, ok := dev.Resources.Limits[apiv1.ResourceMemory]; ok {
			manifestMemory = limitMemory.String()
		}
		errorToReturn += fmt.Sprintf("The value of resources.limits.memory in your okteto manifest (%s) exceeds the maximum memory limit per pod (%s). ", manifestMemory, maximumMemoryPerPod)
	}
	return fmt.Errorf(strings.TrimSpace(errorToReturn))
}

//Deploy creates or updates a deployment
func Deploy(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) (*appsv1.Deployment, error) {
	d.ResourceVersion = ""
	result, err := c.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	if err == nil {
		return result, nil
	}

	if !errors.IsNotFound(err) {
		return nil, err
	}

	return c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
}

//IsDevModeOn returns if a deployment is in devmode
func IsDevModeOn(d *appsv1.Deployment) bool {
	return labels.Get(d.GetObjectMeta(), model.DevLabel) != ""
}

//Destroy destroys a k8s deployment
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	log.Infof("deleting deployment '%s'", name)
	dClient := c.AppsV1().Deployments(namespace)
	err := dClient.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64Ptr(0)})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	d, err := c.AppsV1().Deployments(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return d.Status.ReadyReplicas > 0
}

func TranslateDivert(username string, d *appsv1.Deployment) *appsv1.Deployment {
	name := model.DivertName(d.Name, username)
	result := d.DeepCopy()
	result.UID = ""
	result.Name = name
	if result.Annotations == nil {
		result.Annotations = map[string]string{}
	}
	result.Annotations[model.OktetoAutoCreateAnnotation] = model.OktetoUpCmd
	result.Labels = map[string]string{model.OktetoDivertLabel: username}
	if d.Labels != nil && d.Labels[model.DeployedByLabel] != "" {
		result.Labels[model.DeployedByLabel] = d.Labels[model.DeployedByLabel]
	}
	result.Spec.Selector = &metav1.LabelSelector{
		MatchLabels: map[string]string{
			model.OktetoDivertLabel: username,
		},
	}
	result.Spec.Template.Labels = map[string]string{
		model.OktetoDivertLabel: username,
	}
	result.ResourceVersion = ""
	return result
}
