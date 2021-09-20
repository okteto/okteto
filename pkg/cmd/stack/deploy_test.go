// Copyright 2021 The Okteto Authors
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
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_deploySvc(t *testing.T) {
	ctx := context.Background()
	client := fake.NewSimpleClientset()
	var tests = []struct {
		name    string
		stack   *model.Stack
		svcName string
	}{
		{
			name:    "deploy deployment",
			svcName: "test",
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
		},
		{
			name:    "deploy sfs",
			svcName: "test",
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "stack-test",
				Services: map[string]*model.Service{
					"test": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
						Volumes: []model.StackVolume{
							{
								LocalPath:  "a",
								RemotePath: "b",
							},
						},
					},
				},
				Volumes: map[string]*model.VolumeSpec{
					"a": {},
				},
			},
		},
		{
			name:    "deploy job",
			svcName: "test",
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
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spinner := utils.NewSpinner("testing")
			err := deploySvc(ctx, tt.stack, tt.svcName, client, spinner)
			if err != nil {
				t.Fatal("Not deployed correctly")
			}
		})
	}
}

func Test_deployDeployment(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: corev1.RestartPolicyAlways,
			},
		},
	}
	client := fake.NewSimpleClientset()

	err := deployDeployment(ctx, "test", stack, client)
	if err != nil {
		t.Fatal("Not deployed correctly")
	}

	_, err = client.AppsV1().Deployments("ns").Get(ctx, "test", metav1.GetOptions{})
	if err != nil {
		t.Fatal("Not deployed correctly")
	}
}

func Test_deployVolumes(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: corev1.RestartPolicyAlways,
				Volumes: []model.StackVolume{
					{
						LocalPath:  "a",
						RemotePath: "b",
					},
				},
			},
		},
		Volumes: map[string]*model.VolumeSpec{
			"a": {},
		},
	}
	client := fake.NewSimpleClientset()

	err := deployVolume(ctx, "a", stack, client)
	if err != nil {
		t.Fatal("Not deployed correctly")
	}

	_, err = client.CoreV1().PersistentVolumeClaims("ns").Get(ctx, "a", metav1.GetOptions{})
	if err != nil {
		t.Fatal("Not deployed correctly")
	}
}

func Test_deploySfs(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: corev1.RestartPolicyAlways,
				Volumes: []model.StackVolume{
					{
						LocalPath:  "a",
						RemotePath: "b",
					},
				},
			},
		},
		Volumes: map[string]*model.VolumeSpec{
			"a": {},
		},
	}
	client := fake.NewSimpleClientset()

	err := deployStatefulSet(ctx, "test", stack, client)
	if err != nil {
		t.Fatal("Not deployed correctly")
	}

	_, err = client.AppsV1().StatefulSets("ns").Get(ctx, "test", metav1.GetOptions{})
	if err != nil {
		t.Fatal("Not deployed correctly")
	}
}

func Test_deployJob(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: corev1.RestartPolicyNever,
			},
		},
	}
	client := fake.NewSimpleClientset()

	err := deployJob(ctx, "test", stack, client)
	if err != nil {
		t.Fatal("Not deployed correctly")
	}

	_, err = client.BatchV1().Jobs("ns").Get(ctx, "test", metav1.GetOptions{})
	if err != nil {
		t.Fatal("Not deployed correctly")
	}
}

func Test_ValidateDeploySomeServices(t *testing.T) {
	var tests = []struct {
		name             string
		stack            *model.Stack
		svcsToBeDeployed []string
		expectedErr      bool
	}{
		{
			name: "not defined svc",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"db": {},
					"api": {DependsOn: model.DependsOn{
						"db": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed: []string{"nginx", "db"},
			expectedErr:      true,
		},
		{
			name: "ok",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"db": {},
					"api": {DependsOn: model.DependsOn{
						"db": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed: []string{"api", "db"},
			expectedErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDefinedServices(tt.stack, tt.svcsToBeDeployed)
			if err == nil && tt.expectedErr {
				t.Fatal("Expected err but not thrown")
			}
			if err != nil && !tt.expectedErr {
				t.Fatal("Not Expected err but not thrown")
			}
		})
	}
}

func Test_AddSomeServices(t *testing.T) {
	ctx := context.Background()
	jobActive := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "job-active",
		Namespace: "default",
	},
		Status: batchv1.JobStatus{
			Active: 1,
		},
	}
	jobFailed := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "job-failed",
		Namespace: "default",
	},
		Status: batchv1.JobStatus{
			Failed: 1,
		},
	}
	jobSucceded := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "job-succeded",
		Namespace: "default",
	},
		Status: batchv1.JobStatus{
			Succeeded: 1,
		},
	}
	sfs := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name:      "sfs",
		Namespace: "default",
	},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
	}
	dep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "dep",
		Namespace: "default",
	},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	fakeClient := fake.NewSimpleClientset(jobActive, jobSucceded, jobFailed, sfs, dep)

	var tests = []struct {
		name                     string
		stack                    *model.Stack
		svcsToBeDeployed         []string
		expectedSvcsToBeDeployed []string
	}{
		{
			name: "dependant service is job and not running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"job-not-running": {
						RestartPolicy: corev1.RestartPolicyNever,
					},
					"app": {DependsOn: model.DependsOn{
						"job-not-running": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app", "job-not-running"},
		},
		{
			name: "dependant service is sfs and not running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"sfs-not-running": {
						Volumes: []model.StackVolume{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
						RestartPolicy: corev1.RestartPolicyAlways,
					},
					"app": {DependsOn: model.DependsOn{
						"sfs-not-running": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app", "sfs-not-running"},
		},
		{
			name: "dependant service is deployment and not running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"dep-not-running": {
						RestartPolicy: corev1.RestartPolicyAlways,
					},
					"app": {DependsOn: model.DependsOn{
						"dep-not-running": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app", "dep-not-running"},
		},
		{
			name: "dependant service is job and running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"job-active": {
						RestartPolicy: corev1.RestartPolicyNever,
					},
					"app": {DependsOn: model.DependsOn{
						"job-active": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app"},
		},
		{
			name: "dependant service is job finished with errors",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"job-failed": {
						RestartPolicy: corev1.RestartPolicyNever,
					},
					"app": {DependsOn: model.DependsOn{
						"job-failed": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app", "job-failed"},
		},
		{
			name: "dependant service is job finished successful",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"job-succeded": {
						RestartPolicy: corev1.RestartPolicyNever,
					},
					"app": {DependsOn: model.DependsOn{
						"job-succeded": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app"},
		},
		{
			name: "dependant service is sfs and running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"sfs": {
						Volumes: []model.StackVolume{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
						RestartPolicy: corev1.RestartPolicyAlways,
					},
					"app": {DependsOn: model.DependsOn{
						"sfs": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app"},
		},
		{
			name: "dependant service is deployment and running",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"dep": {
						RestartPolicy: corev1.RestartPolicyAlways,
					},
					"app": {DependsOn: model.DependsOn{
						"dep": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app"},
		},
		{
			name: "dependant service is already on to be deployed",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"db": {},
					"api": {DependsOn: model.DependsOn{
						"db": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"api", "db"},
			expectedSvcsToBeDeployed: []string{"api", "db"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := &StackDeployOptions{ServicesToDeploy: tt.svcsToBeDeployed}
			addDependentServicesIfNotPresent(ctx, tt.stack, options, fakeClient)

			if !reflect.DeepEqual(tt.expectedSvcsToBeDeployed, options.ServicesToDeploy) {
				t.Errorf("Expected %v but got %v", tt.expectedSvcsToBeDeployed, options.ServicesToDeploy)
			}
		})
	}
}
