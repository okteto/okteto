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

package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
	"github.com/okteto/okteto/pkg/env"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_generateManifestFile(t *testing.T) {
	var tests = []struct {
		dev  *model.Dev
		name string
	}{
		{
			name: "empty",
			dev:  nil,
		},
		{
			name: "basic",
			dev: &model.Dev{
				Name:    "dev",
				Image:   "okteto/dev",
				Command: model.Command{Values: []string{"bash"}},
			},
		},
		{
			name: "with-services",
			dev: &model.Dev{
				Name:    "dev",
				Image:   "okteto/dev",
				Command: model.Command{Values: []string{"bash"}},
				Services: []*model.Dev{{
					Name:    "svc",
					Image:   "okteto/svc",
					Command: model.Command{Values: []string{"bash"}},
				}, {
					Name:        "svc2",
					Image:       "",
					Command:     model.Command{Values: []string{"bash"}},
					Environment: []env.Var{{Name: "foo", Value: "bar"}},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			file, err := os.CreateTemp("", "okteto.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(file.Name())

			out, err := yaml.Marshal(tt.dev)
			if err != nil {
				t.Fatal(err)
			}

			if _, err = file.Write(out); err != nil {
				t.Fatal("Failed to write to temporary file", err)
			}

			_, err = generateManifestFile(file.Name())
			if err != nil {
				t.Fatal(err)
			}
		})

	}

}

func Test_generatePodFile(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		dev     *model.Dev
		objects []runtime.Object
	}{
		{
			name: "success - autocreate with running pod",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: true,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
						UID: types.UID("deploy-uid-123"),
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
						Template: apiv1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test",
								},
							},
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:  "dev",
										Image: "okteto/test:latest",
									},
								},
							},
						},
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-abc123",
						Namespace: "test",
						UID:       types.UID("rs-uid-123"),
						Labels: map[string]string{
							"app": "test",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-okteto",
								UID:        types.UID("deploy-uid-123"),
							},
						},
					},
					Spec: appsv1.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
				},
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-abc123-xyz",
						Namespace: "test",
						Labels: map[string]string{
							"app": "test",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "ReplicaSet",
								Name:       "test-okteto-abc123",
								UID:        types.UID("rs-uid-123"),
							},
						},
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  "dev",
								Image: "okteto/test:latest",
							},
						},
					},
					Status: apiv1.PodStatus{
						Phase: apiv1.PodRunning,
						Conditions: []apiv1.PodCondition{
							{
								Type:   apiv1.PodReady,
								Status: apiv1.ConditionTrue,
							},
						},
					},
				},
			},
		},
		{
			name: "success - regular deployment in dev mode with running pod",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: false,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						UID:       types.UID("original-uid"),
						Labels: map[string]string{
							model.DevCloneLabel: "clone-uid-456",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
				},
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						UID:       types.UID("clone-uid-456"),
						Labels: map[string]string{
							model.DevCloneLabel: "clone-uid-456",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
						Template: apiv1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{
									"app": "test",
								},
							},
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:  "dev",
										Image: "okteto/test:latest",
									},
								},
							},
						},
					},
					Status: appsv1.DeploymentStatus{
						ObservedGeneration: 1,
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-xyz789",
						Namespace: "test",
						UID:       types.UID("rs-uid-456"),
						Labels: map[string]string{
							"app": "test",
						},
						Annotations: map[string]string{
							model.DeploymentRevisionAnnotation: "1",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "Deployment",
								Name:       "test-okteto",
								UID:        types.UID("clone-uid-456"),
							},
						},
					},
					Spec: appsv1.ReplicaSetSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
				},
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-xyz789-pod",
						Namespace: "test",
						Labels: map[string]string{
							"app": "test",
						},
						OwnerReferences: []metav1.OwnerReference{
							{
								APIVersion: "apps/v1",
								Kind:       "ReplicaSet",
								Name:       "test-okteto-xyz789",
								UID:        types.UID("rs-uid-456"),
							},
						},
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name:  "dev",
								Image: "okteto/test:latest",
							},
						},
					},
					Status: apiv1.PodStatus{
						Phase: apiv1.PodRunning,
						Conditions: []apiv1.PodCondition{
							{
								Type:   apiv1.PodReady,
								Status: apiv1.ConditionTrue,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			result, err := generatePodFile(ctx, tt.dev, "test", clientset)

			require.NoError(t, err)
			require.NotEmpty(t, result)
			require.FileExists(t, result)

			// Verify the file contains pod information
			content, readErr := os.ReadFile(result)
			require.NoError(t, readErr)
			require.NotEmpty(t, content)

			// Clean up
			os.RemoveAll(result)
		})
	}
}

func Test_generatePodFileError(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		dev           *model.Dev
		objects       []runtime.Object
		errorContains string
	}{
		{
			name: "autocreate deployment exists but no pod",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: true,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
						Template: apiv1.PodTemplateSpec{
							Spec: apiv1.PodSpec{
								Containers: []apiv1.Container{
									{
										Name:  "dev",
										Image: "okteto/test:latest",
									},
								},
							},
						},
					},
				},
			},
			errorContains: "not found",
		},
		{
			name: "regular deployment in dev mode",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: false,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels: map[string]string{
							model.DevCloneLabel: "original-uid",
						},
					},
				},
			},
			errorContains: "not found",
		},
		{
			name: "regular deployment not in dev mode",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: false,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels:    map[string]string{},
					},
				},
			},
			errorContains: "not in development mode",
		},
		{
			name: "deployment not found",
			dev: &model.Dev{
				Name:       "nonexistent",
				Autocreate: false,
				Container:  "dev",
			},
			objects:       []runtime.Object{},
			errorContains: "doesn't exist",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			result, err := generatePodFile(ctx, tt.dev, "test", clientset)

			require.Error(t, err)
			require.Contains(t, err.Error(), tt.errorContains)
			require.Empty(t, result)
		})
	}
}

func Test_generateSyncthingLogsFolder(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name    string
		dev     *model.Dev
		objects []runtime.Object
	}{
		{
			name: "autocreate with running pod",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: true,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto",
						Namespace: "test",
						Labels: map[string]string{
							constants.DevLabel: "true",
						},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app": "test",
							},
						},
					},
				},
				&appsv1.ReplicaSet{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-abc",
						Namespace: "test",
						UID:       "rs-uid",
					},
				},
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-okteto-abc-pod",
						Namespace: "test",
						OwnerReferences: []metav1.OwnerReference{
							{
								UID: "rs-uid",
							},
						},
					},
					Spec: apiv1.PodSpec{
						Containers: []apiv1.Container{
							{
								Name: "dev",
							},
						},
					},
					Status: apiv1.PodStatus{
						Phase: apiv1.PodRunning,
					},
				},
			},
		},
		{
			name: "regular deployment not in dev mode",
			dev: &model.Dev{
				Name:       "test",
				Autocreate: false,
				Container:  "dev",
			},
			objects: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
						Labels:    map[string]string{},
					},
				},
			},
		},
		{
			name: "deployment not found",
			dev: &model.Dev{
				Name:       "nonexistent",
				Autocreate: false,
				Container:  "dev",
			},
			objects: []runtime.Object{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			clientset := fake.NewSimpleClientset(tt.objects...)

			result, err := generateSyncthingLogsFolder(ctx, tt.dev, "test", clientset)

			// The function should not return error, it handles failures gracefully
			require.NoError(t, err)
			require.NotEmpty(t, result)
			// Clean up temp directory
			defer os.RemoveAll(filepath.Dir(result))
		})
	}
}
