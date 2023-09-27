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

package deployments

import (
	"context"
	"fmt"
	"testing"

	oktetoErrors "github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGet(t *testing.T) {
	tests := []struct {
		name               string
		deployments        *appsv1.DeploymentList
		dev                *model.Dev
		namespace          string
		expectedErr        error
		expectedFoundCount int
	}{
		{
			name: "Get by Name: no deployments",
			dev: &model.Dev{
				Name: "fake",
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{},
			},
			expectedErr:        fmt.Errorf("deployments.apps \"%s\" not found", "fake"),
			expectedFoundCount: 0,
		},
		{
			name: "Get by Name: found 1 deployment",
			dev: &model.Dev{
				Name: "fake",
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "fake",
							Namespace: "test",
						},
					},
				},
			},
			expectedErr:        nil,
			expectedFoundCount: 1,
		},
		{
			name: "Search by Label: no deployments found",
			dev: &model.Dev{
				Selector: map[string]string{
					"deployed-by": "fake",
				},
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{},
			},
			expectedErr:        oktetoErrors.ErrNotFound,
			expectedFoundCount: 0,
		},
		{
			name: "Search by Label: cloned deployments are filtered out successfully",
			dev: &model.Dev{
				Selector: map[string]string{
					"deployed-by": "fake",
				},
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "fake1-clone",
							Namespace: "test",
							Labels: map[string]string{
								model.DevCloneLabel: "id-123",
								"deployed-by":       "fake",
							},
						},
					},
				},
			},
			expectedErr:        oktetoErrors.ErrNotFound,
			expectedFoundCount: 0,
		},
		{
			name: "Search by Label: no matching deployments found",
			dev: &model.Dev{
				Selector: map[string]string{
					"deployed-by": "fake",
				},
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-fake",
							Namespace: "test",
						},
					},
				},
			},
			expectedErr:        oktetoErrors.ErrNotFound,
			expectedFoundCount: 0,
		},
		{
			name: "Search by Label: 1 deployment found",
			dev: &model.Dev{
				Selector: map[string]string{
					"deployed-by": "fake",
				},
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "another-fake",
							Namespace: "test",
							Labels: map[string]string{
								"deployed-by": "fake",
							},
						},
					},
				},
			},
			expectedErr:        nil,
			expectedFoundCount: 1,
		},
		{
			name: "Search by Label: Unexpectedly found 2 deployments",
			dev: &model.Dev{
				Selector: map[string]string{
					"deployed-by": "fake",
				},
			},
			namespace: "test",
			deployments: &appsv1.DeploymentList{
				Items: []appsv1.Deployment{
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "fake1",
							Namespace: "test",
							Labels: map[string]string{
								"deployed-by": "fake",
							},
						},
					},
					{
						ObjectMeta: metav1.ObjectMeta{
							Name:      "fake2",
							Namespace: "test",
							Labels: map[string]string{
								"deployed-by": "fake",
							},
						},
					},
				},
			},
			expectedErr:        fmt.Errorf("found '%d' deployments for labels '%s' instead of 1", 2, "deployed-by=fake"),
			expectedFoundCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			clientset := fake.NewSimpleClientset(tt.deployments)
			d, err := GetByDev(ctx, tt.dev, tt.namespace, clientset)

			if err == nil && tt.expectedErr != nil {
				t.Fatalf("wrong error. Got nil, expected %s", tt.expectedErr.Error())
			}
			if err != nil && tt.expectedErr == nil {
				t.Fatalf("wrong error. Got nil, expected %s", err.Error())
			}
			if err != nil && tt.expectedErr != nil && err.Error() != tt.expectedErr.Error() {
				t.Fatalf("wrong error. Got %s, expected %s", err, tt.expectedErr)
			}
			if err == nil && d == nil {
				t.Fatal("deployment is nil found but no errors were returned")
			}
			if tt.expectedFoundCount > 0 && d == nil {
				t.Fatalf("expected %d deployments, instead found none", tt.expectedFoundCount)
			}
			if tt.expectedFoundCount == 0 && d != nil {
				t.Fatal("expected no deployments, instead found one")
			}
		})
	}
}

func TestCheckConditionErrors(t *testing.T) {
	tests := []struct {
		name        string
		deployment  *appsv1.Deployment
		dev         *model.Dev
		expectedErr error
	}{
		{
			"Wrong quota",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "exceeded quota",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			oktetoErrors.ErrQuota,
		},
		{
			"Memory per pod exceeded",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "maximum memory usage per Pod is 3Gi, but limit is 4294967296",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			fmt.Errorf("The value of resources.limits.memory in your okteto manifest (5Gi) exceeds the maximum memory limit per pod (3Gi)."),
		},
		{
			"Memory per pod exceeded 2",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "maximum memory usage",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			fmt.Errorf("The value of resources.limits.memory in your okteto manifest (5Gi) exceeds the maximum memory limit per pod (3Gi)."),
		},
		{
			"Cpu per pod exceeded",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "maximum cpu usage",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			fmt.Errorf("The value of resources.limits.cpu in your okteto manifest (2) exceeds the maximum CPU limit per pod (1)."),
		},
		{
			"Cpu per pod exceeded",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "maximum cpu usage per Pod is 1, but limit is 2,",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			fmt.Errorf("The value of resources.limits.cpu in your okteto manifest (2) exceeds the maximum CPU limit per pod (1)."),
		},
		{
			"Cpu and memory per pod exceeded",
			&appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "maximum cpu usage per Pod is 1, but limit is 2, maximum memory usage per Pod is 3Gi, but limit is 4294967296",
						},
					},
				},
			},
			&model.Dev{
				Resources: model.ResourceRequirements{
					Limits: model.ResourceList{
						apiv1.ResourceCPU:    resource.MustParse("2"),
						apiv1.ResourceMemory: resource.MustParse("5Gi"),
					},
				},
			},
			fmt.Errorf("The value of resources.limits.cpu in your okteto manifest (2) exceeds the maximum CPU limit per pod (1). The value of resources.limits.memory in your okteto manifest (5Gi) exceeds the maximum memory limit per pod (3Gi)."),
		},
		{
			name: "exceeded storage quota",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "exceeded quota: requested: requests.storage=1Gi, used: requests.storage=2Gi, limited: requests.storage=3Gi",
						},
					},
				},
			},
			dev:         &model.Dev{},
			expectedErr: fmt.Errorf("quota exceeded, you have reached the maximum storage per namespace"),
		},
		{
			name: "exceeded number of pods quota",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "exceeded quota: requested: pods=1, used: pods=2, limited: pods=3",
						},
					},
				},
			},
			dev:         &model.Dev{},
			expectedErr: fmt.Errorf("quota exceeded, you have reached the maximum number of pods per namespace"),
		},
		{
			name: "another error",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
				Status: appsv1.DeploymentStatus{
					Conditions: []appsv1.DeploymentCondition{
						{
							Type:    appsv1.DeploymentReplicaFailure,
							Reason:  "FailedCreate",
							Status:  apiv1.ConditionTrue,
							Message: "another error",
						},
					},
				},
			},
			dev:         &model.Dev{},
			expectedErr: fmt.Errorf("another error"),
		},
		{
			name: "No errors",
			deployment: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "fake",
					Namespace: "test",
				},
			},
			dev:         &model.Dev{},
			expectedErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CheckConditionErrors(tt.deployment, tt.dev)

			if err == nil && tt.expectedErr != nil {
				t.Fatalf("wrong error. Got nil, expected %s", tt.expectedErr.Error())
			}
			if err != nil && tt.expectedErr == nil {
				t.Fatalf("wrong error. Expected nil, but got %s", err.Error())
			}
		})
	}
}
