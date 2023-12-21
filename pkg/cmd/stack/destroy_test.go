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

package stack

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/k8s/deployments"
	"github.com/okteto/okteto/pkg/k8s/jobs"
	"github.com/okteto/okteto/pkg/k8s/statefulsets"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_destroyDeployments(t *testing.T) {
	ctx := context.Background()

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Labels:    map[string]string{model.StackNameLabel: "stack-test"},
		},
	}

	client := fake.NewSimpleClientset(dep)
	var tests = []struct {
		stack               *model.Stack
		name                string
		expectedDeployments int
	}{
		{
			name: "not destroy anything",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
			expectedDeployments: 1,
		},
		{
			name: "destroy dep not in stack",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test-2": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
			expectedDeployments: 0,
		},
		{
			name: "destroy dep which is not deployment anymore",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
			expectedDeployments: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := destroyDeployments(ctx, tt.stack, client)
			if err != nil {
				t.Fatal("Not destroyed correctly")
			}
			depList, err := deployments.List(ctx, "ns", tt.stack.GetLabelSelector(), client)
			if err != nil {
				t.Fatal("could not retrieve list correctly")
			}
			if len(depList) != tt.expectedDeployments {
				t.Fatal("Not destroyed correctly")
			}
		})
	}
}

func Test_destroyStatefulsets(t *testing.T) {
	ctx := context.Background()

	sfs := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Labels:    map[string]string{model.StackNameLabel: "stack-test"},
		},
	}

	client := fake.NewSimpleClientset(sfs)
	var tests = []struct {
		stack               *model.Stack
		name                string
		expectedDeployments int
	}{
		{
			name: "not destroy anything",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
						Volumes: []build.VolumeMounts{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
					},
				},
			},
			expectedDeployments: 1,
		},
		{
			name: "destroy dep not in stack",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test-2": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
						Volumes: []build.VolumeMounts{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
					},
				},
			},
			expectedDeployments: 0,
		},
		{
			name: "destroy dep which is not deployment anymore",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
			expectedDeployments: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := destroyStatefulsets(ctx, tt.stack, client)
			if err != nil {
				t.Fatal("Not destroyed correctly")
			}
			sfsList, err := statefulsets.List(ctx, "ns", tt.stack.GetLabelSelector(), client)
			if err != nil {
				t.Fatal("could not retrieve list correctly")
			}
			if len(sfsList) != tt.expectedDeployments {
				t.Fatal("Not destroyed correctly")
			}
		})
	}
}

func Test_destroyJobs(t *testing.T) {
	ctx := context.Background()

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "ns",
			Labels:    map[string]string{model.StackNameLabel: "stack-test"},
		},
	}

	client := fake.NewSimpleClientset(job)
	var tests = []struct {
		stack               *model.Stack
		name                string
		expectedDeployments int
	}{
		{
			name: "not destroy anything",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
			expectedDeployments: 1,
		},
		{
			name: "destroy dep not in stack",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test-2": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyNever,
					},
				},
			},
			expectedDeployments: 0,
		},
		{
			name: "destroy dep which is not deployment anymore",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
			expectedDeployments: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := destroyJobs(ctx, tt.stack, client)
			if err != nil {
				t.Fatal("Not destroyed correctly")
			}
			jobsList, err := jobs.List(ctx, "ns", tt.stack.GetLabelSelector(), client)
			if err != nil {
				t.Fatal("could not retrieve list correctly")
			}
			if len(jobsList) != tt.expectedDeployments {
				t.Fatal("Not destroyed correctly")
			}
		})
	}
}
