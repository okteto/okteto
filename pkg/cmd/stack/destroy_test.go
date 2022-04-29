// Copyright 2022 The Okteto Authors
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
	"reflect"
	"testing"

	"github.com/okteto/okteto/cmd/utils"
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
		name                string
		stack               *model.Stack
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
			spinner := utils.NewSpinner("testing")
			err := destroyDeployments(ctx, spinner, tt.stack, client)
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
		name                string
		stack               *model.Stack
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
						Volumes: []model.StackVolume{
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
						Volumes: []model.StackVolume{
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
			spinner := utils.NewSpinner("testing")
			err := destroyStatefulsets(ctx, spinner, tt.stack, client)
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
		name                string
		stack               *model.Stack
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
			spinner := utils.NewSpinner("testing")
			err := destroyJobs(ctx, spinner, tt.stack, client)
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

func Test_onlyDeployVolumesFromServicesToDeploy(t *testing.T) {
	type args struct {
		stack            *model.Stack
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		name     string
		args     args
		expected map[string]*model.VolumeSpec
	}{
		{
			name: "multiple volumes",
			args: args{
				servicesToDeploy: map[string]bool{
					"service b":  true,
					"service bc": true,
				},
				stack: &model.Stack{
					Services: map[string]*model.Service{
						"service ab": {
							Volumes: []model.StackVolume{
								{
									LocalPath: "volume a",
								},
								{
									LocalPath: "volume b",
								},
							},
						},
						"service b": {
							Volumes: []model.StackVolume{
								{
									LocalPath: "volume b",
								},
							},
						},
						"service bc": {
							Volumes: []model.StackVolume{
								{
									LocalPath: "volume b",
								},
								{
									LocalPath: "volume c",
								},
							},
						},
					},
					Volumes: map[string]*model.VolumeSpec{
						"volume a": {},
						"volume b": {},
						"volume c": {},
					},
				},
			},
			expected: map[string]*model.VolumeSpec{
				"volume b": {},
				"volume c": {},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVolumesToDeployFromServicesToDeploy(tt.args.stack, tt.args.servicesToDeploy)
			if !reflect.DeepEqual(tt.args.stack.Volumes, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func Test_onlyDeployEndpointsFromServicesToDeploy(t *testing.T) {
	type args struct {
		endpoints        model.EndpointSpec
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		name     string
		args     args
		expected []string
	}{
		{
			name: "multiple endpoints",
			args: args{
				endpoints: model.EndpointSpec{
					"manifest": {
						Rules: []model.EndpointRule{
							{Service: "a"},
							{Service: "b"},
						},
					},
				},
				servicesToDeploy: map[string]bool{
					"a": true,
				},
			},
			expected: []string{"manifest"},
		},
		{
			name: "no endpoints",
			args: args{
				endpoints: model.EndpointSpec{},
				servicesToDeploy: map[string]bool{
					"manifest": true,
				},
			},
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEndpointsToDeployFromServicesToDeploy(tt.args.endpoints, tt.args.servicesToDeploy)
			if !reflect.DeepEqual(tt.args.endpoints, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
