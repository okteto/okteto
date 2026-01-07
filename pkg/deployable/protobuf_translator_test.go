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

	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
	"k8s.io/kubectl/pkg/scheme"
)

// runProtobufTranslatorTest is a helper that marshals the input object to protobuf,
// runs the translator, and returns the decoded output object.
func runProtobufTranslatorTest(t *testing.T, inputObj runtime.Object, translatorName string, dDriver divert.Driver) runtime.Object {
	// Encode input object to protobuf
	pbSerializer := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
	var buf bytes.Buffer
	err := pbSerializer.Encode(inputObj, &buf)
	require.NoError(t, err, "failed to encode object to protobuf")

	// Run translator
	translator := newProtobufTranslator(translatorName, dDriver)
	outBytes, err := translator.Translate(buf.Bytes())
	require.NoError(t, err, "Translate returned error")
	require.NotNil(t, outBytes, "expected non-nil output bytes")

	// Decode output bytes
	decodedObj, _, err := pbSerializer.Decode(outBytes, nil, nil)
	require.NoError(t, err, "failed to decode output bytes")

	return decodedObj
}

func TestProtobufTranslatorTranslateDeployment(t *testing.T) {
	inputObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployment-test",
			Labels: map[string]string{
				"original": "label",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	deployment, ok := decodedObj.(*appsv1.Deployment)
	require.True(t, ok, "decoded object is not a Deployment")

	// Verify top-level metadata
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify Deployment spec template metadata
	assert.Equal(t, translatorName, deployment.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	// Verify that the divert driver was applied (container "diverted" added)
	found := false
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorTranslateStatefulSet(t *testing.T) {
	inputObj := &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "StatefulSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "statefulset-test",
		},
		Spec: appsv1.StatefulSetSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "statefulset",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	sts, ok := decodedObj.(*appsv1.StatefulSet)
	require.True(t, ok, "decoded object is not a StatefulSet")

	assert.Equal(t, translatorName, sts.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")
	assert.Equal(t, translatorName, sts.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range sts.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorTranslateJob(t *testing.T) {
	inputObj := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "job-test",
		},
		Spec: batchv1.JobSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"job": "true",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "busybox"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	job, ok := decodedObj.(*batchv1.Job)
	require.True(t, ok, "decoded object is not a Job")

	assert.Equal(t, translatorName, job.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")
	assert.Equal(t, translatorName, job.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range job.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorTranslateCronJob(t *testing.T) {
	inputObj := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: "batch/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cronjob-test",
		},
		Spec: batchv1.CronJobSpec{
			Schedule: "*/5 * * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"cron": "job",
							},
						},
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{Name: "test-container", Image: "busybox"},
							},
						},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	cronJob, ok := decodedObj.(*batchv1.CronJob)
	require.True(t, ok, "decoded object is not a CronJob")

	assert.Equal(t, translatorName, cronJob.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")
	assert.Equal(t, translatorName, cronJob.Spec.JobTemplate.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorTranslateDaemonSet(t *testing.T) {
	inputObj := &appsv1.DaemonSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "DaemonSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "daemonset-test",
		},
		Spec: appsv1.DaemonSetSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"daemon": "true",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	daemonSet, ok := decodedObj.(*appsv1.DaemonSet)
	require.True(t, ok, "decoded object is not a DaemonSet")

	assert.Equal(t, translatorName, daemonSet.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")
	assert.Equal(t, translatorName, daemonSet.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range daemonSet.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorTranslateReplicaSet(t *testing.T) {
	inputObj := &appsv1.ReplicaSet{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicaSet",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "replicaset-test",
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "replicaset",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, &fakeDivertDriver{})

	rs, ok := decodedObj.(*appsv1.ReplicaSet)
	require.True(t, ok, "decoded object is not a ReplicaSet")

	assert.Equal(t, translatorName, rs.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")
	assert.Equal(t, translatorName, rs.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range rs.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestProtobufTranslatorWithNilDivertDriver(t *testing.T) {
	inputObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "deployment-test",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "test-container", Image: "nginx"},
					},
				},
			},
		},
	}

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, nil)

	deployment, ok := decodedObj.(*appsv1.Deployment)
	require.True(t, ok, "decoded object is not a Deployment")

	// Verify labels are set
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify that no diverted container was added (divert driver is nil)
	found := false
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.False(t, found, "expected no diverted container when divert driver is nil")

	// Verify we still have the original container
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1, "expected original container to remain")
	assert.Equal(t, "test-container", deployment.Spec.Template.Spec.Containers[0].Name)
}

func TestProtobufTranslatorInvalidInput(t *testing.T) {
	invalidBytes := []byte("this is not valid protobuf data")
	translator := newProtobufTranslator("test-deployer", nil)
	outputBytes, err := translator.Translate(invalidBytes)
	assert.Error(t, err, "Translate should return an error for invalid input")
	assert.Nil(t, outputBytes, "expected output bytes to be nil for invalid input")
}

func TestProtobufTranslatorTranslatePod(t *testing.T) {
	inputObj := &apiv1.Pod{
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

	translatorName := "test-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, nil)

	pod, ok := decodedObj.(*apiv1.Pod)
	require.True(t, ok, "decoded object is not a Pod")

	// Verify labels are set
	labels := pod.GetLabels()
	require.Equal(t, translatorName, labels[model.DeployedByLabel], "expected deployed-by label to be set")

	// Verify annotations are set
	annotations := pod.GetAnnotations()
	require.NotNil(t, annotations, "annotations should not be nil after translation")
	assert.Equal(t, "true", annotations[model.OktetoSampleAnnotation], "expected okteto sample annotation to be set")
}

func TestProtobufTranslatorDeploymentWithCustomDeployerName(t *testing.T) {
	inputObj := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-deployment",
			Namespace: "default",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"version": "v1",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "app", Image: "myapp:latest"},
					},
				},
			},
		},
	}

	translatorName := "custom-deployer"
	decodedObj := runProtobufTranslatorTest(t, inputObj, translatorName, nil)

	deployment, ok := decodedObj.(*appsv1.Deployment)
	require.True(t, ok, "decoded object is not a Deployment")

	// Verify the deployed-by label was injected with custom name
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, translatorName, deployment.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel])

	// Verify original fields are preserved
	assert.Equal(t, "my-deployment", deployment.ObjectMeta.Name)
	assert.Equal(t, "default", deployment.ObjectMeta.Namespace)
	assert.Equal(t, "v1", deployment.Spec.Template.ObjectMeta.Labels["version"])
	assert.Equal(t, "app", deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "myapp:latest", deployment.Spec.Template.Spec.Containers[0].Image)
}
