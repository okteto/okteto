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

package statefulsets

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	apiv1 "k8s.io/api/core/v1"
	"k8s.io/utils/pointer"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

type patchAnnotations struct {
	Op    string            `json:"op"`
	Path  string            `json:"path"`
	Value map[string]string `json:"value"`
}

// Sandbox returns a default statefulset for a given dev
func Sandbox(dev *model.Dev) *appsv1.StatefulSet {
	image := dev.Image.Name
	if image == "" {
		image = model.DefaultImage
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        dev.Name,
			Namespace:   dev.Namespace,
			Labels:      model.Labels{},
			Annotations: model.Annotations{},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: pointer.Int32Ptr(1),
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
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

// Deploy creates or updates a statefulset
func Deploy(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	sfs.ResourceVersion = ""
	result, err := c.AppsV1().StatefulSets(sfs.Namespace).Update(ctx, sfs, metav1.UpdateOptions{})
	if err == nil {
		return result, nil
	}

	if !oktetoErrors.IsNotFound(err) {
		return nil, err
	}

	return c.AppsV1().StatefulSets(sfs.Namespace).Create(ctx, sfs, metav1.CreateOptions{})
}

// List returns the list of statefulsets
func List(ctx context.Context, namespace, labels string, c kubernetes.Interface) ([]appsv1.StatefulSet, error) {
	sfsList, err := c.AppsV1().StatefulSets(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: labels,
		},
	)
	if err != nil {
		return nil, err
	}
	return sfsList.Items, nil
}

// Get returns a deployment object by name
func Get(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	return c.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
}

// GetByDev returns a statefulset object given a dev struct (by name or by labels)
func GetByDev(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	if len(dev.Selector) == 0 {
		return Get(ctx, dev.Name, namespace, c)
	}

	sfsList, err := c.AppsV1().StatefulSets(namespace).List(
		ctx,
		metav1.ListOptions{
			LabelSelector: dev.LabelsSelector(),
		},
	)
	if err != nil {
		return nil, err
	}
	if len(sfsList.Items) == 0 {
		return nil, oktetoErrors.ErrNotFound
	}
	validStatefulsets := []*appsv1.StatefulSet{}
	for i := range sfsList.Items {
		if sfsList.Items[i].Labels[model.DevCloneLabel] == "" {
			validStatefulsets = append(validStatefulsets, &sfsList.Items[i])
		}
	}
	if len(validStatefulsets) > 1 {
		return nil, fmt.Errorf("found '%d' statefulsets for labels '%s' instead of 1", len(validStatefulsets), dev.LabelsSelector())
	}
	return validStatefulsets[0], nil
}

// Destroy removes a statefulset object given its name and namespace
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	if err := c.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if oktetoErrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes job: %s", err)
	}
	oktetoLog.Infof("statefulset '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	sfs, err := c.AppsV1().StatefulSets(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sfs.Status.ReadyReplicas > 0
}

// IsDevModeOn returns if a statefulset is in devmode
func IsDevModeOn(s *appsv1.StatefulSet) bool {
	labels := s.GetObjectMeta().GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[constants.DevLabel]
	return ok
}

// CheckConditionErrors checks errors in conditions
func CheckConditionErrors(sfs *appsv1.StatefulSet, dev *model.Dev) error {
	for _, c := range sfs.Status.Conditions {
		if c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
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
	if strings.Contains(errorMessage, "maximum cpu usage") {
		cpuMaximumRegex := regexp.MustCompile(`cpu usage per Pod is (\d*\w*)`)
		maximumCpuPerPod := cpuMaximumRegex.FindStringSubmatch(errorMessage)[1]
		var manifestCpu string
		if limitCpu, ok := dev.Resources.Limits[apiv1.ResourceCPU]; ok {
			manifestCpu = limitCpu.String()
		}
		errorToReturn += fmt.Sprintf("The value of resources.limits.cpu in your okteto manifest (%s) exceeds the maximum CPU limit per pod (%s). ", manifestCpu, maximumCpuPerPod)
	}
	if strings.Contains(errorMessage, "maximum memory usage") {
		memoryMaximumRegex := regexp.MustCompile(`memory usage per Pod is (\d*\w*)`)
		maximumMemoryPerPod := memoryMaximumRegex.FindStringSubmatch(errorMessage)[1]
		var manifestMemory string
		if limitMemory, ok := dev.Resources.Limits[apiv1.ResourceMemory]; ok {
			manifestMemory = limitMemory.String()
		}
		errorToReturn += fmt.Sprintf("The value of resources.limits.memory in your okteto manifest (%s) exceeds the maximum memory limit per pod (%s). ", manifestMemory, maximumMemoryPerPod)
	}
	return fmt.Errorf(strings.TrimSpace(errorToReturn))
}

// PatchAnnotations patches the statefulset annotations
func PatchAnnotations(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) error {
	payload := []patchAnnotations{
		{
			Op:    "replace",
			Path:  "/metadata/annotations",
			Value: sfs.Annotations,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err

	}
	if _, err := c.AppsV1().StatefulSets(sfs.Namespace).Patch(ctx, sfs.Name, types.JSONPatchType, payloadBytes, metav1.PatchOptions{}); err != nil {
		return err
	}
	return nil
}
