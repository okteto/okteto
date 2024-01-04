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

package apps

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/pkg/constants"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/pointer"
)

type DeploymentApp struct {
	d    *appsv1.Deployment
	kind string
}

func NewDeploymentApp(d *appsv1.Deployment) *DeploymentApp {
	return &DeploymentApp{kind: okteto.Deployment, d: d}
}

func (i *DeploymentApp) Kind() string {
	return i.kind
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

func (i *DeploymentApp) Replicas() int32 {
	return *i.d.Spec.Replicas
}

func (i *DeploymentApp) SetReplicas(n int32) {
	i.d.Spec.Replicas = pointer.Int32(n)
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

func (i *DeploymentApp) PodSpec() *apiv1.PodSpec {
	return &i.d.Spec.Template.Spec
}

func (i *DeploymentApp) DevClone() App {
	clone := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        model.DevCloneName(i.d.Name),
			Namespace:   i.d.Namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: *i.d.Spec.DeepCopy(),
	}
	if i.d.Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd {
		clone.Labels[constants.DevLabel] = "true"
	} else {
		clone.Labels[model.DevCloneLabel] = string(i.d.UID)
	}
	for k, v := range i.d.Labels {
		clone.Labels[k] = v
	}
	for k, v := range i.d.Annotations {
		clone.Annotations[k] = v
	}
	delete(clone.Annotations, model.OktetoAutoCreateAnnotation)
	clone.Spec.Strategy = appsv1.DeploymentStrategy{
		Type: appsv1.RecreateDeploymentStrategyType,
	}
	return NewDeploymentApp(clone)
}

func (i *DeploymentApp) CheckConditionErrors(dev *model.Dev) error {
	return deployments.CheckConditionErrors(i.d, dev)
}

func (i *DeploymentApp) GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error) {
	rs, err := replicasets.GetReplicaSetByDeployment(ctx, i.d, c)
	if err != nil {
		return nil, err
	}
	return pods.GetPodByReplicaSet(ctx, rs, c)
}

func (i *DeploymentApp) RestoreOriginal() error {
	manifest := i.d.Annotations[model.DeploymentAnnotation]
	if manifest == "" {
		return nil
	}
	oktetoLog.Info("deprecated devmodeoff behavior")
	dOrig := &appsv1.Deployment{}
	if err := json.Unmarshal([]byte(manifest), dOrig); err != nil {
		return fmt.Errorf("malformed manifest: %w", err)
	}
	i.d = dOrig
	return nil
}

func (i *DeploymentApp) Refresh(ctx context.Context, c kubernetes.Interface) error {
	d, err := deployments.Get(ctx, i.d.Name, i.d.Namespace, c)
	if err == nil {
		i.d = d
	}
	return err
}

func (i *DeploymentApp) Watch(ctx context.Context, result chan error, c kubernetes.Interface) {
	optsWatch := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", i.d.Name),
	}

	watcher, err := c.AppsV1().Deployments(i.d.Namespace).Watch(ctx, optsWatch)
	if err != nil {
		result <- err
		return
	}

	for {
		select {
		case e := <-watcher.ResultChan():
			oktetoLog.Debugf("Received deployment '%s' event: %s", i.d.Name, e)
			if e.Object == nil {
				oktetoLog.Debugf("Recreating deployment '%s' watcher", i.d.Name)
				watcher, err = c.AppsV1().Deployments(i.d.Namespace).Watch(ctx, optsWatch)
				if err != nil {
					result <- err
					return
				}
				continue
			}
			if e.Type == watch.Deleted {
				result <- oktetoErrors.ErrDeleteToApp
				return
			} else if e.Type == watch.Modified {
				d, ok := e.Object.(*appsv1.Deployment)
				if !ok {
					oktetoLog.Debugf("Failed to parse deployment event: %s", e)
					continue
				}
				if d.Annotations[model.DeploymentRevisionAnnotation] != "" && d.Annotations[model.DeploymentRevisionAnnotation] != i.d.Annotations[model.DeploymentRevisionAnnotation] {
					result <- oktetoErrors.ErrApplyToApp
					return
				}
			}
		case err := <-ctx.Done():
			oktetoLog.Debugf("call to up.applyToApp cancelled: %v", err)
			return
		}
	}
}

func (i *DeploymentApp) Deploy(ctx context.Context, c kubernetes.Interface) error {
	if string(i.d.UID) == "" && i.d.Annotations[model.OktetoAutoCreateAnnotation] == model.OktetoUpCmd {
		return nil
	}

	d, err := deployments.Deploy(ctx, i.d, c)
	if err == nil {
		i.d = d
	}
	return err
}

func (i *DeploymentApp) Destroy(ctx context.Context, c kubernetes.Interface) error {
	return deployments.Destroy(ctx, i.d.Name, i.d.Namespace, c)
}

func (i *DeploymentApp) PatchAnnotations(ctx context.Context, c kubernetes.Interface) error {
	return deployments.PatchAnnotations(ctx, i.d, c)
}

// GetDevClone Returns from Kubernetes the cloned deployment
func (i *DeploymentApp) GetDevClone(ctx context.Context, c kubernetes.Interface) (App, error) {
	clonedName := model.DevCloneName(i.d.Name)
	d, err := deployments.Get(ctx, clonedName, i.d.Namespace, c)
	if err == nil {
		return NewDeploymentApp(d), nil
	}
	return nil, err
}
