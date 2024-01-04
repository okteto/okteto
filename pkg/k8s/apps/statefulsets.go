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

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
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

type StatefulSetApp struct {
	sfs  *appsv1.StatefulSet
	kind string
}

func NewStatefulSetApp(sfs *appsv1.StatefulSet) *StatefulSetApp {
	return &StatefulSetApp{kind: okteto.StatefulSet, sfs: sfs}
}

func (i *StatefulSetApp) Kind() string {
	return i.kind
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

func (i *StatefulSetApp) Replicas() int32 {
	return *i.sfs.Spec.Replicas
}

func (i *StatefulSetApp) SetReplicas(n int32) {
	i.sfs.Spec.Replicas = pointer.Int32(n)
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

func (i *StatefulSetApp) DevClone() App {
	clone := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        model.DevCloneName(i.sfs.Name),
			Namespace:   i.sfs.Namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: *i.sfs.Spec.DeepCopy(),
	}
	clone.Labels[model.DevCloneLabel] = string(i.sfs.UID)
	for k, v := range i.sfs.Labels {
		clone.Labels[k] = v
	}
	for k, v := range i.sfs.Annotations {
		clone.Annotations[k] = v
	}
	return NewStatefulSetApp(clone)
}

func (i *StatefulSetApp) CheckConditionErrors(dev *model.Dev) error {
	return statefulsets.CheckConditionErrors(i.sfs, dev)
}

func (i *StatefulSetApp) GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error) {
	if i.sfs.Generation != i.sfs.Status.ObservedGeneration {
		return nil, oktetoErrors.ErrNotFound
	}
	return pods.GetPodByStatefulSet(ctx, i.sfs, c)
}

func (i *StatefulSetApp) RestoreOriginal() error {
	manifest := i.sfs.Annotations[model.StatefulsetAnnotation]
	if manifest == "" {
		return nil
	}
	oktetoLog.Info("depreccated devmodeoff behavior")
	sfsOrig := &appsv1.StatefulSet{}
	if err := json.Unmarshal([]byte(manifest), sfsOrig); err != nil {
		return fmt.Errorf("malformed manifest: %w", err)
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

func (i *StatefulSetApp) Watch(ctx context.Context, result chan error, c kubernetes.Interface) {
	optsWatch := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", i.sfs.Name),
	}

	watcher, err := c.AppsV1().StatefulSets(i.sfs.Namespace).Watch(ctx, optsWatch)
	if err != nil {
		result <- err
		return
	}

	for {
		select {
		case e := <-watcher.ResultChan():
			oktetoLog.Debugf("Received statefulset '%s' event: %s", i.sfs.Name, e)
			if e.Object == nil {
				oktetoLog.Debugf("Recreating statefulset '%s' watcher", i.sfs.Name)
				watcher, err = c.AppsV1().StatefulSets(i.sfs.Namespace).Watch(ctx, optsWatch)
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
				sfs, ok := e.Object.(*appsv1.StatefulSet)
				if !ok {
					oktetoLog.Debugf("Failed to parse statefulset event: %s", e)
					continue
				}
				if sfs.Generation != i.sfs.Generation {
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

func (i *StatefulSetApp) Deploy(ctx context.Context, c kubernetes.Interface) error {
	sfs, err := statefulsets.Deploy(ctx, i.sfs, c)
	if err == nil {
		i.sfs = sfs
	}
	return err
}

func (i *StatefulSetApp) PatchAnnotations(ctx context.Context, c kubernetes.Interface) error {
	return statefulsets.PatchAnnotations(ctx, i.sfs, c)
}

func (i *StatefulSetApp) Destroy(ctx context.Context, c kubernetes.Interface) error {
	return statefulsets.Destroy(ctx, i.sfs.Name, i.sfs.Namespace, c)
}

// GetDevClone Returns from Kubernetes the cloned statefulset
func (i *StatefulSetApp) GetDevClone(ctx context.Context, c kubernetes.Interface) (App, error) {
	clonedName := model.DevCloneName(i.sfs.Name)
	sfs, err := statefulsets.Get(ctx, clonedName, i.sfs.Namespace, c)
	if err == nil {
		return NewStatefulSetApp(sfs), nil
	}
	return nil, err
}
