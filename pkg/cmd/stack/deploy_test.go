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
	"os"
	"reflect"
	"testing"

	"github.com/okteto/okteto/cmd/utils"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.OktetoContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.OktetoContext{
			"test": {
				Name:      "test",
				Namespace: "namespace",
				UserID:    "user-id",
			},
		},
	}
	os.Exit(m.Run())
}

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

func Test_reDeploySvc(t *testing.T) {
	ctx := context.Background()
	oldJobSucceeded := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "serviceName",
		Namespace: "test",
		Labels: map[string]string{
			model.StackNameLabel:  "okteto",
			model.DeployedByLabel: "okteto",
		},
	},
		Status: batchv1.JobStatus{
			Succeeded: 1,
		},
	}
	oldSfs := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Name:      "serviceName",
		Namespace: "test",
		Labels: map[string]string{
			model.StackNameLabel:  "okteto",
			model.DeployedByLabel: "okteto",
		},
	},
		Status: appsv1.StatefulSetStatus{
			ReadyReplicas: 1,
		},
	}
	oldDep := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Name:      "serviceName",
		Namespace: "test",
		Labels: map[string]string{
			model.StackNameLabel:  "okteto",
			model.DeployedByLabel: "okteto",
		},
	},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}
	fakeClient := fake.NewSimpleClientset(oldJobSucceeded, oldSfs, oldDep)
	var tests = []struct {
		name      string
		component string
		stack     *model.Stack
		svcName   string
	}{
		{
			name:      "redeploy deployment",
			component: "deployment",
			svcName:   "serviceName",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "testName",
				Services: map[string]*model.Service{
					"serviceName": {
						Image:         "test_image",
						RestartPolicy: corev1.RestartPolicyAlways,
					},
				},
			},
		},
		{
			name:      "redeploy sfs",
			svcName:   "serviceName",
			component: "sfs",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "testName",
				Services: map[string]*model.Service{
					"serviceName": {
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
			name:      "redeploy job",
			svcName:   "serviceName",
			component: "job",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "testName",
				Services: map[string]*model.Service{
					"serviceName": {
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
			err := deploySvc(ctx, tt.stack, tt.svcName, fakeClient, spinner)
			if err != nil {
				t.Fatal("Not re-deployed correctly")
			}

			if tt.component == "deployment" {
				d, err := fakeClient.AppsV1().Deployments(tt.stack.Namespace).Get(ctx, tt.svcName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if d.Labels[model.StackNameLabel] != tt.stack.Name {
					t.Fatal()
				}
				if d.Labels[model.DeployedByLabel] != tt.stack.Name {
					t.Fatal()
				}
			}
			if tt.component == "sfs" {
				sfs, err := fakeClient.AppsV1().StatefulSets(tt.stack.Namespace).Get(ctx, tt.svcName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if sfs.Labels[model.StackNameLabel] != tt.stack.Name {
					t.Fatal()
				}
				if sfs.Labels[model.DeployedByLabel] != tt.stack.Name {
					t.Fatal()
				}

			}
			if tt.component == "job" {
				job, err := fakeClient.BatchV1().Jobs(tt.stack.Namespace).Get(ctx, tt.svcName, metav1.GetOptions{})
				if err != nil {
					t.Fatal(err)
				}
				if job.Labels[model.StackNameLabel] != tt.stack.Name {
					t.Fatal()
				}
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

	spinner := utils.NewSpinner("Starting...")
	spinner.Start()
	_, err := deployDeployment(ctx, "test", stack, client, spinner)
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

	spinner := utils.NewSpinner("Starting...")
	spinner.Start()
	err := deployVolume(ctx, "a", stack, client, spinner)
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

	spinner := utils.NewSpinner("Starting...")
	spinner.Start()
	_, err := deployStatefulSet(ctx, "test", stack, client, spinner)
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

	spinner := utils.NewSpinner("Starting...")
	spinner.Start()
	_, err := deployJob(ctx, "test", stack, client, spinner)
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
			err := ValidateDefinedServices(tt.stack, tt.svcsToBeDeployed)
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
	jobSucceeded := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:      "job-succeeded",
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
	fakeClient := fake.NewSimpleClientset(jobActive, jobSucceeded, jobFailed, sfs, dep)

	var tests = []struct {
		name                     string
		stack                    *model.Stack
		svcsToBeDeployed         []string
		expectedSvcsToBeDeployed []string
	}{
		{
			name: "dependent service is job and not running",
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
			name: "dependent service is sfs and not running",
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
			name: "dependent service is deployment and not running",
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
			name: "dependent service is job and running",
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
			name: "dependent service is job finished with errors",
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
			name: "dependent service is job finished successful",
			stack: &model.Stack{
				Namespace: "default",
				Services: map[string]*model.Service{
					"job-succeeded": {
						RestartPolicy: corev1.RestartPolicyNever,
					},
					"app": {DependsOn: model.DependsOn{
						"job-succeeded": model.DependsOnConditionSpec{},
					}},
				},
			},
			svcsToBeDeployed:         []string{"app"},
			expectedSvcsToBeDeployed: []string{"app"},
		},
		{
			name: "dependent service is sfs and running",
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
			name: "dependent service is deployment and running",
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
			name: "dependent service is already on to be deployed",
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
			options.ServicesToDeploy = AddDependentServicesIfNotPresent(ctx, tt.stack, options.ServicesToDeploy, fakeClient)

			if !reflect.DeepEqual(tt.expectedSvcsToBeDeployed, options.ServicesToDeploy) {
				t.Errorf("Expected %v but got %v", tt.expectedSvcsToBeDeployed, options.ServicesToDeploy)
			}
		})
	}
}

func Test_getVolumesToDeployFromServicesToDeploy(t *testing.T) {
	type args struct {
		stack            *model.Stack
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		name     string
		args     args
		expected map[string]bool
	}{
		{
			name: "should return volumes from services to deploy",
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
			expected: map[string]bool{"volume b": true, "volume c": true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVolumesToDeployFromServicesToDeploy(tt.args.stack, tt.args.servicesToDeploy)
			resultSet := make(map[string]bool, len(result))
			for _, v := range result {
				resultSet[v] = true
			}
			if !reflect.DeepEqual(resultSet, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func Test_getEndpointsToDeployFromServicesToDeploy(t *testing.T) {
	type args struct {
		endpoints        model.EndpointSpec
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		name     string
		args     args
		expected map[string]bool
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
			expected: map[string]bool{"manifest": true},
		},
		{
			name: "no endpoints",
			args: args{
				endpoints: model.EndpointSpec{},
				servicesToDeploy: map[string]bool{
					"manifest": true,
				},
			},
			expected: map[string]bool{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getEndpointsToDeployFromServicesToDeploy(tt.args.endpoints, tt.args.servicesToDeploy)
			resultSet := make(map[string]bool, len(result))
			for _, v := range result {
				resultSet[v] = true
			}
			if !reflect.DeepEqual(resultSet, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
