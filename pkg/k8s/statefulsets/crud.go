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

package statefulsets

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	apiv1 "k8s.io/api/core/v1"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Deploy creates or updates a statefulset
func Deploy(ctx context.Context, s *appsv1.StatefulSet, c kubernetes.Interface) error {
	old, err := c.AppsV1().StatefulSets(s.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("error getting statefulset '%s'': %s", s.Name, err)
	}

	if old.Name == "" {
		log.Infof("creating statefulset '%s'", s.Name)
		_, err = c.AppsV1().StatefulSets(s.Namespace).Create(ctx, s, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("error creating kubernetes statefulset: %s", err)
		}
		log.Infof("created statefulset '%s'", s.Name)
	} else {
		log.Infof("updating statefulset '%s'", s.Name)
		old.Annotations = s.Annotations
		old.Labels = s.Labels
		old.Spec = s.Spec
		if err := Update(ctx, old, c); err != nil {
			return fmt.Errorf("error updating kubernetes statefulset: %s", err)
		}
		log.Infof("updated statefulset '%s'.", s.Name)
	}
	return nil
}

//List returns the list of statefulsets
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

//GetDeploymentByName returns a deployment object if is created
func GetStatefulsetByName(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	sfs, err := c.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get statefulset %s/%s: %w", namespace, name, err)
	}

	return sfs, nil
}

//Get returns a statefulset object given its name and namespace
func Get(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*appsv1.StatefulSet, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	var s *appsv1.StatefulSet
	var err error

	if len(dev.Labels) == 0 {
		s, err = GetStatefulsetByName(ctx, dev.Name, namespace, c)
		if err != nil {
			return nil, err
		}
	} else {
		statefulsets, err := c.AppsV1().StatefulSets(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: dev.LabelsSelector(),
			},
		)
		if err != nil {
			return nil, err
		}
		if len(statefulsets.Items) == 0 {
			return nil, fmt.Errorf("StatefulSet for labels '%s' not found", dev.LabelsSelector())
		}
		if len(statefulsets.Items) > 1 {
			return nil, fmt.Errorf("Found '%d' statefulsets for labels '%s' instead of 1", len(statefulsets.Items), dev.LabelsSelector())
		}
		s = &statefulsets.Items[0]
	}

	return s, nil
}

//Create creates a statefulset
func Create(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) error {
	_, err := c.AppsV1().StatefulSets(sfs.Namespace).Create(ctx, sfs, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//DestroyDev destroys the k8s deployment of a dev environment
func DestroyDev(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return Destroy(ctx, dev.Name, dev.Namespace, c)
}

//Update updates a statefulset
func Update(ctx context.Context, sfs *appsv1.StatefulSet, c kubernetes.Interface) error {
	sfs.ResourceVersion = ""
	_, err := c.AppsV1().StatefulSets(sfs.Namespace).Update(ctx, sfs, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

//Destroy removes a statefulset object given its name and namespace
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	if err := c.AppsV1().StatefulSets(namespace).Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes job: %s", err)
	}
	log.Infof("statefulset '%s' deleted", name)
	return nil
}

func IsRunning(ctx context.Context, namespace, svcName string, c kubernetes.Interface) bool {
	sfs, err := c.AppsV1().StatefulSets(namespace).Get(ctx, svcName, metav1.GetOptions{})
	if err != nil {
		return false
	}
	return sfs.Status.ReadyReplicas > 0
}

//IsDevModeOn returns if a statefulset is in devmode
func IsDevModeOn(s *appsv1.StatefulSet) bool {
	labels := s.GetObjectMeta().GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[model.DevLabel]
	return ok
}

//HasBeenChanged returns if a statefulset has been updated since the development container was activated
func HasBeenChanged(s *appsv1.StatefulSet) bool {
	oktetoRevision := s.Annotations[model.RevisionAnnotation]
	if oktetoRevision == "" {
		return false
	}
	return oktetoRevision != s.Status.UpdateRevision
}

//SetLastBuiltAnnotation sets the deployment timestacmp
func SetLastBuiltAnnotation(s *appsv1.StatefulSet) {
	annotations.Set(s.Spec.Template.GetObjectMeta(), model.LastBuiltAnnotation, time.Now().UTC().Format(model.TimeFormat))
}

func GetRevisionAnnotatedStatefulsetOrFailed(ctx context.Context, dev *model.Dev, c kubernetes.Interface, waitUntilDeployed bool) (*appsv1.StatefulSet, error) {
	sfs, err := Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		if waitUntilDeployed && errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if err = checkConditionErrors(sfs, dev); err != nil {
		return nil, err
	}

	if sfs.Generation != sfs.Status.ObservedGeneration {
		return nil, nil
	}

	if err := updateOktetoRevision(ctx, sfs, c, dev.Timeout.Default); err != nil {
		return nil, err
	}

	return sfs, nil
}

func updateOktetoRevision(ctx context.Context, s *appsv1.StatefulSet, client kubernetes.Interface, timeout time.Duration) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	to := time.Now().Add(timeout * 2) // 60 seconds

	for retries := 0; ; retries++ {
		updated, err := client.AppsV1().StatefulSets(s.Namespace).Get(ctx, s.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get statefulset %s/%s: %w", s.Namespace, s.Name, err)
		}

		revision := updated.Status.UpdateRevision
		if revision != "" {
			if updated.Annotations[model.RevisionAnnotation] == revision {
				return nil
			}
			updated.Annotations[model.RevisionAnnotation] = revision
			return Update(ctx, updated, client)
		}

		if time.Now().After(to) && retries >= 10 {
			return fmt.Errorf("kubernetes is taking too long to update the '%s' annotation of the statefulset '%s'. Please check for errors and try again", model.RevisionAnnotation, s.Name)
		}

		select {
		case <-ticker.C:
			continue
		case <-ctx.Done():
			log.Info("call to deployments.UpdateOktetoRevision cancelled")
			return ctx.Err()
		}
	}
}

func checkConditionErrors(sfs *appsv1.StatefulSet, dev *model.Dev) error {
	for _, c := range sfs.Status.Conditions {
		if c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
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
