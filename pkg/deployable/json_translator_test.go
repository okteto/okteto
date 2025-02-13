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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// fakeDivertDriver implements divert.Driver so we can simulate divert behavior.
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

// runTranslatorTest is a helper that marshals the input, runs the translator,
// and returns the resulting JSON as a map.
func runTranslatorTest(t *testing.T, input interface{}, translatorName string, dDriver divert.Driver) map[string]json.RawMessage {
	b, err := json.Marshal(input)
	assert.NoError(t, err, "failed to marshal input")
	translator := NewJSONTranslator(b, translatorName, dDriver)
	outBytes, err := translator.Translate()
	assert.NoError(t, err, "Translate returned error")
	var out map[string]json.RawMessage
	err = json.Unmarshal(outBytes, &out)
	assert.NoError(t, err, "failed to unmarshal translator output")
	return out
}

func TestJSONTranslatorTranslateDeployment(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":   "deployment-test",
			"labels": map[string]string{"original": "label"},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"app": "myapp"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer1"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	// Verify top-level metadata.
	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify Deployment spec template metadata.
	var spec appsv1.DeploymentSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal deployment spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	// Verify that the divert driver was applied (container "diverted" added).
	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateStatefulSet(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "StatefulSet",
		"metadata": map[string]interface{}{
			"name":   "statefulset-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"app": "statefulset"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec appsv1.StatefulSetSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal statefulset spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateJob(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "Job",
		"metadata": map[string]interface{}{
			"name":   "job-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"job": "true"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec batchv1.JobSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal job spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateCronJob(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "batch/v1",
		"kind":       "CronJob",
		"metadata": map[string]interface{}{
			"name":   "cronjob-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"jobTemplate": map[string]interface{}{
				"spec": map[string]interface{}{
					"template": map[string]interface{}{
						"metadata": map[string]interface{}{
							"labels": map[string]string{"cron": "job"},
						},
						"spec": map[string]interface{}{
							"containers": []interface{}{},
						},
					},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec batchv1.CronJobSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal cronjob spec")
	assert.Equal(t, translatorName, spec.JobTemplate.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.JobTemplate.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateDaemonSet(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "DaemonSet",
		"metadata": map[string]interface{}{
			"name":   "daemonset-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"daemon": "true"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec appsv1.DaemonSetSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal daemonset spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateReplicationController(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ReplicationController",
		"metadata": map[string]interface{}{
			"name":   "rc-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"rc": "true"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec apiv1.ReplicationControllerSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal replicationcontroller spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateReplicaSet(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "ReplicaSet",
		"metadata": map[string]interface{}{
			"name":   "rs-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]string{"rs": "true"},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{},
				},
			},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var spec appsv1.ReplicaSetSpec
	err = json.Unmarshal(out["spec"], &spec)
	assert.NoError(t, err, "failed to unmarshal replicaset spec")
	assert.Equal(t, translatorName, spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	found := false
	for _, c := range spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestJSONTranslatorTranslateVirtualService(t *testing.T) {
	input := map[string]interface{}{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]interface{}{
			"name":   "vs-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"hosts": []string{"original.example.com"},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, &fakeDivertDriver{})

	var meta metav1.ObjectMeta
	err := json.Unmarshal(out["metadata"], &meta)
	assert.NoError(t, err, "failed to unmarshal metadata")
	assert.Equal(t, translatorName, meta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	var vs istioNetworkingV1beta1.VirtualService
	err = json.Unmarshal(out["spec"], &vs)
	assert.NoError(t, err, "failed to unmarshal virtual service spec")
	expectedHosts := []string{"diverted.example.com"}
	assert.True(t, reflect.DeepEqual(vs.Hosts, expectedHosts), "expected virtual service hosts to be %v, got %v", expectedHosts, vs.Hosts)
}

func TestJSONTranslatorTranslateVirtualServiceWithoutDivertDriver(t *testing.T) {
	// When no divert driver is provided, the virtual service should remain unchanged.
	input := map[string]interface{}{
		"apiVersion": "networking.istio.io/v1beta1",
		"kind":       "VirtualService",
		"metadata": map[string]interface{}{
			"name":   "vs-test",
			"labels": map[string]string{},
		},
		"spec": map[string]interface{}{
			"hosts": []string{"original.example.com"},
		},
	}
	translatorName := "test-deployer"
	out := runTranslatorTest(t, input, translatorName, nil)

	var vs istioNetworkingV1beta1.VirtualService
	err := json.Unmarshal(out["spec"], &vs)
	assert.NoError(t, err, "failed to unmarshal virtual service spec")
	expectedHosts := []string{"original.example.com"}
	assert.True(t, reflect.DeepEqual(vs.Hosts, expectedHosts), "expected virtual service hosts to remain unchanged as %v, got %v", expectedHosts, vs.Hosts)
}

func TestJSONTranslatorMissingMetadata(t *testing.T) {
	// Create an input JSON without the "metadata" field.
	input := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Pod",
		// "metadata" is intentionally missing
	}
	b, err := json.Marshal(input)
	assert.NoError(t, err, "failed to marshal input without metadata")
	translator := NewJSONTranslator(b, "test-deployer", nil)
	_, err = translator.Translate()
	assert.Error(t, err, "expected an error when metadata is missing")
	assert.Equal(t, "request body doesn't have metadata field", err.Error(), "unexpected error message")
}
