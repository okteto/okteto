// Copyright 2022 The Okteto Authors
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

package deploy

import (
	"encoding/json"
	"fmt"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/k8s/labels"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func translateBody(b []byte, name string) ([]byte, error) {
	var body map[string]json.RawMessage
	if err := json.Unmarshal(b, &body); err != nil {
		return nil, fmt.Errorf("could not unmarshal request: %s", err)
	}

	if err := translateMetadata(body, name); err != nil {
		return nil, err
	}

	var typeMeta metav1.TypeMeta
	if err := json.Unmarshal(b, &typeMeta); err != nil {
		return nil, fmt.Errorf("could not process resource's type: %s", err)
	}

	switch typeMeta.Kind {
	case "Deployment":
		if err := translateDeploymentSpec(body, name); err != nil {
			return nil, err
		}
	case "StatefulSet":
		if err := translateStatefulSetSpec(body, name); err != nil {
			return nil, err
		}
	case "Job":
		if err := translateJobSpec(body, name); err != nil {
			return nil, err
		}
	case "CronJob":
		if err := translateCronJobSpec(body, name); err != nil {
			return nil, err
		}
	case "DaemonSet":
		if err := translateDaemonSetSpec(body, name); err != nil {
			return nil, err
		}
	case "ReplicationController":
		if err := translateReplicationControllerSpec(body, name); err != nil {
			return nil, err
		}
	case "ReplicaSet":
		if err := translateReplicaSetSpec(body, name); err != nil {
			return nil, err
		}
	}

	return json.Marshal(body)
}

func translateMetadata(body map[string]json.RawMessage, name string) error {
	m, ok := body["metadata"]
	if !ok {
		return fmt.Errorf("request body doesn't have metadata field")
	}

	var metadata metav1.ObjectMeta
	if err := json.Unmarshal(m, &metadata); err != nil {
		return fmt.Errorf("could not process resource's metadata: %s", err)
	}

	labels.SetInMetadata(&metadata, model.DeployedByLabel, name)

	if metadata.Annotations == nil {
		metadata.Annotations = map[string]string{}
	}
	if utils.IsOktetoRepo() {
		metadata.Annotations[model.OktetoSampleAnnotation] = "true"
	}

	metadataAsByte, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("could not process resource's metadata: %s", err)
	}

	body["metadata"] = metadataAsByte

	return nil
}

func translateDeploymentSpec(body map[string]json.RawMessage, name string) error {
	var spec appsv1.DeploymentSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process deployment spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process deployment's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateStatefulSetSpec(body map[string]json.RawMessage, name string) error {
	var spec appsv1.StatefulSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process statefulset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process statefulset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateJobSpec(body map[string]json.RawMessage, name string) error {
	var spec batchv1.JobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process job spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process job's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateCronJobSpec(body map[string]json.RawMessage, name string) error {
	var spec batchv1.CronJobSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process cronjob spec: %s", err)
	}
	labels.SetInMetadata(&spec.JobTemplate.Spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process cronjob's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateDaemonSetSpec(body map[string]json.RawMessage, name string) error {
	var spec appsv1.DaemonSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process daemonset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process daemonset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateReplicationControllerSpec(body map[string]json.RawMessage, name string) error {
	var spec apiv1.ReplicationControllerSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process replicationcontroller spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicationcontroller's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}

func translateReplicaSetSpec(body map[string]json.RawMessage, name string) error {
	var spec appsv1.ReplicaSetSpec
	if err := json.Unmarshal(body["spec"], &spec); err != nil {
		return fmt.Errorf("could not process replicaset spec: %s", err)
	}
	labels.SetInMetadata(&spec.Template.ObjectMeta, model.DeployedByLabel, name)
	specAsByte, err := json.Marshal(spec)
	if err != nil {
		return fmt.Errorf("could not process replicaset's spec: %s", err)
	}
	body["spec"] = specAsByte
	return nil
}
