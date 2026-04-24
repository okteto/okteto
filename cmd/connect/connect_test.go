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

package connect

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/env"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestSplitEnvFlag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:     "key equals value",
			input:    "FOO=bar",
			expected: []string{"FOO", "bar"},
		},
		{
			name:     "value with equals sign",
			input:    "FOO=bar=baz",
			expected: []string{"FOO", "bar=baz"},
		},
		{
			name:     "empty value",
			input:    "FOO=",
			expected: []string{"FOO", ""},
		},
		{
			name:     "missing key and value",
			input:    "NOTSET_ENV_THAT_DOESNT_EXIST",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitEnvFlag(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestApplyEnvOverrides(t *testing.T) {
	tests := []struct {
		name     string
		initial  env.Environment
		flags    []string
		expected env.Environment
		wantErr  bool
	}{
		{
			name:     "no flags",
			initial:  env.Environment{{Name: "A", Value: "1"}},
			flags:    nil,
			expected: env.Environment{{Name: "A", Value: "1"}},
		},
		{
			name:    "add new var",
			initial: env.Environment{},
			flags:   []string{"B=2"},
			expected: env.Environment{
				{Name: "B", Value: "2"},
			},
		},
		{
			name:    "override existing",
			initial: env.Environment{{Name: "A", Value: "old"}},
			flags:   []string{"A=new"},
			expected: env.Environment{
				{Name: "A", Value: "new"},
			},
		},
		{
			name:    "invalid flag",
			initial: env.Environment{},
			flags:   []string{"NO_EQUALS_NO_ENV_MATCH"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			devEnv := tt.initial
			err := applyEnvOverrides(&devEnv, tt.flags)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			// Compare as maps since order is non-deterministic.
			got := make(map[string]string)
			for _, e := range devEnv {
				got[e.Name] = e.Value
			}
			want := make(map[string]string)
			for _, e := range tt.expected {
				want[e.Name] = e.Value
			}
			assert.Equal(t, want, got)
		})
	}
}

func TestInferDevFromDeployment_Deployment(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "test-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:       "api",
							Image:      "myimage:latest",
							WorkingDir: "/app",
						},
					},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(deployment)
	opts := &Options{}

	manifest, err := inferDevFromDeployment(context.Background(), "api", "test-ns", opts, k8sClient)

	require.NoError(t, err)
	require.NotNil(t, manifest)
	assert.Equal(t, "api", manifest.Name)

	dev, ok := manifest.Dev["api"]
	require.True(t, ok)
	assert.Equal(t, "api", dev.Name)
	assert.Equal(t, "myimage:latest", dev.Image)
	require.Len(t, dev.Sync.Folders, 1)
	assert.Equal(t, "/app", dev.Sync.Folders[0].RemotePath)
}

func TestInferDevFromDeployment_ImageOverride(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "test-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:       "api",
							Image:      "original:image",
							WorkingDir: "/app",
						},
					},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(deployment)
	opts := &Options{Image: "custom:dev"}

	manifest, err := inferDevFromDeployment(context.Background(), "api", "test-ns", opts, k8sClient)

	require.NoError(t, err)
	dev := manifest.Dev["api"]
	assert.Equal(t, "custom:dev", dev.Image)
}

func TestInferDevFromDeployment_EmptyWorkdirDefaultsToRoot(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "test-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "api",
							Image: "myimage:latest",
							// WorkingDir intentionally empty
						},
					},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(deployment)
	opts := &Options{}

	manifest, err := inferDevFromDeployment(context.Background(), "api", "test-ns", opts, k8sClient)
	require.NoError(t, err)
	assert.Equal(t, "/app", manifest.Dev["api"].Sync.Folders[0].RemotePath)
}

func TestInferDevFromDeployment_NotFound(t *testing.T) {
	k8sClient := fake.NewSimpleClientset()
	opts := &Options{}

	_, err := inferDevFromDeployment(context.Background(), "api", "test-ns", opts, k8sClient)
	require.Error(t, err)
}

func TestGetContainerInfo_NoContainers(t *testing.T) {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "api",
			Namespace: "test-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{},
				},
			},
		},
	}

	k8sClient := fake.NewSimpleClientset(deployment)
	opts := &Options{}

	_, err := inferDevFromDeployment(context.Background(), "api", "test-ns", opts, k8sClient)
	require.Error(t, err)
}
