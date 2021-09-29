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

package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

func Get(ctx context.Context, dev *model.Dev, namespace string, c kubernetes.Interface) (App, error) {
	d, err := deployments.GetByDev(ctx, dev, namespace, c)

	if err == nil {
		return &DeploymentApp{d: d}, nil
	}

	if !errors.IsNotFound(err) {
		return nil, err
	}

	sfs, err := statefulsets.GetByDev(ctx, dev, namespace, c)
	if err != nil {
		return nil, err
	}

	return &StatefulSetApp{sfs: sfs}, nil
}

//IsDevModeOn returns if a statefulset is in devmode
func IsDevModeOn(app App) bool {
	return app.ObjectMeta().Labels[model.DevLabel] == "true"
}

func (t *Translation) DevModeOff() {
	t.App.DevModeOff(t)
	delete(t.App.ObjectMeta().Annotations, oktetoVersionAnnotation)
	delete(t.App.ObjectMeta().Annotations, model.OktetoRevisionAnnotation)
	deleteUserAnnotations(t.App.ObjectMeta().Annotations, t)

	delete(t.App.TemplateObjectMeta().Annotations, model.TranslationAnnotation)
	delete(t.App.TemplateObjectMeta().Annotations, model.OktetoRestartAnnotation)

	delete(t.App.ObjectMeta().Labels, model.DevLabel)

	delete(t.App.TemplateObjectMeta().Labels, model.InteractiveDevLabel)
	delete(t.App.TemplateObjectMeta().Labels, model.DetachedDevLabel)

}

//HasBeenChanged returns if an app has been updated since the development container was activated
func HasBeenChanged(app App) bool {
	oktetoRevision := app.ObjectMeta().Annotations[model.OktetoRevisionAnnotation]
	if oktetoRevision == "" {
		return false
	}
	return oktetoRevision != app.GetRevision()
}

//SetLastBuiltAnnotation sets the app timestamp
func SetLastBuiltAnnotation(app App) {
	app.ObjectMeta().Annotations[model.LastBuiltAnnotation] = time.Now().UTC().Format(model.TimeFormat)
}

//GetDeploymentSandbox returns a base deployment when using "autocreate"
func GetDeploymentSandbox(dev *model.Dev) *appsv1.Deployment {
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

//GetStatefulSetSandbox returns a base statefulset when using "autocreate"
func GetStatefulSetSandbox(dev *model.Dev) *appsv1.StatefulSet {
	image := dev.Image.Name
	if image == "" {
		image = model.DefaultImage
	}
	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      dev.Name,
			Namespace: dev.Namespace,
			Labels:    model.Labels{},
			Annotations: model.Annotations{
				model.OktetoAutoCreateAnnotation: model.OktetoUpCmd,
			},
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

// GetRunningPodInLoop returns the dev pod for an app and loops until it success
func GetRunningPodInLoop(ctx context.Context, dev *model.Dev, app App, c kubernetes.Interface) (*apiv1.Pod, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	start := time.Now()
	to := start.Add(dev.Timeout.Resources)

	for retries := 0; ; retries++ {
		err := app.Refresh(ctx, c)
		if err != nil {
			return nil, err
		}
		if err = app.CheckConditionErrors(dev); err != nil {
			return nil, err
		}

		pod, err := app.GetRunningPod(ctx, c)

		if err == nil {
			if !okteto.IsOktetoContext() {
				app.ObjectMeta().Annotations[model.OktetoRevisionAnnotation] = app.GetRevision()
				if err := app.Update(ctx, c); err != nil {
					return nil, err
				}
			}
			return pod, nil
		}

		if !errors.IsNotFound(err) {
			return nil, err
		}

		if time.Now().After(to) && retries > 10 {
			return nil, fmt.Errorf("kubernetes is taking too long to start your development container. Please check for errors and try again")
		}

		select {
		case <-ticker.C:
			if retries%5 == 0 {
				log.Info("development container is not ready yet, will retry")
			}
			continue
		case <-ctx.Done():
			log.Debug("call to apps.GetRunningPodInLoop cancelled")
			return nil, ctx.Err()
		}
	}
}

//GetTranslations fills all the deployments pointed by a development container
func GetTranslations(ctx context.Context, dev *model.Dev, app App, reset bool, c kubernetes.Interface) (map[string]*Translation, error) {
	result := map[string]*Translation{}
	t := app.NewTranslation(dev)
	trRulesJSON := app.TemplateObjectMeta().Annotations[model.TranslationAnnotation]
	if trRulesJSON != "" {
		trRules := &Translation{}
		if err := json.Unmarshal([]byte(trRulesJSON), trRules); err != nil {
			return nil, fmt.Errorf("malformed tr rules: %s", err)
		}
		t.Replicas = trRules.Replicas
		t.DeploymentStrategy = trRules.DeploymentStrategy
		t.StatefulsetStrategy = trRules.StatefulsetStrategy
	} else {
		t.Replicas = getPreviousAppReplicas(app)
	}

	rule := dev.ToTranslationRule(dev, reset)
	t.Rules = []*model.TranslationRule{rule}
	result[app.ObjectMeta().Name] = t

	if err := loadServiceTranslations(ctx, dev, reset, result, c); err != nil {
		return nil, err
	}

	for _, rule := range t.Rules {
		devContainer := GetDevContainer(t.App.PodSpec(), rule.Container)
		if devContainer == nil {
			return nil, fmt.Errorf("%s '%s': container '%s' not found", t.App.TypeMeta().Kind, t.App.ObjectMeta().Name, rule.Container)
		}
		rule.Container = devContainer.Name
		if rule.Image == "" {
			rule.Image = devContainer.Image
		}
	}

	return result, nil
}

func loadServiceTranslations(ctx context.Context, dev *model.Dev, reset bool, result map[string]*Translation, c kubernetes.Interface) error {
	for _, s := range dev.Services {
		app, err := Get(ctx, s, dev.Namespace, c)
		if err != nil {
			return err
		}

		rule := s.ToTranslationRule(dev, reset)

		if _, ok := result[app.ObjectMeta().Name]; ok {
			result[app.ObjectMeta().Name].Rules = append(result[app.ObjectMeta().Name].Rules, rule)
			continue
		}

		t := app.NewTranslation(dev)
		t.Interactive = false
		t.Rules = []*model.TranslationRule{rule}
		result[app.ObjectMeta().Name] = t
	}

	return nil
}

//TranslateDevMode translates the deployment manifests to put them in dev mode
func TranslateDevMode(tr map[string]*Translation) error {
	for _, t := range tr {
		err := translate(t)
		if err != nil {
			return err
		}
	}
	return nil
}

// DivertName returns the name of the diverted version of a given resource
func DivertName(username, name string) string {
	return fmt.Sprintf("%s-%s", username, name)
}
