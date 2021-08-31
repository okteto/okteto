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
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	revisionAnnotation = "deployment.kubernetes.io/revision"
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

//GetDeploymentByName returns a deployment object if is created
func GetDeploymentByName(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	d, err := c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get deployment %s/%s: %w", namespace, name, err)
	}

	return d, nil
}

//Get returns a deployment object given its name and namespace
func Get(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	var d *appsv1.Deployment
	var err error

	if len(dev.Labels) == 0 {
		d, err = c.AppsV1().Deployments(namespace).Get(ctx, dev.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		deploys, err := c.AppsV1().Deployments(namespace).List(
			ctx,
			metav1.ListOptions{
				LabelSelector: dev.LabelsSelector(),
			},
		)
		if err != nil {
			return nil, err
		}
		if len(deploys.Items) == 0 {
			return nil, fmt.Errorf("deployment for labels '%s' not found", dev.LabelsSelector())
		}
		if len(deploys.Items) > 1 {
			return nil, fmt.Errorf("Found '%d' deployments for labels '%s' instead of 1", len(deploys.Items), dev.LabelsSelector())
		}
		d = &deploys.Items[0]
	}
	return d, nil
}

//GetRevisionAnnotatedDeploymentOrFailed returns a deployment object if it is healthy and annotated with its revision or an error
func GetRevisionAnnotatedDeploymentOrFailed(ctx context.Context, dev *model.Dev, c kubernetes.Interface, waitUntilDeployed bool) (*appsv1.Deployment, error) {
	d, err := Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		if waitUntilDeployed && errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	if err = checkConditionErrors(d, dev); err != nil {
		return nil, err
	}

	if d.Generation != d.Status.ObservedGeneration {
		return nil, nil
	}

	if err := updateOktetoRevision(ctx, d, c, dev.Timeout.Default); err != nil {
		return nil, err
	}

	return d, nil
}

func checkConditionErrors(deployment *appsv1.Deployment, dev *model.Dev) error {
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

func Create(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) error {
	_, err := c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func Update(ctx context.Context, d *appsv1.Deployment, c kubernetes.Interface) error {
	d.ResourceVersion = ""
	d.Status = appsv1.DeploymentStatus{}
	_, err := c.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
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
		if err := Update(ctx, old, c); err != nil {
			return fmt.Errorf("error updating kubernetes deployment: %s", err)
		}
		log.Infof("updated deployment '%s'.", d.Name)
	}
	return nil
}

func updateOktetoRevision(ctx context.Context, d *appsv1.Deployment, client kubernetes.Interface, timeout time.Duration) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	to := time.Now().Add(timeout * 2) // 60 seconds

	for retries := 0; ; retries++ {
		updated, err := client.AppsV1().Deployments(d.Namespace).Get(ctx, d.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment %s/%s: %w", d.Namespace, d.Name, err)
		}

		revision := updated.Annotations[revisionAnnotation]
		if revision != "" {
			if updated.Annotations[model.RevisionAnnotation] == revision {
				return nil
			}
			updated.Annotations[model.RevisionAnnotation] = revision
			return Update(ctx, updated, client)
		}

		if time.Now().After(to) && retries >= 10 {
			return fmt.Errorf("kubernetes is taking too long to update the '%s' annotation of the deployment '%s'. Please check for errors and try again", revisionAnnotation, d.Name)
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

//SetLastBuiltAnnotation sets the deployment timestacmp
func SetLastBuiltAnnotation(d *appsv1.Deployment) {
	annotations.Set(d.Spec.Template.GetObjectMeta(), model.LastBuiltAnnotation, time.Now().UTC().Format(model.TimeFormat))
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

//HasBeenChanged returns if a deployment has been updated since the development container was activated
func HasBeenChanged(d *appsv1.Deployment) bool {
	oktetoRevision := d.Annotations[model.RevisionAnnotation]
	if oktetoRevision == "" {
		return false
	}
	return oktetoRevision != d.Annotations[revisionAnnotation]
}

// UpdateDeployments update all deployments in the given translation list
func UpdateDeployments(ctx context.Context, trList map[string]*model.Translation, c kubernetes.Interface) error {
	for _, tr := range trList {
		if tr.K8sObject.Deployment == nil {
			continue
		}
		if err := Update(ctx, tr.K8sObject.Deployment, c); err != nil {
			return err
		}
	}
	return nil
}

//DestroyDev destroys the k8s deployment of a dev environment
func DestroyDev(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return Destroy(ctx, dev.Name, dev.Namespace, c)
}

//Destroy destroys a k8s deployment
func Destroy(ctx context.Context, name, namespace string, c kubernetes.Interface) error {
	log.Infof("deleting deployment '%s'", name)
	dClient := c.AppsV1().Deployments(namespace)
	var devTerminationGracePeriodSeconds int64
	err := dClient.Delete(ctx, name, metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
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
