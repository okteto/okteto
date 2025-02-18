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
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/k8s/labels"
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type jsonTranslator struct {
	name         string
	divertDriver divert.Driver
}

func newJSONTranslator(name string, divertDriver divert.Driver) *jsonTranslator {
	return &jsonTranslator{
		name:         name,
		divertDriver: divertDriver,
	}
}

func (j *jsonTranslator) Translate(b []byte) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(b, &body); err != nil {
		oktetoLog.Infof("error unmarshalling resource body on proxy: %s", err.Error())
		return nil, nil
	}

	if err := j.translateMetadata(body); err != nil {
		return nil, err
	}

	var typeMeta metav1.TypeMeta
	if err := json.Unmarshal(b, &typeMeta); err != nil {
		oktetoLog.Infof("error unmarshalling typemeta on proxy: %s", err.Error())
		return nil, nil
	}

	switch typeMeta.Kind {
	case "Deployment":
		if err := j.translateDeploymentSpec(body); err != nil {
			return nil, err
		}
	case "StatefulSet":
		if err := j.translateStatefulSetSpec(body); err != nil {
			return nil, err
		}
	case "Job":
		if err := j.translateJobSpec(body); err != nil {
			return nil, err
		}
	case "CronJob":
		if err := j.translateCronJobSpec(body); err != nil {
			return nil, err
		}
	case "DaemonSet":
		if err := j.translateDaemonSetSpec(body); err != nil {
			return nil, err
		}
	case "ReplicationController":
		if err := j.translateReplicationControllerSpec(body); err != nil {
			return nil, err
		}
	case "ReplicaSet":
		if err := j.translateReplicaSetSpec(body); err != nil {
			return nil, err
		}
	case "VirtualService":
		if err := j.translateVirtualServiceSpec(body); err != nil {
			return nil, err
		}
	}

	return json.Marshal(body)
}

func (j *jsonTranslator) translateMetadata(body map[string]json.RawMessage) error {
	m, ok := body["metadata"]
	if !ok {
		return fmt.Errorf("request body doesn't have metadata field")
	}

	var metadata metav1.ObjectMeta
	if err := json.Unmarshal(m, &metadata); err != nil {
		oktetoLog.Infof("error unmarshalling objectmeta on proxy: %s", err.Error())
		return nil
	}

	labels.SetInMetadata(&metadata, model.DeployedByLabel, j.name)

	if metadata.Annotations == nil {
		metadata.Annotations = map[string]string{}
	}
	if utils.IsOktetoRepo() {
		metadata.Annotations[model.OktetoSampleAnnotation] = "true"
	}

	metadataAsByte, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not process resource's metadata: %w", err)
	}

	body["metadata"] = metadataAsByte

	return nil
}

func (j *jsonTranslator) translateDeploymentSpec(body map[string]json.RawMessage) error {
	var spec appsv1.DeploymentSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling deployment spec on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process deployment's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateStatefulSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.StatefulSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling statefulset spec on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process statefulset's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateJobSpec(body map[string]json.RawMessage) error {
	var spec batchv1.JobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling job spec on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process job's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateCronJobSpec(body map[string]json.RawMessage) error {
	var spec batchv1.CronJobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling cronjob spec on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.JobTemplate.Spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.JobTemplate.Spec.Template.Spec = j.applyDivertToPod(spec.JobTemplate.Spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process cronjob's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateDaemonSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.DaemonSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling daemonset spec on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process daemonset's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateReplicationControllerSpec(body map[string]json.RawMessage) error {
	var spec apiv1.ReplicationControllerSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling replicationcontroller on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicationcontroller's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}

func (j *jsonTranslator) translateReplicaSetSpec(body map[string]json.RawMessage) error {
	var spec appsv1.ReplicaSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling replicaset on proxy: %s", err.Error())
		return nil
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, j.name)
	spec.Template.Spec = j.applyDivertToPod(spec.Template.Spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicaset's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}
func (j *jsonTranslator) applyDivertToPod(podSpec apiv1.PodSpec) apiv1.PodSpec {
	if j.divertDriver == nil {
		return podSpec
	}
	return j.divertDriver.UpdatePod(podSpec)
}

func (j *jsonTranslator) translateVirtualServiceSpec(body map[string]json.RawMessage) error {
	if j.divertDriver == nil {
		return nil
	}

	var spec *istioNetworkingV1beta1.VirtualService
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		oktetoLog.Infof("error unmarshalling replicaset on proxy: %s", err.Error())
		return nil
	}
	j.divertDriver.UpdateVirtualService(spec)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process virtual service's spec: %w", err)
	}
	body["spec"] = specAsByte
	return nil
}
