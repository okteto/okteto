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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

// fake object that does not support metadata.
type noMetaObject struct{}

func (n *noMetaObject) GetObjectKind() schema.ObjectKind {
	return schema.EmptyObjectKind
}

func (n *noMetaObject) DeepCopyObject() runtime.Object {
	return n
}

func TestProtobufTranslator_Translate_Success(t *testing.T) {
	// Create a sample Pod with some existing labels.
	pod := &apiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
			Labels: map[string]string{
				"existing": "value",
			},
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{Name: "container1", Image: "nginx"},
			},
		},
	}

	// Create a protobuf serializer (using the same scheme as the translator).
	pbSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	var buf bytes.Buffer
	err := pbSerializer.Encode(pod, &buf)
	require.NoError(t, err, "failed to encode pod to protobuf")
	inputBytes := buf.Bytes()

	translatorName := "test-deployer"
	translator := newProtobufTranslator(translatorName, nil)
	outputBytes, err := translator.Translate(inputBytes)
	require.NoError(t, err, "Translate returned an error")
	require.NotNil(t, outputBytes, "expected non-nil output bytes")

	decodedObj, _, err := pbSerializer.Decode(outputBytes, nil, nil)
	require.NoError(t, err, "failed to decode output bytes")

	podOut, ok := decodedObj.(*apiv1.Pod)
	require.True(t, ok, "decoded object is not a Pod")

	labels := podOut.GetLabels()
	require.Equal(t, translatorName, labels[model.DeployedByLabel], "expected deployed-by label to be set")

	annotations := podOut.GetAnnotations()
	require.NotNil(t, annotations, "annotations should not be nil after translation")
	assert.Equal(t, "true", annotations[model.OktetoSampleAnnotation], "expected okteto sample annotation to be set")
}

func TestProtobufTranslator_InvalidInput(t *testing.T) {
	invalidBytes := []byte("this is not valid protobuf data")
	translator := newProtobufTranslator("test-deployer", nil)
	outputBytes, err := translator.Translate(invalidBytes)
	assert.Error(t, err, "Translate should not return an error for invalid input")
	assert.Nil(t, outputBytes, "expected output bytes to be nil for invalid input")
}

func TestProtobufTranslator_translateMetadata_NoMetadata(t *testing.T) {
	translator := newProtobufTranslator("test-deployer", nil)
	obj := &noMetaObject{}
	err := translator.translateMetadata(obj)
	assert.Error(t, err, "expected error when object has no metadata")
}

func TestTranslateDeploymentSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid deployment",
			obj: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{}, // no labels set yet
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.StatefulSet{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateDeploymentSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			dep := tc.obj.(*appsv1.Deployment)
			label, exists := dep.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label, "unexpected label value")
			containers := dep.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateStatefulSetSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid statefulset",
			obj: &appsv1.StatefulSet{
				Spec: appsv1.StatefulSetSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateStatefulSetSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			sts := tc.obj.(*appsv1.StatefulSet)
			label, exists := sts.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := sts.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateJobSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid job",
			obj: &batchv1.Job{
				Spec: batchv1.JobSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateJobSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			job := tc.obj.(*batchv1.Job)
			label, exists := job.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := job.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateCronJobSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid cronjob",
			obj: &batchv1.CronJob{
				Spec: batchv1.CronJobSpec{
					JobTemplate: batchv1.JobTemplateSpec{
						Spec: batchv1.JobSpec{
							Template: apiv1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{},
								Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
							},
						},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateCronJobSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			cron := tc.obj.(*batchv1.CronJob)
			label, exists := cron.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := cron.Spec.JobTemplate.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateDaemonSetSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid daemonset",
			obj: &appsv1.DaemonSet{
				Spec: appsv1.DaemonSetSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateDaemonSetSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			ds := tc.obj.(*appsv1.DaemonSet)
			label, exists := ds.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := ds.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateReplicationControllerSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid replicationcontroller",
			obj: &apiv1.ReplicationController{
				Spec: apiv1.ReplicationControllerSpec{
					Template: &apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateReplicationControllerSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			rc := tc.obj.(*apiv1.ReplicationController)
			label, exists := rc.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := rc.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}

func TestTranslateReplicaSetSpec(t *testing.T) {
	tests := []struct {
		name        string
		obj         runtime.Object
		expectError bool
	}{
		{
			name: "valid replicaset",
			obj: &appsv1.ReplicaSet{
				Spec: appsv1.ReplicaSetSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{},
						Spec:       apiv1.PodSpec{Containers: []apiv1.Container{}},
					},
				},
			},
			expectError: false,
		},
		{
			name:        "invalid type",
			obj:         &appsv1.Deployment{},
			expectError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			translator := &protobufTranslator{
				name:         "test-name",
				divertDriver: &fakeDivertDriver{},
			}

			err := translator.translateReplicaSetSpec(tc.obj)
			if tc.expectError {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)

			rs := tc.obj.(*appsv1.ReplicaSet)
			label, exists := rs.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel]
			assert.True(t, exists, "expected label %q to exist", model.DeployedByLabel)
			assert.Equal(t, translator.name, label)
			containers := rs.Spec.Template.Spec.Containers
			assert.Len(t, containers, 1)
			assert.Equal(t, "diverted", containers[0].Name)
		})
	}
}
