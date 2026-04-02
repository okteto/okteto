// Copyright 2026 The Okteto Authors
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
	"strconv"

	rolloutsv1alpha1 "github.com/argoproj/argo-rollouts/pkg/apis/rollouts/v1alpha1"
	rolloutsclientset "github.com/argoproj/argo-rollouts/pkg/client/clientset/versioned"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/pods"
	"github.com/okteto/okteto/pkg/k8s/replicasets"
	k8srollouts "github.com/okteto/okteto/pkg/k8s/rollouts"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
)

type RolloutApp struct {
	r  *rolloutsv1alpha1.Rollout
	rc rolloutsclientset.Interface
}

func NewRolloutApp(r *rolloutsv1alpha1.Rollout, rc rolloutsclientset.Interface) *RolloutApp {
	return &RolloutApp{r: r, rc: rc}
}

func (i *RolloutApp) Kind() string {
	return okteto.Rollout
}

func (i *RolloutApp) ObjectMeta() metav1.ObjectMeta {
	if i.r.ObjectMeta.Annotations == nil {
		i.r.ObjectMeta.Annotations = map[string]string{}
	}
	if i.r.ObjectMeta.Labels == nil {
		i.r.ObjectMeta.Labels = map[string]string{}
	}
	return i.r.ObjectMeta
}

func (i *RolloutApp) Replicas() int32 {
	if i.r.Spec.Replicas == nil {
		return 1
	}
	return *i.r.Spec.Replicas
}

func (i *RolloutApp) SetReplicas(n int32) {
	i.r.Spec.Replicas = ptr.To(n)
}

func (i *RolloutApp) TemplateObjectMeta() metav1.ObjectMeta {
	if i.r.Spec.Template.ObjectMeta.Annotations == nil {
		i.r.Spec.Template.ObjectMeta.Annotations = map[string]string{}
	}
	if i.r.Spec.Template.ObjectMeta.Labels == nil {
		i.r.Spec.Template.ObjectMeta.Labels = map[string]string{}
	}
	return i.r.Spec.Template.ObjectMeta
}

func (i *RolloutApp) PodSpec() *apiv1.PodSpec {
	return &i.r.Spec.Template.Spec
}

func (i *RolloutApp) DevClone() App {
	clone := &rolloutsv1alpha1.Rollout{
		TypeMeta: metav1.TypeMeta{
			Kind:       i.r.TypeMeta.Kind,
			APIVersion: i.r.TypeMeta.APIVersion,
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        model.DevCloneName(i.r.Name),
			Namespace:   i.r.Namespace,
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
		Spec: *i.r.Spec.DeepCopy(),
	}

	clone.Spec.WorkloadRef = nil
	clone.Labels[model.DevCloneLabel] = string(i.r.UID)

	for k, v := range i.r.Labels {
		clone.Labels[k] = v
	}
	for k, v := range i.r.Annotations {
		clone.Annotations[k] = v
	}

	delete(clone.Annotations, model.OktetoAutoCreateAnnotation)
	clone.Spec.Strategy = rolloutsv1alpha1.RolloutStrategy{
		Canary: &rolloutsv1alpha1.CanaryStrategy{
			MaxSurge:       ptr.To(intstr.FromInt(0)),
			MaxUnavailable: ptr.To(intstr.FromInt(1)),
		},
	}

	return NewRolloutApp(clone, i.rc)
}

func (i *RolloutApp) CheckConditionErrors(dev *model.Dev) error {
	return k8srollouts.CheckConditionErrors(i.r, dev)
}

func (i *RolloutApp) GetRunningPod(ctx context.Context, c kubernetes.Interface) (*apiv1.Pod, error) {
	if strconv.FormatInt(i.r.Generation, 10) != i.r.Status.ObservedGeneration {
		return nil, oktetoErrors.ErrNotFound
	}

	rs, err := replicasets.GetReplicaSetByRollout(ctx, i.r, c)
	if err != nil {
		return nil, err
	}
	return pods.GetPodByReplicaSet(ctx, rs, c)
}

func (i *RolloutApp) RestoreOriginal() error {
	manifest := i.r.Annotations[model.RolloutAnnotation]
	if manifest == "" {
		return nil
	}
	oktetoLog.Info("deprecated devmodeoff behavior")
	rOrig := &rolloutsv1alpha1.Rollout{}
	if err := json.Unmarshal([]byte(manifest), rOrig); err != nil {
		return fmt.Errorf("malformed manifest: %w", err)
	}
	i.r = rOrig
	return nil
}

func (i *RolloutApp) Refresh(ctx context.Context, _ kubernetes.Interface) error {
	r, err := k8srollouts.Get(ctx, i.r.Name, i.r.Namespace, i.rc)
	if err == nil {
		i.r = r
	}
	return err
}

func (i *RolloutApp) Watch(ctx context.Context, result chan error, _ kubernetes.Interface) {
	optsWatch := metav1.ListOptions{
		Watch:         true,
		FieldSelector: fmt.Sprintf("metadata.name=%s", i.r.Name),
	}

	watcher, err := i.rc.ArgoprojV1alpha1().Rollouts(i.r.Namespace).Watch(ctx, optsWatch)
	if err != nil {
		result <- err
		return
	}

	for {
		select {
		case e := <-watcher.ResultChan():
			oktetoLog.Debugf("Received rollout '%s' event: %s", i.r.Name, e)
			if e.Object == nil {
				oktetoLog.Debugf("Recreating rollout '%s' watcher", i.r.Name)
				watcher, err = i.rc.ArgoprojV1alpha1().Rollouts(i.r.Namespace).Watch(ctx, optsWatch)
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
				r, ok := e.Object.(*rolloutsv1alpha1.Rollout)
				if !ok {
					oktetoLog.Debugf("Failed to parse rollout event: %s", e)
					continue
				}
				if r.Generation != i.r.Generation {
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

func (i *RolloutApp) Deploy(ctx context.Context, _ kubernetes.Interface) error {
	r, err := k8srollouts.Deploy(ctx, i.r, i.rc)
	if err == nil {
		i.r = r
	}
	return err
}

func (i *RolloutApp) Destroy(ctx context.Context, _ kubernetes.Interface) error {
	return k8srollouts.Destroy(ctx, i.r.Name, i.r.Namespace, i.rc)
}

func (i *RolloutApp) PatchAnnotations(ctx context.Context, _ kubernetes.Interface) error {
	return k8srollouts.PatchAnnotations(ctx, i.r, i.rc)
}

func (i *RolloutApp) GetDevClone(ctx context.Context, _ kubernetes.Interface) (App, error) {
	clonedName := model.DevCloneName(i.r.Name)
	r, err := k8srollouts.Get(ctx, clonedName, i.r.Namespace, i.rc)
	if err == nil {
		return NewRolloutApp(r, i.rc), nil
	}
	return nil, err
}
