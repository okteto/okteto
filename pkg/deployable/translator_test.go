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
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

type fakeDivertDriver struct{}

func (f *fakeDivertDriver) UpdatePod(podSpec apiv1.PodSpec) apiv1.PodSpec {
	podSpec.Containers = append(podSpec.Containers, apiv1.Container{Name: "diverted"})
	return podSpec
}

func (f *fakeDivertDriver) UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService) {
	vs.Hosts = []string{"diverted.example.com"}
}

func (f *fakeDivertDriver) Deploy(ctx context.Context) error {
	return nil
}

func (f *fakeDivertDriver) Destroy(ctx context.Context) error {
	return nil
}

func TestTranslatorWithJSON(t *testing.T) {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
			Labels: map[string]string{
				"app": "test",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	// Encode to JSON using Kubernetes serializer
	encoder := scheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion)
	var jsonBuf bytes.Buffer
	err := encoder.Encode(deployment, &jsonBuf)
	require.NoError(t, err)

	// Translate
	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	result, err := translator.Translate(jsonBuf.Bytes(), "application/json")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultDep, ok := obj.(*appsv1.Deployment)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultDep.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", resultDep.Spec.Template.Labels[model.DeployedByLabel])

	// Verify annotations were added
	assert.Equal(t, "true", resultDep.Annotations[model.OktetoSampleAnnotation])

	// Verify divert driver was applied
	assert.Len(t, resultDep.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", resultDep.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorWithYAML(t *testing.T) {
	yamlInput := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: test-statefulset
spec:
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: nginx
        image: nginx:latest`

	// Translate
	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	result, err := translator.Translate([]byte(yamlInput), "application/apply-patch+yaml")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultSts, ok := obj.(*appsv1.StatefulSet)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultSts.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", resultSts.Spec.Template.Labels[model.DeployedByLabel])

	// Verify divert driver was applied
	assert.Len(t, resultSts.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", resultSts.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorWithProtobuf(t *testing.T) {
	job := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-job",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "busybox", Image: "busybox:latest"},
					},
				},
			},
		},
	}

	// Encode to Protobuf
	pbSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	var pbBuf bytes.Buffer
	err := pbSerializer.Encode(job, &pbBuf)
	require.NoError(t, err)

	// Translate
	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	result, err := translator.Translate(pbBuf.Bytes(), "application/vnd.kubernetes.protobuf")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultJob, ok := obj.(*batchv1.Job)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultJob.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", resultJob.Spec.Template.Labels[model.DeployedByLabel])

	// Verify divert driver was applied
	assert.Len(t, resultJob.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", resultJob.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorWithoutDivertDriver(t *testing.T) {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-deployment",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	// Encode to JSON
	encoder := scheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion)
	var jsonBuf bytes.Buffer
	err := encoder.Encode(deployment, &jsonBuf)
	require.NoError(t, err)

	// Translate without divert driver
	translator := newTranslator("test-deployer", nil)
	result, err := translator.Translate(jsonBuf.Bytes(), "application/json")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultDep, ok := obj.(*appsv1.Deployment)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultDep.Labels[model.DeployedByLabel])

	// Verify no divert modifications (only original container)
	assert.Len(t, resultDep.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "nginx", resultDep.Spec.Template.Spec.Containers[0].Name)
}

func TestTranslatorWithDaemonSet(t *testing.T) {
	daemonSet := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-daemonset",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "fluentd", Image: "fluentd:latest"},
					},
				},
			},
		},
	}

	// Encode to JSON
	encoder := scheme.Codecs.LegacyCodec(appsv1.SchemeGroupVersion)
	var jsonBuf bytes.Buffer
	err := encoder.Encode(daemonSet, &jsonBuf)
	require.NoError(t, err)

	// Translate
	translator := newTranslator("test-deployer", nil)
	result, err := translator.Translate(jsonBuf.Bytes(), "application/json")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultDs, ok := obj.(*appsv1.DaemonSet)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultDs.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", resultDs.Spec.Template.Labels[model.DeployedByLabel])
}

func TestTranslatorWithCronJob(t *testing.T) {
	cronJob := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-cronjob",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: apiv1.PodTemplateSpec{
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{Name: "hello", Image: "busybox:latest"},
							},
						},
					},
				},
			},
		},
	}

	// Encode to JSON
	encoder := scheme.Codecs.LegacyCodec(batchv1.SchemeGroupVersion)
	var jsonBuf bytes.Buffer
	err := encoder.Encode(cronJob, &jsonBuf)
	require.NoError(t, err)

	// Translate
	translator := newTranslator("test-deployer", nil)
	result, err := translator.Translate(jsonBuf.Bytes(), "application/json")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultCron, ok := obj.(*batchv1.CronJob)
	require.True(t, ok)

	// Verify labels were added
	assert.Equal(t, "test-deployer", resultCron.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", resultCron.Spec.JobTemplate.Spec.Template.Labels[model.DeployedByLabel])
}

func TestTranslatorInvalidInput(t *testing.T) {
	invalidBytes := []byte("this is not valid kubernetes resource")

	translator := newTranslator("test-deployer", nil)
	result, err := translator.Translate(invalidBytes, "application/json")

	// Should not return an error but nil result
	assert.NoError(t, err)
	assert.Nil(t, result)
}

func TestTranslatorWithPod(t *testing.T) {
	pod := &apiv1.Pod{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}

	// Encode to JSON
	encoder := scheme.Codecs.LegacyCodec(apiv1.SchemeGroupVersion)
	var jsonBuf bytes.Buffer
	err := encoder.Encode(pod, &jsonBuf)
	require.NoError(t, err)

	// Translate
	translator := newTranslator("test-deployer", nil)
	result, err := translator.Translate(jsonBuf.Bytes(), "application/json")
	require.NoError(t, err)

	// Decode result
	decoder := scheme.Codecs.UniversalDeserializer()
	obj, _, err := decoder.Decode(result, nil, nil)
	require.NoError(t, err)

	resultPod, ok := obj.(*apiv1.Pod)
	require.True(t, ok)

	// Verify labels and annotations were added
	assert.Equal(t, "test-deployer", resultPod.Labels[model.DeployedByLabel])
	assert.Equal(t, "true", resultPod.Annotations[model.OktetoSampleAnnotation])
}
