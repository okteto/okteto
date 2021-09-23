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
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

type StatefulSetApp struct {
	sfs *appsv1.StatefulSet
}

func NewStatefulSetApp(sfs *appsv1.StatefulSet) *StatefulSetApp {
	return &StatefulSetApp{sfs: sfs}
}

func (i *StatefulSetApp) Replicas() int32 {
	return *i.sfs.Spec.Replicas
}

func (i *StatefulSetApp) TypeMeta() metav1.TypeMeta {
	return i.sfs.TypeMeta
}

func (i *StatefulSetApp) ObjectMeta() metav1.ObjectMeta {
	if i.sfs.ObjectMeta.Annotations == nil {
		i.sfs.ObjectMeta.Annotations = map[string]string{}
	}
	if i.sfs.ObjectMeta.Labels == nil {
		i.sfs.ObjectMeta.Labels = map[string]string{}
	}
	return i.sfs.ObjectMeta
}

func (i *StatefulSetApp) TemplateObjectMeta() metav1.ObjectMeta {
	if i.sfs.Spec.Template.ObjectMeta.Annotations == nil {
		i.sfs.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	if i.sfs.Spec.Template.ObjectMeta.Labels == nil {
		i.sfs.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	return i.sfs.Spec.Template.ObjectMeta
}

func (i *StatefulSetApp) PodSpec() *apiv1.PodSpec {
	return &i.sfs.Spec.Template.Spec
}

func (i *StatefulSetApp) NewTranslation(dev *model.Dev) *Translation {
	return &Translation{
		Interactive:         true,
		Name:                dev.Name,
		Version:             model.TranslationVersion,
		Annotations:         dev.Annotations,
		Tolerations:         dev.Tolerations,
		Replicas:            i.Replicas(),
		App:                 i,
		StatefulsetStrategy: i.sfs.Spec.UpdateStrategy,
	}
}

func (i *StatefulSetApp) DevModeOn() {
	i.sfs.Spec.Replicas = pointer.Int32Ptr(1)
	i.sfs.Spec.UpdateStrategy = appsv1.StatefulSetUpdateStrategy{
		Type: appsv1.RollingUpdateStatefulSetStrategyType,
	}
}

func (i *StatefulSetApp) DevModeOff(t *Translation) {
	i.sfs.Spec.Replicas = pointer.Int32Ptr(t.Replicas)
	i.sfs.Spec.UpdateStrategy = t.StatefulsetStrategy
}

func (i *StatefulSetApp) CheckConditionErrors(dev *model.Dev) error {
	return statefulsets.CheckConditionErrors(i.sfs, dev)
}

func (i *StatefulSetApp) GetRevision() string {
	return i.sfs.Status.UpdateRevision
}

func (i *StatefulSetApp) GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error) {
	return pods.GetPodByStatefulSet(ctx, i.sfs, c)
}

func (i *StatefulSetApp) Divert(ctx context.Context, username string, dev *model.Dev, c kubernetes.Interface) (App, error) {
	sfs, err := statefulsets.GetByDev(ctx, dev, dev.Namespace, c)
	if err != nil {
		return nil, fmt.Errorf("error diverting statefulset: %s", err.Error())
	}
	divertStatefulset := translateStatefulset(username, sfs)
	if err := statefulsets.Deploy(ctx, divertStatefulset, c); err != nil {
		return nil, fmt.Errorf("error creating divert statefulset '%s': %s", divertStatefulset.Name, err.Error())
	}
	return &StatefulSetApp{sfs: divertStatefulset}, nil
}

func translateStatefulset(username string, d *appsv1.StatefulSet) *appsv1.StatefulSet {
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

func (i *StatefulSetApp) SetOriginal() error {
	delete(i.sfs.Annotations, model.StatefulsetAnnotation)
	i.sfs.Status = appsv1.StatefulSetStatus{}
	manifestBytes, err := json.Marshal(i.sfs)
	if err != nil {
		return err
	}
	i.sfs.Annotations[model.StatefulsetAnnotation] = string(manifestBytes)
	return nil
}

func (i *StatefulSetApp) RestoreOriginal() error {
	manifest := i.sfs.Annotations[model.StatefulsetAnnotation]
	if manifest == "" {
		return nil
	}
	log.Info("depreccated devmodeoff behavior")
	sfsOrig := &appsv1.StatefulSet{}
	if err := json.Unmarshal([]byte(manifest), sfsOrig); err != nil {
		return fmt.Errorf("malformed manifest: %v", err)
	}
	i.sfs = sfsOrig
	return nil
}

func (i *StatefulSetApp) Refresh(ctx context.Context, c kubernetes.Interface) error {
	sfs, err := statefulsets.Get(ctx, i.sfs.Name, i.sfs.Namespace, c)
	if err == nil {
		i.sfs = sfs
	}
	return err
}

func (i *StatefulSetApp) Create(ctx context.Context, c kubernetes.Interface) error {
	sfs, err := statefulsets.Create(ctx, i.sfs, c)
	if err == nil {
		i.sfs = sfs
	}
	return err
}

func (i *StatefulSetApp) Update(ctx context.Context, c kubernetes.Interface) error {
	sfs, err := statefulsets.Update(ctx, i.sfs, c)
	if err == nil {
		i.sfs = sfs
	}
	return err
}

func (i *StatefulSetApp) Destroy(ctx context.Context, dev *model.Dev, c kubernetes.Interface) error {
	return statefulsets.Destroy(ctx, dev.Name, dev.Namespace, c)
}
