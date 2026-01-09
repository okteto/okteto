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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type fakeDivertDriver struct{}

func (*fakeDivertDriver) UpdatePod(podSpec apiv1.PodSpec) apiv1.PodSpec {
	podSpec.Containers = append(podSpec.Containers, apiv1.Container{Name: "diverted"})
	return podSpec
}

func (*fakeDivertDriver) UpdateVirtualService(vs *istioNetworkingV1beta1.VirtualService) {
	vs.Hosts = []string{"diverted.example.com"}
}

func (*fakeDivertDriver) Deploy(_ context.Context) error {
	return nil
}

func (*fakeDivertDriver) Destroy(_ context.Context) error {
	return nil
}

func TestTranslatorModifyDeployment(t *testing.T) {
	deployment := &appsv1.Deployment{
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

	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	err := translator.Modify(deployment)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", deployment.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", deployment.Spec.Template.Labels[model.DeployedByLabel])

	// Verify annotations were added
	assert.Equal(t, "true", deployment.ObjectMeta.Annotations[model.OktetoSampleAnnotation])

	// Verify divert driver was applied
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "nginx", deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "diverted", deployment.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorModifyStatefulSet(t *testing.T) {
	sts := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-statefulset",
		},
		Spec: appsv1.StatefulSetSpec{
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

	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	err := translator.Modify(sts)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", sts.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", sts.Spec.Template.Labels[model.DeployedByLabel])

	// Verify divert driver was applied
	assert.Len(t, sts.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", sts.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorModifyJob(t *testing.T) {
	job := &batchv1.Job{
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

	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	err := translator.Modify(job)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", job.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", job.Spec.Template.Labels[model.DeployedByLabel])

	// Verify divert driver was applied
	assert.Len(t, job.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", job.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorModifyCronJob(t *testing.T) {
	cronJob := &batchv1.CronJob{
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

	translator := newTranslator("test-deployer", &fakeDivertDriver{})
	err := translator.Modify(cronJob)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", cronJob.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", cronJob.Spec.JobTemplate.Spec.Template.Labels[model.DeployedByLabel])

	// Verify divert driver was applied
	assert.Len(t, cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers, 2)
	assert.Equal(t, "diverted", cronJob.Spec.JobTemplate.Spec.Template.Spec.Containers[1].Name)
}

func TestTranslatorModifyDaemonSet(t *testing.T) {
	ds := &appsv1.DaemonSet{
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

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(ds)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", ds.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", ds.Spec.Template.Labels[model.DeployedByLabel])
}

func TestTranslatorModifyReplicaSet(t *testing.T) {
	rs := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-replicaset",
		},
		Spec: appsv1.ReplicaSetSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(rs)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", rs.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", rs.Spec.Template.Labels[model.DeployedByLabel])
}

func TestTranslatorModifyReplicationController(t *testing.T) {
	rc := &apiv1.ReplicationController{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-rc",
		},
		Spec: apiv1.ReplicationControllerSpec{
			Template: &apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{Name: "nginx", Image: "nginx:latest"},
					},
				},
			},
		},
	}

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(rc)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", rc.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, "test-deployer", rc.Spec.Template.Labels[model.DeployedByLabel])
}

func TestTranslatorModifyPod(t *testing.T) {
	pod := &apiv1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-pod",
		},
		Spec: apiv1.PodSpec{
			Containers: []apiv1.Container{
				{Name: "nginx", Image: "nginx:latest"},
			},
		},
	}

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(pod)
	require.NoError(t, err)

	// Verify labels and annotations were added
	assert.Equal(t, "test-deployer", pod.Labels[model.DeployedByLabel])
	assert.Equal(t, "true", pod.Annotations[model.OktetoSampleAnnotation])
}

func TestTranslatorWithoutDivertDriver(t *testing.T) {
	deployment := &appsv1.Deployment{
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

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(deployment)
	require.NoError(t, err)

	// Verify labels were added
	assert.Equal(t, "test-deployer", deployment.ObjectMeta.Labels[model.DeployedByLabel])

	// Verify no divert modifications (only original container)
	assert.Len(t, deployment.Spec.Template.Spec.Containers, 1)
	assert.Equal(t, "nginx", deployment.Spec.Template.Spec.Containers[0].Name)
}

func TestTranslatorModifyUnstructured(t *testing.T) {
	// Create an unstructured object representing a CRD (like Okteto's External resource)
	unstructuredObj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "dev.okteto.com/v1",
			"kind":       "External",
			"metadata": map[string]interface{}{
				"name":      "test-external",
				"namespace": "default",
				"labels": map[string]interface{}{
					"app": "test",
				},
			},
			"spec": map[string]interface{}{
				"name": "test-external",
				"endpoints": []interface{}{
					map[string]interface{}{
						"name": "endpoint1",
						"url":  "https://example.com",
					},
				},
			},
		},
	}

	translator := newTranslator("test-deployer", nil)
	err := translator.Modify(unstructuredObj)
	require.NoError(t, err)

	// Verify labels were added
	labels := unstructuredObj.GetLabels()
	assert.Equal(t, "test-deployer", labels[model.DeployedByLabel])
	assert.Equal(t, "test", labels["app"]) // Original label preserved

	// Verify annotations were added
	annotations := unstructuredObj.GetAnnotations()
	assert.Equal(t, "true", annotations[model.OktetoSampleAnnotation])
}
