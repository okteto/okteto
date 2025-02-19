// Copyright 2025 The Okteto Authors
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

package deployable

import (
	"bytes"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

type protobufTranslator struct {
	serializer   *protobuf.Serializer
	divertDriver divert.Driver
	name         string
}

func newProtobufTranslator(name string, divertDriver divert.Driver) *protobufTranslator {
	protobufSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	return &protobufTranslator{
		name:         name,
		serializer:   protobufSerializer,
		divertDriver: divertDriver,
	}
}

func (p *protobufTranslator) Translate(b []byte) ([]byte, error) {
	// Passing nil for defaultGVK and into, so the serializer infers the GVK from the data and creates a new object.
	// This is necessary because the object is not known at compile time.
	obj, _, err := p.serializer.Decode(b, nil, nil)
	if err != nil {
		oktetoLog.Infof("error unmarshalling resource body on proxy: %s", err.Error())
		return nil, fmt.Errorf("could not unmarshal resource body: %w", err)
	}

	if err := p.translateMetadata(obj); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := p.serializer.Encode(obj, &buf); err != nil {
		return nil, fmt.Errorf("could not encode resource: %w", err)
	}

	switch obj.GetObjectKind().GroupVersionKind().Kind {
	case "Deployment":
		if err := p.translateDeploymentSpec(obj); err != nil {
			return nil, err
		}
	case "StatefulSet":
		if err := p.translateStatefulSetSpec(obj); err != nil {
			return nil, err
		}
	case "Job":
		if err := p.translateJobSpec(obj); err != nil {
			return nil, err
		}
	case "CronJob":
		if err := p.translateCronJobSpec(obj); err != nil {
			return nil, err
		}
	case "DaemonSet":
		if err := p.translateDaemonSetSpec(obj); err != nil {
			return nil, err
		}
	case "ReplicationController":
		if err := p.translateReplicationControllerSpec(obj); err != nil {
			return nil, err
		}
	case "ReplicaSet":
		if err := p.translateReplicaSetSpec(obj); err != nil {
			return nil, err
		}

	}

	return buf.Bytes(), nil
}

func (p *protobufTranslator) translateMetadata(obj runtime.Object) error {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return fmt.Errorf("could not access object metadata: %w", err)
	}

	// Update labels directly instead of using labels.SetInMetadata.
	currentLabels := metaObj.GetLabels()
	if currentLabels == nil {
		currentLabels = make(map[string]string)
	}
	currentLabels[model.DeployedByLabel] = p.name
	metaObj.SetLabels(currentLabels)

	// Update annotations directly.
	annotations := metaObj.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	if utils.IsOktetoRepo() {
		annotations[model.OktetoSampleAnnotation] = "true"
	}
	metaObj.SetAnnotations(annotations)

	return nil
}

func (p *protobufTranslator) translateDeploymentSpec(obj runtime.Object) error {
	deployment, ok := obj.(*appsv1.Deployment)
	if !ok {
		return fmt.Errorf("expected *appsv1.Deployment, got %T", obj)
	}

	labels.SetInMetadata(&deployment.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	deployment.Spec.Template.Spec = p.applyDivertToPod(deployment.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateStatefulSetSpec(obj runtime.Object) error {
	sts, ok := obj.(*appsv1.StatefulSet)
	if !ok {
		return fmt.Errorf("expected *appsv1.Statefulset, got %T", obj)
	}

	labels.SetInMetadata(&sts.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	sts.Spec.Template.Spec = p.applyDivertToPod(sts.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateJobSpec(obj runtime.Object) error {
	job, ok := obj.(*batchv1.Job)
	if !ok {
		return fmt.Errorf("expected *batchv1.Job, got %T", obj)
	}

	labels.SetInMetadata(&job.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	job.Spec.Template.Spec = p.applyDivertToPod(job.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateCronJobSpec(obj runtime.Object) error {
	cronJob, ok := obj.(*batchv1.CronJob)
	if !ok {
		return fmt.Errorf("expected *batchv1.CronJob, got %T", obj)
	}

	labels.SetInMetadata(&cronJob.Spec.JobTemplate.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	cronJob.Spec.JobTemplate.Spec.Template.Spec = p.applyDivertToPod(cronJob.Spec.JobTemplate.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateDaemonSetSpec(obj runtime.Object) error {
	daemonSet, ok := obj.(*appsv1.DaemonSet)
	if !ok {
		return fmt.Errorf("expected *appsv1.DaemonSet, got %T", obj)
	}

	labels.SetInMetadata(&daemonSet.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	daemonSet.Spec.Template.Spec = p.applyDivertToPod(daemonSet.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateReplicationControllerSpec(obj runtime.Object) error {
	replicationController, ok := obj.(*apiv1.ReplicationController)
	if !ok {
		return fmt.Errorf("expected *apiv1.ReplicationController, got %T", obj)
	}

	labels.SetInMetadata(&replicationController.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	replicationController.Spec.Template.Spec = p.applyDivertToPod(replicationController.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) translateReplicaSetSpec(obj runtime.Object) error {
	replicaSet, ok := obj.(*appsv1.ReplicaSet)
	if !ok {
		return fmt.Errorf("expected *appsv1.ReplicaSet, got %T", obj)
	}

	labels.SetInMetadata(&replicaSet.Spec.Template.ObjectMeta, model.DeployedByLabel, p.name)

	replicaSet.Spec.Template.Spec = p.applyDivertToPod(replicaSet.Spec.Template.Spec)

	return nil
}

func (p *protobufTranslator) applyDivertToPod(podSpec apiv1.PodSpec) apiv1.PodSpec {
	if p.divertDriver == nil {
		return podSpec
	}
	return p.divertDriver.UpdatePod(podSpec)
}
