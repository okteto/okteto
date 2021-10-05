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
	if len(dev.Selector) == 0 {
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
	if len(dList.Items) > 1 {
		return nil, fmt.Errorf("Found '%d' deployments for labels '%s' instead of 1", len(dList.Items), dev.LabelsSelector())
	}
	d := dList.Items[0]
	return &d, nil
}

//CheckConditionErrors checks errors in conditions
func CheckConditionErrors(deployment *appsv1.Deployment, dev *model.Dev) error {
	for _, c := range deployment.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
				log.Infof("%s: %s", errors.ErrQuota, c.Message)
				if strings.Contains(c.Message, "requested: pods=") {
					return fmt.Errorf("Quota exceeded, you have reached the maximum number of pods per namespace")
				}
				if strings.Contains(c.Message, "requested: requests.storage=") {
					return fmt.Errorf("Quota exceeded, you have reached the maximum storage per namespace")
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

func Create(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) (*appsv1.Deployment, error) {
	return c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
}

func Update(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) (*appsv1.Deployment, error) {
	d.ResourceVersion = ""
	d.Status = appsv1.DeploymentStatus{}
	return c.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
}

//Deploy creates or updates a deployment
func Deploy(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) error {
	old, err := c.AppsV1().Deployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting deployment '%s'': %s", d.Name, err)
	}

	if old.Name == "" {
		log.Infof("creating deployment '%s'", d.Name)
		_, err = c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes deployment: %s", err)
		}
		log.Infof("created deployment '%s'", d.Name)
	} else {
		log.Infof("updating deployment '%s'", d.Name)
		old.Annotations = d.Annotations
		old.Labels = d.Labels
		old.Spec = d.Spec
		if _, err := Update(ctx, old, c); err != nil {
			return fmt.Errorf("error updating kubernetes deployment: %s", err)
		}
		log.Infof("updated deployment '%s'.", d.Name)
	}
	return nil
}

//IsDevModeOn returns if a deployment is in devmode
func IsDevModeOn(d *appsv1.Deployment) bool {
	return labels.Get(d.GetObjectMeta(), model.DevLabel) != ""
}

//RestoreDevModeFrom restores labels an annotations from a deployment in dev mode
func RestoreDevModeFrom(d, old *appsv1.Deployment) {
	d.Labels[model.DevLabel] = old.Labels[model.DevLabel]
	d.Spec.Replicas = old.Spec.Replicas
	d.Annotations = old.Annotations
	d.Spec.Template.Annotations = old.Spec.Template.Annotations
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
