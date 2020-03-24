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

	"github.com/okteto/okteto/pkg/errors"
	okLabels "github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

//Get returns a deployment object given its name and namespace
func Get(dev *model.Dev, namespace string, c *kubernetes.Clientset) (*appsv1.Deployment, error) {
	if namespace == "" {
		return nil, fmt.Errorf("empty namespace")
	}

	var d *appsv1.Deployment
	var err error

	if len(dev.Labels) == 0 {
		d, err = c.AppsV1().Deployments(namespace).Get(dev.Name, metav1.GetOptions{})
		if err != nil {
			log.Debugf("error while retrieving deployment %s/%s: %s", namespace, dev.Name, err)
			return nil, err
		}
	} else {
		deploys, err := c.AppsV1().Deployments(namespace).List(
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
func GetRevisionAnnotatedDeploymentOrFailed(dev *model.Dev, c *kubernetes.Clientset, waitUntilDeployed bool) (*appsv1.Deployment, error) {
	d, err := Get(dev, dev.Namespace, c)
	if err != nil {
		if waitUntilDeployed && errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}

	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentReplicaFailure && c.Reason == "FailedCreate" && c.Status == apiv1.ConditionTrue {
			if strings.Contains(c.Message, "exceeded quota") {
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

//GetTranslations fills all the deployments pointed by a dev environment
func GetTranslations(dev *model.Dev, d *appsv1.Deployment, c *kubernetes.Clientset) (map[string]*model.Translation, error) {
	result := map[string]*model.Translation{}
	if d != nil {
		rule := dev.ToTranslationRule(dev)
		result[d.Name] = &model.Translation{
			Interactive: true,
			Name:        dev.Name,
			Version:     model.TranslationVersion,
			Deployment:  d,
			Annotations: dev.Annotations,
			Replicas:    *d.Spec.Replicas,
			Rules:       []*model.TranslationRule{rule},
		}
	}
	for _, s := range dev.Services {
		d, err := Get(s, dev.Namespace, c)
		if err != nil {
			return nil, err
		}
		rule := s.ToTranslationRule(dev)
		if _, ok := result[d.Name]; ok {
			result[d.Name].Rules = append(result[d.Name].Rules, rule)
		} else {
			result[d.Name] = &model.Translation{
				Name:        dev.Name,
				Interactive: false,
				Version:     model.TranslationVersion,
				Deployment:  d,
				Annotations: dev.Annotations,
				Replicas:    *d.Spec.Replicas,
				Rules:       []*model.TranslationRule{rule},
			}
		}
	}
	return result, nil
}

//Deploy creates or updates a deployment
func Deploy(d *appsv1.Deployment, forceCreate bool, client *kubernetes.Clientset) error {
	if forceCreate {
		if err := create(d, client); err != nil {
			return err
		}
	} else {
		if err := update(d, client); err != nil {
			return err
		}
	}
	return nil
}

//UpdateOktetoRevision updates the okteto version annotation
func UpdateOktetoRevision(ctx context.Context, d *appsv1.Deployment, client *kubernetes.Clientset) error {
	tries := 0
	ticker := time.NewTicker(200 * time.Millisecond)
	for tries < maxRetriesUpdateRevision {
		updated, err := client.AppsV1().Deployments(d.Namespace).Get(d.Name, metav1.GetOptions{})
		if err != nil {
			log.Debugf("error while retrieving deployment %s/%s: %s", d.Namespace, d.Name, err)
			return err
		}
		revision := updated.Annotations[revisionAnnotation]
		if revision != "" {
			d.Annotations[okLabels.RevisionAnnotation] = revision
			if err := update(d, client); err != nil {
				return err
			}
			return nil
		}
		select {
		case <-ticker.C:
			tries++
			continue
		case <-ctx.Done():
			log.Debug("cancelling call to update okteto revision")
			return ctx.Err()
		}
	}
	return fmt.Errorf("kubernetes is taking too long to update the '%s' annotation of the deployment '%s'. Please check for errors and try again", revisionAnnotation, d.Name)
}

//TranslateDevMode translates the deployment manifests to put them in dev mode
func TranslateDevMode(tr map[string]*model.Translation, ns *apiv1.Namespace, c *kubernetes.Clientset) error {
	for _, t := range tr {
		err := translate(t, ns, c)
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

//HasBeenChanged returns if a deployment has been updated since the development environment was activated
func HasBeenChanged(d *appsv1.Deployment) bool {
	oktetoRevision := d.Annotations[okLabels.RevisionAnnotation]
	if oktetoRevision == "" {
		return false
	}
	return oktetoRevision != d.Annotations[revisionAnnotation]
}

// UpdateDeployments update all deployments in the given translaation list
func UpdateDeployments(trList map[string]*model.Translation, c *kubernetes.Clientset) error {
	for _, tr := range trList {
		if tr.Deployment == nil {
			continue
		}
		if err := update(tr.Deployment, c); err != nil {
			return err
		}
	}
	return nil
}

//TranslateDevModeOff reverse the dev mode translation
func TranslateDevModeOff(d *appsv1.Deployment) (*appsv1.Deployment, error) {
	trRulesJSON := getAnnotation(d.Spec.Template.GetObjectMeta(), okLabels.TranslationAnnotation)
	if len(trRulesJSON) == 0 {
		dManifest := getAnnotation(d.GetObjectMeta(), oktetoDeploymentAnnotation)
		if len(dManifest) == 0 {
			log.Infof("%s/%s is not a development environment", d.Namespace, d.Name)
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
	d.GetObjectMeta().SetAnnotations(annotations)
	annotations = d.Spec.Template.GetObjectMeta().GetAnnotations()
	if err := deleteUserAnnotations(annotations); err != nil {
		return nil, err
	}
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

func create(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Debugf("creating deployment %s/%s", d.Namespace, d.Name)
	_, err := c.AppsV1().Deployments(d.Namespace).Create(d)
	if err != nil {
		return err
	}
	return nil
}

func update(d *appsv1.Deployment, c *kubernetes.Clientset) error {
	log.Debugf("updating deployment %s/%s", d.Namespace, d.Name)
	d.ResourceVersion = ""
	d.Status = appsv1.DeploymentStatus{}
	_, err := c.AppsV1().Deployments(d.Namespace).Update(d)
	if err != nil {
		return err
	}
	return nil
}

func deleteUserAnnotations(annotations map[string]string) error {
	tr, err := getTranslationFromAnnotation(annotations)
	if err != nil {
		return err
	}
	userAnnotations := tr.Annotations
	for key := range userAnnotations {
		delete(annotations, key)
	}
	return nil
}

//Destroy destroys a k8s service
func Destroy(dev *model.Dev, c *kubernetes.Clientset) error {
	log.Infof("deleting deployment '%s'...", dev.Name)
	dClient := c.AppsV1().Deployments(dev.Namespace)
	err := dClient.Delete(dev.Name, &metav1.DeleteOptions{GracePeriodSeconds: &devTerminationGracePeriodSeconds})
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			log.Infof("deployment '%s' was already deleted.", dev.Name)
			return nil
		}
		return fmt.Errorf("error deleting kubernetes deployment: %s", err)
	}
	log.Infof("deployment '%s' deleted", dev.Name)
	return nil
}
