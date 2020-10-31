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

package deployments

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/okteto/okteto/pkg/config"
	"github.com/okteto/okteto/pkg/errors"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

//List returns the list of deployments
func List(ctx context.Context, namespace string, c kubernetes.Interface) ([]appsv1.Deployment, error) {
	dList, err := c.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return dList.Items, nil
}

//Get returns a deployment object given its name and namespace
func Get(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	if len(dev.Labels) > 0 {
		return getByLabel(ctx, dev.LabelsSelector(), namespace, c)
	}

	return getByName(ctx, dev.Name, namespace, c)
}

func getByName(ctx context.Context, name, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	var dep *appsv1.Deployment
	err := retry.OnError(config.DefaultBackoff, errors.IsTransient, func() error {
		d, err := c.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		dep = d
		return nil
	})

	if err != nil {
		return nil, err
	}

	return dep, nil
}

func getByLabel(ctx context.Context, selector, namespace string, c kubernetes.Interface) (*appsv1.Deployment, error) {
	var deploys *appsv1.DeploymentList
	err := retry.OnError(config.DefaultBackoff, errors.IsTransient, func() error {
		d, err := c.AppsV1().Deployments(namespace).List(ctx,
			metav1.ListOptions{
				LabelSelector: selector,
			},
		)

		if err != nil {
			return err
		}

		deploys = d
		return nil
	})

	if err != nil {
		return nil, err
	}

	if len(deploys.Items) == 0 {
		return nil, fmt.Errorf("deployment for labels '%s' not found", selector)
	}
	if len(deploys.Items) > 1 {
		return nil, fmt.Errorf("found '%d' deployments for labels '%s' instead of 1", len(deploys.Items), selector)
	}
	return &deploys.Items[0], nil
}

//GetRevisionAnnotatedDeploymentOrFailed returns a deployment object if it is healthy and annotated with its revision or an error
func GetRevisionAnnotatedDeploymentOrFailed(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*appsv1.Deployment, error) {
	d, err := Get(ctx, dev, dev.Namespace, c)
	if err != nil {
		if waitUntilDeployed && errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
				log.Infof("%s: %s", errors.ErrQuota, c.Message)
				return nil, errors.ErrQuota
			}
			return nil, fmt.Errorf(c.Message)
		}
	}

	if d.Generation != d.Status.ObservedGeneration {
		return nil, nil
	}

	return d, nil
}

//GetTranslations fills all the deployments pointed by a development container
func GetTranslations(ctx context.Context, dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) (map[string]*model.Translation, error) {
	result := map[string]*model.Translation{}
	if d != nil {
		rule := dev.ToTranslationRule(dev)
		result[d.Name] = &model.Translation{
			Interactive: true,
			Name:        dev.Name,
			Version:     model.TranslationVersion,
			Deployment:  d,
			Annotations: dev.Annotations,
			Tolerations: dev.Tolerations,
			Replicas:    *d.Spec.Replicas,
			Rules:       []*model.TranslationRule{rule},
		}
	}

	if err := loadServiceTranslations(ctx, dev, result, c); err != nil {
		return nil, err
	}

	return result, nil
}

func loadServiceTranslations(ctx context.Context, dev *model.Dev, result map[string]*model.Translation, c kubernetes.Interface) error {
	for _, s := range dev.Services {
		d, err := Get(ctx, s, dev.Namespace, c)
		if err != nil {
			return err
		}

		rule := s.ToTranslationRule(dev)

		if _, ok := result[d.Name]; ok {
			result[d.Name].Rules = append(result[d.Name].Rules, rule)
			continue
		}

		result[d.Name] = &model.Translation{
			Name:        dev.Name,
			Interactive: false,
			Version:     model.TranslationVersion,
			Deployment:  d,
			Annotations: dev.Annotations,
			Tolerations: dev.Tolerations,
			Replicas:    *d.Spec.Replicas,
			Rules:       []*model.TranslationRule{rule},
		}

	}

	return nil
}

//Deploy creates or updates a deployment
func Deploy(ctx context.Context, d *appsv1.Deployment, forceCreate bool, client *kubernetes.Clientset) error {
	if forceCreate {
		if err := create(ctx, d, client); err != nil {
			return err
		}
	} else {
		if err := update(ctx, d, client); err != nil {
			return err
		}
	}
	return nil
}

//UpdateOktetoRevision updates the okteto version annotation
func UpdateOktetoRevision(ctx context.Context, d *appsv1.Deployment, client *kubernetes.Clientset) error {
	ticker := time.NewTicker(200 * time.Millisecond)
	timeout := time.Now().Add(2 * config.GetTimeout()) // 60 seconds

	for i := 0; ; i++ {
		updated, err := getByName(ctx, d.GetName(), d.GetNamespace(), client)
		if err != nil {
			return err
		}

		revision := updated.Annotations[revisionAnnotation]
		if revision != "" {
			d.Annotations[okLabels.RevisionAnnotation] = revision
			return update(ctx, d, client)
		}

		if time.Now().After(timeout) {
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

//TranslateDevMode translates the deployment manifests to put them in dev mode
func TranslateDevMode(tr map[string]*model.Translation, c *kubernetes.Clientset, isOktetoNamespace bool) error {
	for _, t := range tr {
		err := translate(t, c, isOktetoNamespace)
		if err != nil {
			return err
		}
	}
	return nil
}

//IsDevModeOn returns if a deployment is in devmode
func IsDevModeOn(d *appsv1.Deployment) bool {
	labels := d.GetObjectMeta().GetLabels()
	if labels == nil {
		return false
	}
	_, ok := labels[okLabels.DevLabel]
	return ok
}

//HasBeenChanged returns if a deployment has been updated since the development container was activated
func HasBeenChanged(d *appsv1.Deployment) bool {
	oktetoRevision := d.Annotations[okLabels.RevisionAnnotation]
	if oktetoRevision == "" {
		return false
	}
	return oktetoRevision != d.Annotations[revisionAnnotation]
}

// UpdateDeployments update all deployments in the given translation list
func UpdateDeployments(ctx context.Context, trList map[string]*model.Translation, c *kubernetes.Clientset) error {
	for _, tr := range trList {
		if tr.Deployment == nil {
			continue
		}
		if err := update(ctx, tr.Deployment, c); err != nil {
			return err
		}
	}
	return nil
}

//TranslateDevModeOff reverses the dev mode translation
func TranslateDevModeOff(d *appsv1.Deployment) (*appsv1.Deployment, error) {
	trRulesJSON := getAnnotation(d.Spec.Template.GetObjectMeta(), okLabels.TranslationAnnotation)
	if trRulesJSON == "" {
		dManifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
		if dManifest == "" {
			log.Infof("%s/%s is not a development container", d.Namespace, d.Name)
			return d, nil
		}
		dOrig := &appsv1.Deployment{}
		if err := json.Unmarshal([]byte(dManifest), dOrig); err != nil {
			return nil, fmt.Errorf("malformed manifest: %s", err)
		}
		return dOrig, nil
	}
	trRules := &model.Translation{}
	if err := json.Unmarshal([]byte(trRulesJSON), trRules); err != nil {
		return nil, fmt.Errorf("malformed tr rules: %s", err)
	}
	d.Spec.Replicas = &trRules.Replicas
	annotations := d.GetObjectMeta().GetAnnotations()
	delete(annotations, oktetoVersionAnnotation)
	if err := deleteUserAnnotations(annotations, trRules); err != nil {
		return nil, err
	}
	d.GetObjectMeta().SetAnnotations(annotations)
	annotations = d.Spec.Template.GetObjectMeta().GetAnnotations()
	delete(annotations, okLabels.TranslationAnnotation)
	delete(annotations, model.OktetoRestartAnnotation)
	d.Spec.Template.GetObjectMeta().SetAnnotations(annotations)
	labels := d.GetObjectMeta().GetLabels()
	delete(labels, okLabels.DevLabel)
	delete(labels, okLabels.InteractiveDevLabel)
	delete(labels, okLabels.DetachedDevLabel)
	d.GetObjectMeta().SetLabels(labels)
	labels = d.Spec.Template.GetObjectMeta().GetLabels()
	delete(labels, okLabels.InteractiveDevLabel)
	delete(labels, okLabels.DetachedDevLabel)
	d.Spec.Template.GetObjectMeta().SetLabels(labels)
	return d, nil
}

func create(ctx context.Context, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	_, err := c.AppsV1().Deployments(d.Namespace).Create(ctx, d, metav1.CreateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func update(ctx context.Context, d *appsv1.Deployment, c *kubernetes.Clientset) error {
	d.ResourceVersion = ""
	d.Status = appsv1.DeploymentStatus{}
	_, err := c.AppsV1().Deployments(d.Namespace).Update(ctx, d, metav1.UpdateOptions{})
	if err != nil {
		return err
	}
	return nil
}

func deleteUserAnnotations(annotations map[string]string, tr *model.Translation) error {
	if tr.Annotations == nil {
		return nil
	}
	for key := range tr.Annotations {
		delete(annotations, key)
	}
	return nil
}

//Destroy destroys a k8s service
func Destroy(ctx context.Context, dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("deleting deployment '%s'", dev.Name)
	dClient := c.AppsV1().Deployments(dev.Namespace)
	err := dClient.Delete(ctx, dev.Name, metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			return nil
		}
		return fmt.Errorf("error deleting kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' deleted", dev.Name)
	return nil
}
