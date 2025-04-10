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

package istio

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
)

func Test_updateEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []apiv1.EnvVar
		envName  string
		envValue string
		expected []apiv1.EnvVar
	}{
		{
			name:     "empty-env-vars",
			envVars:  []apiv1.EnvVar{},
			envName:  "TEST_VAR",
			envValue: "test-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "TEST_VAR",
					Value: "test-value",
				},
			},
		},
		{
			name: "update-existing-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "old-value",
				},
			},
			envName:  "TEST_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "new-value",
				},
			},
		},
		{
			name: "add-new-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
			},
			envName:  "NEW_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "NEW_VAR",
					Value: "new-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := tt.envVars
			updateEnvVar(&envVars, tt.envName, tt.envValue)
			assert.Equal(t, tt.expected, envVars)
		})
	}
}

func Test_UpdatePod(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  apiv1.PodSpec
		expected apiv1.PodSpec
		driver   *Driver
	}{
		{
			name: "empty-pod",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-existing-env-vars",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "old-value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-multiple-containers",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.driver.UpdatePod(tt.podSpec)
			assert.Equal(t, tt.expected, result)
		})
	}
}