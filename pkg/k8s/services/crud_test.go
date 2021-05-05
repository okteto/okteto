// Copyright 2020 The Okteto Authors
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

package services

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/errors"
	"github.com/okteto/okteto/pkg/k8s/labels"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestGet(t *testing.T) {
	ctx := context.Background()
	svc := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(svc)
	s, err := Get(ctx, svc.GetName(), svc.GetNamespace(), clientset)
	if err != nil {
		t.Fatal(err)
	}

	if s == nil {
		t.Fatal("empty service")
	}

	if s.Name != svc.GetName() {
		t.Fatalf("wrong service. Got %s, expected %s", s.Name, svc.GetName())
	}

	_, err = Get(ctx, "test", "missing", clientset)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.IsNotFound(err) {
		t.Fatalf("expected not found error got: %s", err)
	}
}

func TestGetNameBySelector(t *testing.T) {

	ctx := context.Background()
	svcNameToFind := "svc"

	tests := []struct {
		name          string
		userLabels    map[string]string
		svcLabels     map[string]string
		expected      string
		expectedError bool
	}{
		{
			"full-match-one-only-label-get-correct-name",
			map[string]string{"app": "db"},
			map[string]string{"app": "db"},
			svcNameToFind,
			false,
		},
		{
			"full-match-labels-get-correct-name",
			map[string]string{"app": "db", "stage": "prod"},
			map[string]string{"app": "db", "stage": "prod"},
			svcNameToFind,
			false,
		},
		{
			"partial-match-labels-get-correct-name",
			map[string]string{"app": "db"},
			map[string]string{"app": "db", "stage": "prod"},
			svcNameToFind,
			false,
		},
		{
			"partial-match-labels-when-multiple-services-same-labels-get-error",
			map[string]string{"stage": "prod"},
			map[string]string{"app": "db", "stage": "prod"},
			"",
			true,
		},
		{
			"none-match-labels",
			map[string]string{"stage": "dev"},
			map[string]string{"app": "db", "stage": "prod"},
			"",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svcToFind := &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      svcNameToFind,
					Namespace: "test",
					Labels:    tt.svcLabels,
				},
			}
			anotherService := &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "error-svc",
					Namespace: "test",
					Labels:    map[string]string{"app": "api", "stage": "prod"},
				},
			}
			clientset := fake.NewSimpleClientset(svcToFind, anotherService)
			userSelector := labels.TransformLabelsToSelector(tt.userLabels)
			svcName, err := GetServiceNameByLabel(ctx, svcToFind.GetNamespace(), clientset, userSelector)
			if err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error while getting service name: %s", err.Error())
				}
				return
			}
			if svcName != svcNameToFind {
				t.Errorf("Not correct service found. Found %s but expected %s", svcName, svcNameToFind)
			}
		})
	}

}
