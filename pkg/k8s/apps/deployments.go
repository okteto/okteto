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

	"github.com/okteto/okteto/pkg/k8s/annotations"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

type DeploymentApp struct {
	d *appsv1.Deployment
}

func NewDeploymentApp(d *appsv1.Deployment) *DeploymentApp {
	return &DeploymentApp{d: d}
}

func (i *DeploymentApp) TypeMeta() metav1.TypeMeta {
	return i.d.TypeMeta
}

func (i *DeploymentApp) ObjectMeta() metav1.ObjectMeta {
	if i.d.ObjectMeta.Annotations == nil {
		i.d.ObjectMeta.Annotations = map[string]string{}
	}
	if i.d.ObjectMeta.Labels == nil {
		i.d.ObjectMeta.Labels = map[string]string{}
	}
	return i.d.ObjectMeta
}

func (i *DeploymentApp) TemplateObjectMeta() metav1.ObjectMeta {
	if i.d.Spec.Template.ObjectMeta.Annotations == nil {
		i.d.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	if i.d.Spec.Template.ObjectMeta.Labels == nil {
		i.d.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	return i.d.Spec.Template.ObjectMeta
}

func (i *DeploymentApp) Replicas() int32 {
	return *i.d.Spec.Replicas
}

func (i *DeploymentApp) PodSpec() *apiv1.PodSpec {
	return &i.d.Spec.Template.Spec
}

func (i *DeploymentApp) NewTranslation(dev *model.Dev) *Translation {
	return &Translation{
		Interactive:        true,
		Name:               dev.Name,
		Version:            model.TranslationVersion,
		Annotations:        dev.Annotations,
		Tolerations:        dev.Tolerations,
		Replicas:           i.Replicas(),
		App:                i,
		DeploymentStrategy: i.d.Spec.Strategy,
	}
}

func (i *DeploymentApp) IsDevModeOn() bool {
	return deployments.IsDevModeOn(i.d)
}

func (i *DeploymentApp) DevModeOn() {
	i.d.Spec.Replicas = pointer.Int32Ptr(1)
	i.d.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}
}

func (i *DeploymentApp) DevModeOff(t *Translation) {
	i.d.Spec.Replicas = pointer.Int32Ptr(t.Replicas)
	i.d.Spec.Strategy = t.DeploymentStrategy

	delete(i.d.Annotations, oktetoVersionAnnotation)
	delete(i.d.Annotations, model.OktetoRevisionAnnotation)
	deleteUserAnnotations(i.d.Annotations, t)

	delete(i.d.Spec.Template.Annotations, model.TranslationAnnotation)
	delete(i.d.Spec.Template.Annotations, model.OktetoRestartAnnotation)

	delete(i.d.Labels, model.DevLabel)

	delete(i.d.Spec.Template.Labels, model.InteractiveDevLabel)
	delete(i.d.Spec.Template.Labels, model.DetachedDevLabel)
}

func (i *DeploymentApp) CheckConditionErrors(dev *model.Dev) error {
	return deployments.CheckConditionErrors(i.d, dev)
}

func (i *DeploymentApp) SetOktetoRevision() {
	i.d.Annotations[model.OktetoRevisionAnnotation] = i.d.Annotations[model.DeploymentRevisionAnnotation]
}

func (i *DeploymentApp) GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error) {
	rs, err := replicasets.GetReplicaSetByDeployment(ctx, i.d, c)
	if err != nil {
		return nil, err
	}
	return pods.GetPodByReplicaSet(ctx, rs, c)
}

func (i *DeploymentApp) Divert(ctx context.Context, username string, dev *model.Dev, c kubernetes.Interface) (App, error) {
	d, err := deployments.GetByDev(ctx, dev, dev.Namespace, c)
	if err != nil {
		return nil, fmt.Errorf("error diverting deployment: %s", err.Error())
	}

	divertDeployment := translateDivertDeployment(username, d)
	if err := deployments.Deploy(ctx, divertDeployment, c); err != nil {
		return nil, fmt.Errorf("error creating diver deployment '%s': %s", divertDeployment.Name, err.Error())
	}
	return &DeploymentApp{d: divertDeployment}, nil
}

func (i *DeploymentApp) DestroyDivert(ctx context.Context, username string, dev *model.Dev, c kubernetes.Interface) error {
	d, err := deployments.GetByDev(ctx, dev, dev.Namespace, c)
	if err != nil {
		return fmt.Errorf("error diverting deployment: %s", err.Error())
	}

	divertDeploymentName := DivertName(username, d.Name)
	if err := deployments.Destroy(ctx, divertDeploymentName, d.Namespace, c); err != nil {
		return fmt.Errorf("error creating diver deployment '%s': %s", divertDeploymentName, err.Error())
	}
	return nil
}

func translateDivertDeployment(username string, d *appsv1.Deployment) *appsv1.Deployment {
	result := d.DeepCopy()
	result.UID = ""
	result.Name = DivertName(username, d.Name)
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
	annotations.Set(result.GetObjectMeta(), model.OktetoAutoCreateAnnotation, model.OktetoUpCmd)
	result.ResourceVersion = ""
	return result
}

func (i *DeploymentApp) SetOriginal() error {
	delete(i.d.Annotations, model.DeploymentAnnotation)
	i.d.Status = appsv1.DeploymentStatus{}
	manifestBytes, err := json.Marshal(i.d)
	if err != nil {
		return err
	}
	i.d.Annotations[model.DeploymentAnnotation] = string(manifestBytes)
	return nil
}

func (i *DeploymentApp) RestoreOriginal() error {
	manifest := i.d.Annotations[model.DeploymentAnnotation]
	if manifest == "" {
		return nil
	}
	log.Info("deprecated devmodeoff behavior")
	dOrig := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
		return fmt.Errorf("malformed manifest: %v", err)
	}
	i.d = dOrig
	return nil
}

func (i *DeploymentApp) HasBeenChanged() bool {
	return deployments.HasBeenChanged(i.d)
}

func (i *DeploymentApp) SetLastBuiltAnnotation() {
	deployments.SetLastBuiltAnnotation(i.d)
}

func (i *DeploymentApp) Refresh(ctx context.Context, c kubernetes.Interface) error {
	d, err := deployments.Get(ctx, i.d.Name, i.d.Namespace, c)
	if err == nil {
		i.d = d
	}
	return err
}

func (i *DeploymentApp) Deploy(ctx context.Context, c kubernetes.Interface) error {
	return deployments.Deploy(ctx, i.d, c)
}

func (i *DeploymentApp) Create(ctx context.Context, c kubernetes.Interface) error {
	d, err := deployments.Create(ctx, i.d, c)
	if err == nil {
		i.d = d
	}
	return err
}

func (i *DeploymentApp) DestroyDev(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return deployments.DestroyDev(ctx, dev, c)
}

func (i *DeploymentApp) Update(ctx context.Context, c kubernetes.Interface) error {
	d, err := deployments.Update(ctx, i.d, c)
	if err == nil {
		i.d = d
	}
	return err
}
