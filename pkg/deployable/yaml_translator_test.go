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
	"testing"

	"github.com/okteto/okteto/pkg/divert"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"sigs.k8s.io/yaml"
)

// runYAMLTranslatorTest is a helper that marshals the input YAML, runs the translator,
// and returns the resulting YAML unmarshaled into the target type.
func runYAMLTranslatorTest(t *testing.T, inputYAML string, translatorName string, dDriver divert.Driver) []byte {
	translator := newYAMLTranslator(translatorName, dDriver)
	outBytes, err := translator.Translate([]byte(inputYAML))
	assert.NoError(t, err, "Translate returned error")
	return outBytes
}

func TestYAMLTranslatorTranslateDeployment(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment-test
  labels:
    original: label
spec:
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: test-container
        image: nginx`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var deployment appsv1.Deployment
	err := yaml.Unmarshal(outBytes, &deployment)
	assert.NoError(t, err, "failed to unmarshal deployment")

	// Verify top-level metadata.
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify Deployment spec template metadata.
	assert.Equal(t, translatorName, deployment.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")

	// Verify that the divert driver was applied (container "diverted" added).
	found := false
	for _, c := range deployment.Spec.Template.Spec.Containers {
		if c.Name == "diverted" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected diverted container to be added by divert driver")
}

func TestYAMLTranslatorTranslateStatefulSet(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: statefulset-test
spec:
  template:
    metadata:
      labels:
        app: statefulset
    spec:
      containers:
      - name: test-container
        image: nginx`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var sts appsv1.StatefulSet
	err := yaml.Unmarshal(outBytes, &sts)
	assert.NoError(t, err, "failed to unmarshal statefulset")

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

func TestYAMLTranslatorTranslateJob(t *testing.T) {
	inputYAML := `apiVersion: batch/v1
kind: Job
metadata:
  name: job-test
spec:
  template:
    metadata:
      labels:
        job: "true"
    spec:
      containers:
      - name: test-container
        image: busybox`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var job batchv1.Job
	err := yaml.Unmarshal(outBytes, &job)
	assert.NoError(t, err, "failed to unmarshal job")

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

func TestYAMLTranslatorTranslateCronJob(t *testing.T) {
	inputYAML := `apiVersion: batch/v1
kind: CronJob
metadata:
  name: cronjob-test
spec:
  schedule: "*/5 * * * *"
  jobTemplate:
    spec:
      template:
        metadata:
          labels:
            cron: job
        spec:
          containers:
          - name: test-container
            image: busybox`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var cronJob batchv1.CronJob
	err := yaml.Unmarshal(outBytes, &cronJob)
	assert.NoError(t, err, "failed to unmarshal cronjob")

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

func TestYAMLTranslatorTranslateDaemonSet(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: daemonset-test
spec:
  template:
    metadata:
      labels:
        daemon: "true"
    spec:
      containers:
      - name: test-container
        image: nginx`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var daemonSet appsv1.DaemonSet
	err := yaml.Unmarshal(outBytes, &daemonSet)
	assert.NoError(t, err, "failed to unmarshal daemonset")

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

func TestYAMLTranslatorTranslateGenericResource(t *testing.T) {
	inputYAML := `apiVersion: v1
kind: Service
metadata:
  name: service-test
spec:
  selector:
    app: myapp
  ports:
  - port: 80
    targetPort: 8080`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, nil)

	var result map[string]interface{}
	err := yaml.Unmarshal(outBytes, &result)
	assert.NoError(t, err, "failed to unmarshal generic resource")

	metadata := result["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, translatorName, labels[model.DeployedByLabel], "expected metadata label to be set for generic resource")
}

func TestYAMLTranslatorWithNilDivertDriver(t *testing.T) {
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment-test
spec:
  template:
    metadata:
      labels:
        app: myapp
    spec:
      containers:
      - name: test-container
        image: nginx`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, nil)

	var deployment appsv1.Deployment
	err := yaml.Unmarshal(outBytes, &deployment)
	assert.NoError(t, err, "failed to unmarshal deployment")

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

func TestYAMLTranslatorInvalidYAML(t *testing.T) {
	invalidYAML := `this is not valid yaml: [[[`
	translator := newYAMLTranslator("test-deployer", nil)
	outBytes, err := translator.Translate([]byte(invalidYAML))
	assert.NoError(t, err, "Translate should not return error for invalid YAML, just return nil")
	assert.Nil(t, outBytes, "expected nil output for invalid YAML")
}

func TestYAMLTranslatorDeploymentWithoutMetadata(t *testing.T) {
	// Test that we handle resources without complete metadata properly
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: deployment-test
spec:
  template:
    spec:
      containers:
      - name: test-container
        image: nginx`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var deployment appsv1.Deployment
	err := yaml.Unmarshal(outBytes, &deployment)
	assert.NoError(t, err, "failed to unmarshal deployment")

	// Verify top-level metadata label is set
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify template metadata label is set even though it wasn't in the original YAML
	assert.Equal(t, translatorName, deployment.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel], "expected template metadata label to be set")
}

func TestYAMLTranslatorVirtualService(t *testing.T) {
	inputYAML := `apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: vs-test
spec:
  hosts:
  - original.example.com`

	translatorName := "test-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, &fakeDivertDriver{})

	var result map[string]interface{}
	err := yaml.Unmarshal(outBytes, &result)
	assert.NoError(t, err, "failed to unmarshal virtual service")

	// Verify metadata label is set
	metadata := result["metadata"].(map[string]interface{})
	labels := metadata["labels"].(map[string]interface{})
	assert.Equal(t, translatorName, labels[model.DeployedByLabel], "expected metadata label to be set")

	// Verify spec was modified by divert driver
	spec := result["spec"].(map[string]interface{})
	hosts := spec["hosts"].([]interface{})
	assert.Equal(t, "diverted.example.com", hosts[0], "expected hosts to be modified by divert driver")
}

func TestYAMLTranslatorServerSideApplyPatchScenario(t *testing.T) {
	// Test a scenario that mimics server-side apply, where we might have partial resources
	inputYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deployment
  namespace: default
spec:
  replicas: 3
  template:
    metadata:
      labels:
        version: v1
    spec:
      containers:
      - name: app
        image: myapp:latest
        ports:
        - containerPort: 8080`

	translatorName := "ssa-deployer"
	outBytes := runYAMLTranslatorTest(t, inputYAML, translatorName, nil)

	var deployment appsv1.Deployment
	err := yaml.Unmarshal(outBytes, &deployment)
	assert.NoError(t, err, "failed to unmarshal deployment")

	// Verify the deployed-by label was injected
	assert.Equal(t, translatorName, deployment.ObjectMeta.Labels[model.DeployedByLabel])
	assert.Equal(t, translatorName, deployment.Spec.Template.ObjectMeta.Labels[model.DeployedByLabel])

	// Verify original fields are preserved
	assert.Equal(t, "my-deployment", deployment.ObjectMeta.Name)
	assert.Equal(t, "default", deployment.ObjectMeta.Namespace)
	assert.Equal(t, int32(3), *deployment.Spec.Replicas)
	assert.Equal(t, "v1", deployment.Spec.Template.ObjectMeta.Labels["version"])
	assert.Equal(t, "app", deployment.Spec.Template.Spec.Containers[0].Name)
	assert.Equal(t, "myapp:latest", deployment.Spec.Template.Spec.Containers[0].Image)
}
