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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/build"
	"github.com/okteto/okteto/pkg/divert"
	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/format"
	"github.com/okteto/okteto/pkg/k8s/services"
	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	apiv1 "k8s.io/api/core/v1"
	k8sErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestMain(m *testing.M) {
	okteto.CurrentStore = &okteto.ContextStore{
		CurrentContext: "test",
		Contexts: map[string]*okteto.Context{
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
						RestartPolicy: apiv1.RestartPolicyAlways,
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
						RestartPolicy: apiv1.RestartPolicyAlways,
						Volumes: []build.VolumeMounts{
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
						RestartPolicy: apiv1.RestartPolicyNever,
					},
				},
			},
		},
	}

	divertDriver := divert.NewNoop()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := deploySvc(ctx, tt.stack, tt.svcName, client, divertDriver)
			require.NoError(t, err)
		})
	}
}

func Test_reDeploySvc_deployment(t *testing.T) {
	ctx := context.Background()
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
	fakeClient := fake.NewSimpleClientset(oldDep)
	stack := &model.Stack{
		Namespace: "test",
		Name:      "testName",
		Services: map[string]*model.Service{
			"serviceName": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyAlways,
			},
		},
	}

	err := deploySvc(ctx, stack, "serviceName", fakeClient, divert.NewNoop())
	require.NoError(t, err)

	d, err := fakeClient.AppsV1().Deployments("test").Get(ctx, "serviceName", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, format.ResourceK8sMetaString("testName"), d.Labels[model.StackNameLabel])
	require.Equal(t, format.ResourceK8sMetaString("testName"), d.Labels[model.DeployedByLabel])
}

func Test_reDeploySvc_sfs(t *testing.T) {
	ctx := context.Background()
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
	fakeClient := fake.NewSimpleClientset(oldSfs)
	stack := &model.Stack{
		Namespace: "test",
		Name:      "testName",
		Services: map[string]*model.Service{
			"serviceName": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyAlways,
				Volumes: []build.VolumeMounts{
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

	err := deploySvc(ctx, stack, "serviceName", fakeClient, divert.NewNoop())
	require.NoError(t, err)

	sfs, err := fakeClient.AppsV1().StatefulSets("test").Get(ctx, "serviceName", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, format.ResourceK8sMetaString("testName"), sfs.Labels[model.StackNameLabel])
	require.Equal(t, format.ResourceK8sMetaString("testName"), sfs.Labels[model.DeployedByLabel])
}

func Test_reDeploySvc_job(t *testing.T) {
	ctx := context.Background()
	oldJob := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
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
	fakeClient := fake.NewSimpleClientset(oldJob)
	stack := &model.Stack{
		Namespace: "test",
		Name:      "testName",
		Services: map[string]*model.Service{
			"serviceName": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyNever,
			},
		},
	}

	err := deploySvc(ctx, stack, "serviceName", fakeClient, divert.NewNoop())
	require.NoError(t, err)

	job, err := fakeClient.BatchV1().Jobs("test").Get(ctx, "serviceName", metav1.GetOptions{})
	require.NoError(t, err)
	require.Equal(t, format.ResourceK8sMetaString("testName"), job.Labels[model.StackNameLabel])
}

func Test_deployDeployment(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyAlways,
			},
		},
	}
	client := fake.NewSimpleClientset()

	divertDriver := divert.NewNoop()
	_, err := deployDeployment(ctx, "test", stack, client, divertDriver)
	require.NoError(t, err)

	_, err = client.AppsV1().Deployments("ns").Get(ctx, "test", metav1.GetOptions{})
	require.NoError(t, err)
}

func Test_deployVolumes(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyAlways,
				Volumes: []build.VolumeMounts{
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
	require.NoError(t, err)

	_, err = client.CoreV1().PersistentVolumeClaims("ns").Get(ctx, "a", metav1.GetOptions{})
	require.NoError(t, err)
}

func Test_deploySfs(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyAlways,
				Volumes: []build.VolumeMounts{
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

	divertDriver := divert.NewNoop()
	_, err := deployStatefulSet(ctx, "test", stack, client, divertDriver)
	require.NoError(t, err)

	_, err = client.AppsV1().StatefulSets("ns").Get(ctx, "test", metav1.GetOptions{})
	require.NoError(t, err)
}

func Test_deployJob(t *testing.T) {
	ctx := context.Background()
	stack := &model.Stack{
		Namespace: "ns",
		Name:      "stack-test",
		Services: map[string]*model.Service{
			"test": {
				Image:         "test_image",
				RestartPolicy: apiv1.RestartPolicyNever,
			},
		},
	}
	client := fake.NewSimpleClientset()

	divertDriver := divert.NewNoop()
	_, err := deployJob(ctx, "test", stack, client, divertDriver)
	require.NoError(t, err)

	_, err = client.BatchV1().Jobs("ns").Get(ctx, "test", metav1.GetOptions{})
	require.NoError(t, err)
}

func TestValidateDefinedServices_undefinedService(t *testing.T) {
	stack := &model.Stack{
		Services: map[string]*model.Service{
			"db": {},
			"api": {DependsOn: model.DependsOn{
				"db": model.DependsOnConditionSpec{},
			}},
		},
	}
	err := ValidateDefinedServices(stack, []string{"nginx", "db"})
	require.Error(t, err)
}

func TestValidateDefinedServices_ok(t *testing.T) {
	stack := &model.Stack{
		Services: map[string]*model.Service{
			"db": {},
			"api": {DependsOn: model.DependsOn{
				"db": model.DependsOnConditionSpec{},
			}},
		},
	}
	err := ValidateDefinedServices(stack, []string{"api", "db"})
	require.NoError(t, err)
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
						RestartPolicy: apiv1.RestartPolicyNever,
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
						Volumes: []build.VolumeMounts{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
						RestartPolicy: apiv1.RestartPolicyAlways,
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
						RestartPolicy: apiv1.RestartPolicyAlways,
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
						RestartPolicy: apiv1.RestartPolicyNever,
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
						RestartPolicy: apiv1.RestartPolicyNever,
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
						RestartPolicy: apiv1.RestartPolicyNever,
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
						Volumes: []build.VolumeMounts{
							{
								LocalPath:  "/",
								RemotePath: "/",
							},
						},
						RestartPolicy: apiv1.RestartPolicyAlways,
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
						RestartPolicy: apiv1.RestartPolicyAlways,
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
			options := &DeployOptions{ServicesToDeploy: tt.svcsToBeDeployed}
			options.ServicesToDeploy = AddDependentServicesIfNotPresent(ctx, tt.stack, options.ServicesToDeploy, fakeClient)
			require.ElementsMatch(t, tt.expectedSvcsToBeDeployed, options.ServicesToDeploy)
		})
	}
}

func Test_getVolumesToDeployFromServicesToDeploy(t *testing.T) {
	type args struct {
		stack            *model.Stack
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		args     args
		expected []string
		name     string
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
							Volumes: []build.VolumeMounts{
								{
									LocalPath: "volume a",
								},
								{
									LocalPath: "volume b",
								},
							},
						},
						"service b": {
							Volumes: []build.VolumeMounts{
								{
									LocalPath: "volume b",
								},
							},
						},
						"service bc": {
							Volumes: []build.VolumeMounts{
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
			expected: []string{"volume b", "volume c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getVolumesToDeployFromServicesToDeploy(tt.args.stack, tt.args.servicesToDeploy)
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func Test_getEndpointsToDeployFromServicesToDeploy(t *testing.T) {
	type args struct {
		endpoints        model.EndpointSpec
		servicesToDeploy map[string]bool
	}
	tests := []struct {
		args     args
		expected []string
		name     string
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
			require.ElementsMatch(t, tt.expected, result)
		})
	}
}

func TestDeployK8sService(t *testing.T) {
	tests := []struct {
		stack             *model.Stack
		name              string
		expectedNameLabel string
		k8sObjects        []runtime.Object
	}{
		{
			name: "skip service",
			k8sObjects: []runtime.Object{
				&apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
						Labels: map[string]string{
							model.StackNameLabel: "hola",
						},
					},
				},
			},
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "test",
				Services: map[string]*model.Service{
					"test": {
						Labels: map[string]string{
							"ey": "a",
						},
					},
				},
			},
			expectedNameLabel: "hola",
		},
		{
			name: "update service",
			k8sObjects: []runtime.Object{
				&apiv1.Service{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "ns",
						Labels: map[string]string{
							model.StackNameLabel: "test",
						},
					},
				},
			},
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "test",
				Services: map[string]*model.Service{
					"test": {
						Labels: map[string]string{
							"ey": "a",
						},
					},
				},
			},
			expectedNameLabel: "test",
		},
		{
			name:       "create new service",
			k8sObjects: []runtime.Object{},
			stack: &model.Stack{
				Namespace: "ns",
				Name:      "test",
				Services: map[string]*model.Service{
					"test": {
						Labels: map[string]string{
							"ey": "a",
						},
					},
				},
			},
			expectedNameLabel: "test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.k8sObjects...)
			err := deployK8sService(context.Background(), "test", tt.stack, fakeClient)
			require.NoError(t, err)
			svc, err := services.Get(context.Background(), "test", "ns", fakeClient)
			require.NoError(t, err)
			require.Equal(t, tt.expectedNameLabel, svc.ObjectMeta.Labels[model.StackNameLabel])
		})
	}
}

func TestGetErrorDueToRestartLimit(t *testing.T) {
	tests := []struct {
		err                              error
		stack                            *model.Stack
		previousDeployedServicesRestarts map[string]int
		name                             string
		k8sObjects                       []runtime.Object
	}{
		{
			name: "no dependent services",
			stack: &model.Stack{
				Services: map[string]*model.Service{
					"test2": {
						DependsOn: model.DependsOn{},
					},
				},
			},
			previousDeployedServicesRestarts: map[string]int{},
			err:                              nil,
		},
		{
			name: "dependent svc without reaching backoff",
			k8sObjects: []runtime.Object{
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							model.StackNameLabel:        "test",
							model.StackServiceNameLabel: "test1",
						},
					},
					Status: apiv1.PodStatus{
						ContainerStatuses: []apiv1.ContainerStatus{
							{
								RestartCount: 1,
							},
						},
					},
				},
			},
			stack: &model.Stack{
				Name: "test",
				Services: map[string]*model.Service{
					"test1": {
						BackOffLimit: 2,
						DependsOn:    model.DependsOn{},
					},
					"test2": {
						DependsOn: model.DependsOn{
							"test1": model.DependsOnConditionSpec{
								Condition: model.DependsOnServiceHealthy,
							},
						},
					},
				},
			},
			previousDeployedServicesRestarts: map[string]int{},
			err:                              nil,
		},
		{
			name: "dependent svc reaching backoff",
			k8sObjects: []runtime.Object{
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							model.StackNameLabel:        "test",
							model.StackServiceNameLabel: "test1",
						},
					},
					Status: apiv1.PodStatus{
						ContainerStatuses: []apiv1.ContainerStatus{
							{
								RestartCount: 5,
							},
						},
					},
				},
			},
			stack: &model.Stack{
				Name: "test",
				Services: map[string]*model.Service{
					"test1": {
						BackOffLimit: 2,
						DependsOn:    model.DependsOn{},
					},
					"test2": {
						DependsOn: model.DependsOn{
							"test1": model.DependsOnConditionSpec{
								Condition: model.DependsOnServiceHealthy,
							},
						},
					},
				},
			},
			previousDeployedServicesRestarts: map[string]int{
				"test1": 0,
			},
			err: fmt.Errorf("service 'test1' has been restarted 5 times within this deploy. Please check the logs and try again"),
		},
		{
			name: "dependent svc reaching backoff with previous deploy with failures",
			k8sObjects: []runtime.Object{
				&apiv1.Pod{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							model.StackNameLabel:        "test",
							model.StackServiceNameLabel: "test1",
						},
					},
					Status: apiv1.PodStatus{
						ContainerStatuses: []apiv1.ContainerStatus{
							{
								RestartCount: 8,
							},
						},
					},
				},
			},
			stack: &model.Stack{
				Name: "test",
				Services: map[string]*model.Service{
					"test1": {
						BackOffLimit: 2,
						DependsOn:    model.DependsOn{},
					},
					"test2": {
						DependsOn: model.DependsOn{
							"test1": model.DependsOnConditionSpec{
								Condition: model.DependsOnServiceHealthy,
							},
						},
					},
				},
			},
			previousDeployedServicesRestarts: map[string]int{
				"test1": 5,
			},
			err: fmt.Errorf("service 'test1' has been restarted 3 times within this deploy. Please check the logs and try again"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.k8sObjects...)
			err := getErrorDueToRestartLimit(context.Background(), tt.stack, "test2", tt.previousDeployedServicesRestarts, fakeClient)
			require.Equal(t, tt.err, err)
		})
	}
}

func TestShouldUseHTTPRoute(t *testing.T) {
	originalStore := okteto.CurrentStore
	defer func() { okteto.CurrentStore = originalStore }()

	tests := []struct {
		name               string
		envVar             string
		defaultGatewayType string
		gateway            *okteto.GatewayMetadata
		expectedUseRoute   bool
		expectedMetadata   types.ClusterMetadata
	}{
		{
			name:             "env var forces ingress",
			envVar:           "ingress",
			gateway:          &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute: false,
			expectedMetadata: types.ClusterMetadata{},
		},
		{
			name:             "env var forces gateway",
			envVar:           "gateway",
			gateway:          &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute: true,
			expectedMetadata: types.ClusterMetadata{GatewayName: "test-gateway", GatewayNamespace: "gateway-ns"},
		},
		{
			name:             "gateway configured in context",
			gateway:          &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute: true,
			expectedMetadata: types.ClusterMetadata{GatewayName: "test-gateway", GatewayNamespace: "gateway-ns"},
		},
		{
			name:             "no gateway configured",
			gateway:          nil,
			expectedUseRoute: false,
			expectedMetadata: types.ClusterMetadata{},
		},
		{
			name:             "gateway without namespace",
			gateway:          &okteto.GatewayMetadata{Name: "test-gateway"},
			expectedUseRoute: false,
			expectedMetadata: types.ClusterMetadata{},
		},
		{
			name:               "default gateway type forces ingress",
			defaultGatewayType: "ingress",
			gateway:            &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute:   false,
			expectedMetadata:   types.ClusterMetadata{},
		},
		{
			name:               "default gateway type forces gateway with metadata",
			defaultGatewayType: "gateway",
			gateway:            &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute:   true,
			expectedMetadata:   types.ClusterMetadata{GatewayName: "test-gateway", GatewayNamespace: "gateway-ns"},
		},
		{
			name:               "feature flag takes precedence over default gateway type",
			envVar:             "ingress",
			defaultGatewayType: "gateway",
			gateway:            &okteto.GatewayMetadata{Name: "test-gateway", Namespace: "gateway-ns"},
			expectedUseRoute:   false,
			expectedMetadata:   types.ClusterMetadata{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {
						Name:      "test",
						Namespace: "namespace",
						UserID:    "user-id",
						Gateway:   tt.gateway,
					},
				},
			}
			t.Setenv(oktetoComposeEndpointsTypeEnvVar, tt.envVar)
			t.Setenv(oktetoDefaultGatewayTypeEnvVar, tt.defaultGatewayType)

			useRoute, metadata, err := ShouldUseHTTPRoute()
			require.NoError(t, err)
			require.Equal(t, tt.expectedUseRoute, useRoute)
			require.Equal(t, tt.expectedMetadata.GatewayName, metadata.GatewayName)
			require.Equal(t, tt.expectedMetadata.GatewayNamespace, metadata.GatewayNamespace)
		})
	}
}

func TestShouldUseHTTPRouteError(t *testing.T) {
	originalStore := okteto.CurrentStore
	defer func() { okteto.CurrentStore = originalStore }()

	tests := []struct {
		name               string
		envVar             string
		defaultGatewayType string
	}{
		{
			name:               "default gateway type forces gateway without metadata",
			defaultGatewayType: "gateway",
		},
		{
			name:   "env var forces gateway without metadata",
			envVar: "gateway",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			okteto.CurrentStore = &okteto.ContextStore{
				CurrentContext: "test",
				Contexts: map[string]*okteto.Context{
					"test": {
						Name:      "test",
						Namespace: "namespace",
						UserID:    "user-id",
						Gateway:   nil,
					},
				},
			}
			t.Setenv(oktetoComposeEndpointsTypeEnvVar, tt.envVar)
			t.Setenv(oktetoDefaultGatewayTypeEnvVar, tt.defaultGatewayType)

			_, _, err := ShouldUseHTTPRoute()
			require.Error(t, err)
		})
	}
}

// newServiceAccount returns a ServiceAccount in the "ns" namespace used by the stacks in these tests
func newServiceAccount(name string) *apiv1.ServiceAccount {
	return &apiv1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}}
}

func countServiceAccountGets(client *fake.Clientset) int {
	count := 0
	for _, action := range client.Actions() {
		if action.GetVerb() == "get" && action.GetResource().Resource == "serviceaccounts" {
			count++
		}
	}
	return count
}

// createServiceAccountWhenDeploymentIsCreated simulates a ServiceAccount created by another service of the same deploy
func createServiceAccountWhenDeploymentIsCreated(t *testing.T, client *fake.Clientset, deploymentName string, sa *apiv1.ServiceAccount) {
	t.Helper()
	client.PrependReactor("create", "deployments", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		created, ok := createAction.GetObject().(*appsv1.Deployment)
		if ok && created.Name == deploymentName {
			require.NoError(t, client.Tracker().Add(sa))
		}
		return false, nil, nil
	})
}

// markJobSucceededAndCreateServiceAccount simulates a compose job that creates a ServiceAccount and completes
func markJobSucceededAndCreateServiceAccount(t *testing.T, client *fake.Clientset, jobName string, sa *apiv1.ServiceAccount) {
	t.Helper()
	client.PrependReactor("create", "jobs", func(action k8stesting.Action) (bool, runtime.Object, error) {
		createAction, ok := action.(k8stesting.CreateAction)
		if !ok {
			return false, nil, nil
		}
		created, ok := createAction.GetObject().(*batchv1.Job)
		if ok && created.Name == jobName {
			created.Status.Succeeded = *created.Spec.Completions
			require.NoError(t, client.Tracker().Add(sa))
		}
		return false, nil, nil
	})
}

// deployServicesWithin runs deployServices and fails the test if it doesn't return in time: deployServices doesn't
// honor options.Timeout while waiting on depends_on, so a regression would otherwise hang the test
func deployServicesWithin(t *testing.T, timeout time.Duration, s *model.Stack, client *fake.Clientset, options *DeployOptions) error {
	t.Helper()
	done := make(chan error, 1)
	go func() { done <- deployServices(context.Background(), s, client, nil, options, nil) }()
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		require.FailNow(t, "deployServices didn't return in time")
		return nil
	}
}

func newStackWithServiceAccount(serviceAccount string) *model.Stack {
	return &model.Stack{
		Name:      "stack",
		Namespace: "ns",
		Services: map[string]*model.Service{
			"api": {Image: "okteto/api", Replicas: 1, ServiceAccount: serviceAccount},
		},
	}
}

func TestCheckServiceAccountExists_Exists(t *testing.T) {
	client := fake.NewSimpleClientset(newServiceAccount("api-sa"))

	err := checkServiceAccountExists(context.Background(), newStackWithServiceAccount("api-sa"), "api", client, map[string]bool{})

	require.NoError(t, err)
}

func TestCheckServiceAccountExists_MissingReturnsUserErrorWithHint(t *testing.T) {
	client := fake.NewSimpleClientset()

	err := checkServiceAccountExists(context.Background(), newStackWithServiceAccount("api-sa"), "api", client, map[string]bool{})

	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.EqualError(t, userErr.E, "ServiceAccount 'api-sa' used by service 'api' doesn't exist in namespace 'ns'")
	require.Equal(t, "Create it (for example in your deploy commands), or remove 'x-okteto-service-account' to use the namespace's default ServiceAccount", userErr.Hint)
}

func TestCheckServiceAccountExists_NoServiceAccountNoAPICalls(t *testing.T) {
	client := fake.NewSimpleClientset()

	err := checkServiceAccountExists(context.Background(), newStackWithServiceAccount(""), "api", client, map[string]bool{})

	require.NoError(t, err)
	require.Empty(t, client.Actions())
}

func TestCheckServiceAccountExists_SharedAccountCheckedOnce(t *testing.T) {
	s := &model.Stack{
		Namespace: "ns",
		Services: map[string]*model.Service{
			"api":    {ServiceAccount: "shared-sa"},
			"worker": {ServiceAccount: "shared-sa"},
		},
	}
	client := fake.NewSimpleClientset(newServiceAccount("shared-sa"))
	checked := map[string]bool{}

	require.NoError(t, checkServiceAccountExists(context.Background(), s, "api", client, checked))
	require.NoError(t, checkServiceAccountExists(context.Background(), s, "worker", client, checked))
	require.Equal(t, 1, countServiceAccountGets(client))
}

func TestCheckServiceAccountExists_ForbiddenWarnsContinuesAndIsNotRetried(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "serviceaccounts", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8sErrors.NewForbidden(schema.GroupResource{Resource: "serviceaccounts"}, "api-sa", fmt.Errorf("RBAC: access denied"))
	})
	s := newStackWithServiceAccount("api-sa")
	checked := map[string]bool{}

	require.NoError(t, checkServiceAccountExists(context.Background(), s, "api", client, checked))
	require.NoError(t, checkServiceAccountExists(context.Background(), s, "api", client, checked))
	require.Equal(t, 1, countServiceAccountGets(client))
}

func TestCheckServiceAccountExists_UnverifiableErrorsWarnAndContinue(t *testing.T) {
	tests := []struct {
		err  error
		name string
	}{
		{name: "forbidden", err: k8sErrors.NewForbidden(schema.GroupResource{Resource: "serviceaccounts"}, "api-sa", fmt.Errorf("RBAC: access denied"))},
		{name: "unauthorized", err: k8sErrors.NewUnauthorized("token expired")},
		{name: "internal error", err: k8sErrors.NewInternalError(fmt.Errorf("etcd unavailable"))},
		{name: "plain error", err: fmt.Errorf("connection reset by peer")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			client.PrependReactor("get", "serviceaccounts", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.err
			})

			err := checkServiceAccountExists(context.Background(), newStackWithServiceAccount("api-sa"), "api", client, map[string]bool{})

			require.NoError(t, err)
			require.Equal(t, 1, countServiceAccountGets(client))
		})
	}
}

// TestCheckServiceAccountExists_ForbiddenWithNotFoundInNameIsNotMissing guards against matching "not found" in the
// error text: a forbidden error for a ServiceAccount whose name contains "not-found" must not fail the deploy
func TestCheckServiceAccountExists_ForbiddenWithNotFoundInNameIsNotMissing(t *testing.T) {
	client := fake.NewSimpleClientset()
	client.PrependReactor("get", "serviceaccounts", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, k8sErrors.NewForbidden(schema.GroupResource{Resource: "serviceaccounts"}, "not-found-sa", fmt.Errorf("RBAC: access denied"))
	})

	err := checkServiceAccountExists(context.Background(), newStackWithServiceAccount("not-found-sa"), "api", client, map[string]bool{})

	require.NoError(t, err)
}

func TestDeployServices_MissingServiceAccountFailsBeforeDeployingTheService(t *testing.T) {
	client := fake.NewSimpleClientset()
	options := &DeployOptions{ServicesToDeploy: []string{"api"}, Timeout: time.Minute}

	err := deployServices(context.Background(), newStackWithServiceAccount("api-sa"), client, nil, options, nil)

	var userErr oktetoErrors.UserError
	require.ErrorAs(t, err, &userErr)
	require.ErrorContains(t, err, "ServiceAccount 'api-sa' used by service 'api' doesn't exist in namespace 'ns'")
	_, err = client.AppsV1().Deployments("ns").Get(context.Background(), "api", metav1.GetOptions{})
	require.True(t, k8sErrors.IsNotFound(err))
}

// TestDeployServices_ServiceAccountCreatedEarlierInTheDeployIsFound covers a ServiceAccount that doesn't exist when
// the compose stage starts but is created by a service deployed before the one using it
func TestDeployServices_ServiceAccountCreatedEarlierInTheDeployIsFound(t *testing.T) {
	s := &model.Stack{
		Name:      "stack",
		Namespace: "ns",
		Services: map[string]*model.Service{
			"creator": {Image: "okteto/creator", Replicas: 1},
			"api":     {Image: "okteto/api", Replicas: 1, ServiceAccount: "api-sa"},
		},
	}
	client := fake.NewSimpleClientset()
	createServiceAccountWhenDeploymentIsCreated(t, client, "creator", newServiceAccount("api-sa"))
	options := &DeployOptions{ServicesToDeploy: []string{"creator", "api"}, Timeout: time.Minute}

	err := deployServices(context.Background(), s, client, nil, options, nil)

	require.NoError(t, err)
	_, err = client.AppsV1().Deployments("ns").Get(context.Background(), "api", metav1.GetOptions{})
	require.NoError(t, err)
}

// TestDeployServices_ServiceAccountCreatedByDependencyJobIsFound covers a compose job that creates the ServiceAccount,
// with the service using it declaring depends_on service_completed_successfully on that job: the check must wait for
// the dependency instead of failing up front. The service is listed first to show the order doesn't matter
func TestDeployServices_ServiceAccountCreatedByDependencyJobIsFound(t *testing.T) {
	s := &model.Stack{
		Name:      "stack",
		Namespace: "ns",
		Services: map[string]*model.Service{
			"init": {Image: "bitnami/kubectl", Replicas: 1, RestartPolicy: apiv1.RestartPolicyNever},
			"api": {
				Image:          "okteto/api",
				Replicas:       1,
				ServiceAccount: "api-sa",
				DependsOn:      model.DependsOn{"init": model.DependsOnConditionSpec{Condition: model.DependsOnServiceCompleted}},
			},
		},
	}
	client := fake.NewSimpleClientset()
	markJobSucceededAndCreateServiceAccount(t, client, "init", newServiceAccount("api-sa"))
	options := &DeployOptions{ServicesToDeploy: []string{"api", "init"}, Timeout: time.Minute}

	err := deployServicesWithin(t, 10*time.Second, s, client, options)

	require.NoError(t, err)
	_, err = client.AppsV1().Deployments("ns").Get(context.Background(), "api", metav1.GetOptions{})
	require.NoError(t, err)
}
