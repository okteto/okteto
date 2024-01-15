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

package deployments

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

type patchAnnotations struct {
	Value map[string]string `json:"value"`
	Op    string            `json:"op"`
	Path  string            `json:"path"`
}

// Sandbox returns a base deployment for a dev
func Sandbox(dev *model.Dev) *appsv1.Deployment {
	image := dev.Image.Name
	if image == "" {
		image = model.DefaultImage
	}
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Labels: model.Labels{
				constants.DevLabel: "true",
			},
			Annotations: model.Annotations{
				model.OktetoAutoCreateAnnotation: model.OktetoUpCmd,
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: pointer.Int32(1),
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
					TerminationGracePeriodSeconds: pointer.Int64(0),
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

// List returns the list of deployments
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

// Get returns a deployment object by name
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	return c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetByDev returns a deployment object given a dev struct (by name or by label)
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
		return nil, oktetoErrors.ErrNotFound
	}
	validDeployments := []*appsv1.Deployment{}
	for i := range dList.Items {
		if dList.Items[i].Labels[model.DevCloneLabel] == "" {
			validDeployments = append(validDeployments, &dList.Items[i])
		}
	}
	if len(validDeployments) == 0 {
		return nil, oktetoErrors.ErrNotFound
	}
	if len(validDeployments) > 1 {
		return nil, fmt.Errorf("found '%d' deployments for labels '%s' instead of 1", len(validDeployments), dev.LabelsSelector())
	}
	return validDeployments[0], nil
}

// CheckConditionErrors checks errors in conditions
func CheckConditionErrors(deployment *appsv1.Deployment, dev *model.Dev) error {
	for _, c := range deployment.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
				oktetoLog.Infof("%s: %s", oktetoErrors.ErrQuota, c.Message)
				if strings.Contains(c.Message, "requested: pods=") {
					return fmt.Errorf("quota exceeded, you have reached the maximum number of pods per namespace")
				}
				if strings.Contains(c.Message, "requested: requests.storage=") {
					return fmt.Errorf("quota exceeded, you have reached the maximum storage per namespace")
				}
				return oktetoErrors.ErrQuota
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
	var regexMaxSubstring = 2
	if strings.Contains(errorMessage, "maximum cpu usage") {
		cpuMaximumRegex := regexp.MustCompile(`cpu usage per Pod is (\d*\w*)`)
		maximumCpuPerPodMatchGroups := cpuMaximumRegex.FindStringSubmatch(errorMessage)
		if len(maximumCpuPerPodMatchGroups) < regexMaxSubstring {
			errorToReturn += "The value of resources.limits.cpu in your okteto manifest exceeds the maximum CPU limit per pod. "
		} else {
			var manifestCpu string
			if limitCpu, ok := dev.Resources.Limits[apiv1.ResourceCPU]; ok {
				manifestCpu = limitCpu.String()
			}
			maximumCpuPerPod := maximumCpuPerPodMatchGroups[1]
			errorToReturn += fmt.Sprintf("The value of resources.limits.cpu in your okteto manifest (%s) exceeds the maximum CPU limit per pod (%s). ", manifestCpu, maximumCpuPerPod)
		}
	}
	if strings.Contains(errorMessage, "maximum memory usage") {
		memoryMaximumRegex := regexp.MustCompile(`memory usage per Pod is (\d*\w*)`)
		maximumMemoryPerPodMatchGroups := memoryMaximumRegex.FindStringSubmatch(errorMessage)
		if len(maximumMemoryPerPodMatchGroups) < regexMaxSubstring {
			errorToReturn += "The value of resources.limits.memory in your okteto manifest exceeds the maximum memory limit per pod."
		} else {
			var manifestMemory string
			if limitMemory, ok := dev.Resources.Limits[apiv1.ResourceMemory]; ok {
				manifestMemory = limitMemory.String()
			}
			maximumMemoryPerPod := maximumMemoryPerPodMatchGroups[1]
			errorToReturn += fmt.Sprintf("The value of resources.limits.memory in your okteto manifest (%s) exceeds the maximum memory limit per pod (%s). ", manifestMemory, maximumMemoryPerPod)
		}
	}
	return fmt.Errorf(strings.TrimSpace(errorToReturn))
}

// Deploy creates or updates a deployment
func Deploy(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) (*appsv1.Deployment, error) {
	d.ResourceVersion = ""
	result, err := c.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	if err == nil {
		return result, nil
	}

	if !oktetoErrors.IsNotFound(err) {
		return nil, err
	}

	return c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
}

// IsDevModeOn returns if a deployment is in devmode
func IsDevModeOn(d *appsv1.Deployment) bool {
	return labels.Get(d.GetObjectMeta(), constants.DevLabel) != ""
}

// Destroy destroys a k8s deployment
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	oktetoLog.Infof("deleting deployment '%s'", name)
	dClient := c.AppsV1().Deployments(namespace)
	err := dClient.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: pointer.Int64(0)})
	if err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes deployment: %w", err)
	}
	oktetoLog.Infof("deployment '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	d, err := c.AppsV1().Deployments(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return d.Status.ReadyReplicas > 0
}

// PatchAnnotations patches the deployment annotations
func PatchAnnotations(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) error {
	payload := []patchAnnotations{
		{
			Op:    "replace",
			Path:  "/metadata/annotations",
			Value: d.Annotations,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err

	}
	if _, err := c.AppsV1().Deployments(d.Namespace).Patch(ctx, d.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{}); err != nil {
		return err
	}
	return nil
}
