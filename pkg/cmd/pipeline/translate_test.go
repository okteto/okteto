// Copyright 2023 The Okteto Authors
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

package pipeline

import (
	"context"
	"encoding/base64"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/stretchr/testify/assert"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_translateConfigMap(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TranslatePipelineName("test"),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Data: map[string]string{
			statusField: DeployedStatus,
		},
	}
	fakeClient := fake.NewSimpleClientset(cmap)
	var tests = []struct {
		name    string
		status  string
		appName string
	}{
		{
			name:    "existing cmap",
			status:  DeployedStatus,
			appName: "test",
		},
		{
			name:    "existing cmap overwrite status",
			status:  ErrorStatus,
			appName: "test",
		},
		{
			name:    "not found cmap",
			status:  ProgressingStatus,
			appName: "not-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := &CfgData{
				Name:      tt.appName,
				Namespace: namespace,
				Status:    tt.status,
			}
			cfg, err := TranslateConfigMapAndDeploy(ctx, data, fakeClient)

			assert.Nil(t, err)
			assert.Equal(t, cfg.Data[statusField], tt.status)

			assert.NotEmpty(t, cfg.Annotations[constants.LastUpdatedAnnotation])
		})
	}
}

func Test_updateEnvsWithoutError(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TranslatePipelineName("test"),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Data: map[string]string{
			statusField: DeployedStatus,
		},
	}
	fakeClient := fake.NewSimpleClientset(cmap)
	envs := []string{
		"ONE=value",
		"TWO=values",
		"URL=https://okteto.com?okteto=rocks",
	}

	err := UpdateEnvs(ctx, "test", namespace, envs, fakeClient)
	assert.NoError(t, err)
}

func Test_updateEnvsWithError(t *testing.T) {
	ctx := context.Background()
	namespace := "test"
	cmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TranslatePipelineName("test"),
			Namespace: namespace,
			Labels:    map[string]string{},
		},
		Data: map[string]string{
			statusField: DeployedStatus,
		},
	}
	fakeClient := fake.NewSimpleClientset(cmap)
	var tests = []struct {
		name    string
		appName string
		envs    []string
	}{
		{
			name:    "invalid env in configmap",
			appName: "test",
			envs: []string{
				"ONE INVALID ENV",
			},
		},
		{
			name:    "not found cmap",
			appName: "not-test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := UpdateEnvs(ctx, tt.appName, namespace, tt.envs, fakeClient)
			assert.Error(t, err)
		})
	}
}

func Test_AddDevAnnotations(t *testing.T) {
	ctx := context.Background()
	okteto.CurrentStore = &okteto.ContextStore{
		Contexts: map[string]*okteto.Context{
			"example": {
				Namespace: "unit-test",
			},
		},
		CurrentContext: "example",
	}
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "deployment",
			Namespace:   "unit-test",
			Labels:      map[string]string{},
			Annotations: map[string]string{},
		},
	}
	fakeClient := fake.NewSimpleClientset(d)
	t.Setenv(model.GithubRepositoryEnvVar, "git-repo")
	manifest := &model.Manifest{
		Dev: model.ManifestDevs{
			"not-found-deployment": &model.Dev{
				Name: "not-found-deployment",
			},
			"deployment": &model.Dev{
				Name: "deployment",
			},
			"autocreate": &model.Dev{
				Autocreate: true,
			},
		},
	}
	AddDevAnnotations(ctx, manifest, fakeClient)
	d, err := fakeClient.AppsV1().Deployments("unit-test").Get(ctx, "deployment", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t,
		d.Annotations,
		map[string]string{
			model.OktetoRepositoryAnnotation: "file://git-repo",
			model.OktetoDevNameAnnotation:    "deployment",
		},
	)
}

func Test_removeSensitiveDataFromGitURL(t *testing.T) {
	tests := []struct {
		name     string
		gitURL   string
		expected string
	}{
		{
			name:     "empty-url",
			gitURL:   "",
			expected: "",
		},
		{
			name:     "without-user-information",
			gitURL:   "https://github.com/okteto/movies",
			expected: "https://github.com/okteto/movies",
		},
		{
			name:     "with-user-and-password",
			gitURL:   "https://my-user:my-strong-pass@github.com:80/okteto/movies",
			expected: "https://github.com:80/okteto/movies",
		},
		{
			name:     "with-auth-token-long",
			gitURL:   "https://adsoifq9389qnjasd:x-oauth-basic@github.com/okteto/movies",
			expected: "https://github.com/okteto/movies",
		},
		{
			name:     "with-auth-token-short",
			gitURL:   "https://adsoifq9389qnjasd@github.com/okteto/movies",
			expected: "https://github.com/okteto/movies",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeSensitiveDataFromGitURL(tt.gitURL)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_translateVariables(t *testing.T) {
	tests := []struct {
		name     string
		expected string
		input    []string
	}{
		{
			name:     "nil input",
			expected: "",
		},
		{
			name:     "empty input",
			input:    []string{},
			expected: "",
		},
		{
			name:     "invalid input",
			input:    []string{"test"},
			expected: "",
		},
		{
			name:     "valid input",
			input:    []string{"test=value"},
			expected: base64.StdEncoding.EncodeToString([]byte("[{\"name\":\"test\",\"value\":\"value\"}]")),
		},
	}

	for _, tt := range tests {

		t.Run(tt.name, func(t *testing.T) {
			res := translateVariables(tt.input)
			assert.Equal(t, tt.expected, res)
		})

	}
}
func Test_AddPhaseDuration(t *testing.T) {
	ctx := context.Background()
	name := "test"
	namespace := "test-namespace"
	phase := "phase1"
	duration := time.Second * 10
	c := fake.NewSimpleClientset()

	// Create a config map with existing phases
	existingPhases := []phaseJSON{
		{
			Name:     "phase1",
			Duration: 5,
		},
		{
			Name:     "phase2",
			Duration: 8,
		},
	}
	existingCmap := &apiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      TranslatePipelineName(name),
			Namespace: namespace,
		},
		Data: map[string]string{
			PhasesField: encodePhases(existingPhases),
		},
	}
	_, err := c.CoreV1().ConfigMaps(namespace).Create(ctx, existingCmap, metav1.CreateOptions{})
	assert.NoError(t, err)

	err = AddPhaseDuration(ctx, name, namespace, phase, duration, c)
	assert.NoError(t, err)

	// Verify that the phase duration is updated
	updatedCmap, err := c.CoreV1().ConfigMaps(namespace).Get(ctx, TranslatePipelineName(name), metav1.GetOptions{})
	assert.NoError(t, err)
	updatedPhases := decodePhases(updatedCmap.Data[PhasesField])
	assert.Equal(t, len(existingPhases), len(updatedPhases))
	for _, p := range updatedPhases {
		if p.Name == phase {
			assert.Equal(t, duration.Seconds(), p.Duration)
		}
	}

	// Verify that a new phase is added if it doesn't exist
	newPhase := "new-phase"
	newDuration := time.Second * 15
	err = AddPhaseDuration(ctx, name, namespace, newPhase, newDuration, c)
	assert.NoError(t, err)

	updatedCmap, err = c.CoreV1().ConfigMaps(namespace).Get(ctx, TranslatePipelineName(name), metav1.GetOptions{})
	assert.NoError(t, err)
	updatedPhases = decodePhases(updatedCmap.Data[PhasesField])
	assert.Equal(t, len(existingPhases)+1, len(updatedPhases))
	found := false
	for _, p := range updatedPhases {
		if p.Name == newPhase {
			assert.Equal(t, newDuration.Seconds(), p.Duration)
			found = true
			break
		}
	}
	assert.True(t, found)
}

func encodePhases(phases []phaseJSON) string {
	encodedPhases, _ := json.Marshal(phases)
	return string(encodedPhases)
}

func decodePhases(encodedPhases string) []phaseJSON {
	var phases []phaseJSON
	_ = json.Unmarshal([]byte(encodedPhases), &phases)
	return phases
}
