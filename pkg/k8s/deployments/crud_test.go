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

package deployments

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGet(t *testing.T) {
	ctx := context.Background()
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	dev := &model.Dev{Name: "fake"}

	clientset := fake.NewSimpleClientset(deployment)
	s, err := Get(ctx, dev, deployment.GetNamespace(), clientset)
	if err != nil {
		t.Fatal(err)
	}

	if s == nil {
		t.Fatal("empty service")
	}

	if s.Name != deployment.GetName() {
		t.Fatalf("wrong service. Got %s, expected %s", s.Name, deployment.GetName())
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
			errors.ErrQuota,
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			err := checkConditionErrors(tt.deployment, tt.dev)

			if err == nil {
				t.Fatalf("Didn't receive any error. Expected %s", tt.expectedErr)
			}

			if err.Error() != tt.expectedErr.Error() {
				t.Fatalf("wrong error. Got %s, expected %s", err, tt.expectedErr)
			}
		})
	}

}
