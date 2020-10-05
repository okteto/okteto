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
	s, err := Get(ctx, svc.GetNamespace(), svc.GetName(), clientset)
	if err != nil {
		t.Fatal(err)
	}

	if s == nil {
		t.Fatal("empty service")
	}

	if s.Name != svc.GetName() {
		t.Fatalf("wrong service. Got %s, expected %s", s.Name, svc.GetName())
	}

	_, err = Get(ctx, "missing", "test", clientset)
	if err == nil {
		t.Fatal("expected error")
	}

	if !errors.IsNotFound(err) {
		t.Fatalf("expected not found error got: %s", err)
	}
}
