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

package ingressesv1

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8sTesting "k8s.io/client-go/testing"
)

func TestDeployCreate(t *testing.T) {
	ctx := context.Background()
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset()
	err := Deploy(ctx, ingress, clientset)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.NetworkingV1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, ingress) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestDeployUpdate(t *testing.T) {
	ctx := context.Background()
	labels := map[string]string{"key": "value"}
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    labels,
		},
	}

	clientset := fake.NewSimpleClientset(ingress)

	updatedLabels := map[string]string{"key": "value", "key2": "value2"}
	updatedIngress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    updatedLabels,
		},
	}
	err := Deploy(ctx, updatedIngress, clientset)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := clientset.NetworkingV1().Ingresses(ingress.Namespace).Get(ctx, ingress.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, updatedIngress) {
		t.Fatalf("Didn't updated correctly")
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := fake.NewSimpleClientset(ingress)
	ingressList, err := List(ctx, ingress.Namespace, "", clientset)
	if err != nil {
		t.Fatal(err)
	}

	if len(ingressList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 ingress, found %d", len(ingressList)))
	}

}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		ingress     *networkingv1.Ingress
		name        string
		ingressName string
		namespace   string
	}{
		{
			name:        "existent-ingress",
			ingressName: "ingress-test",
			namespace:   "test",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ingress-test",
					Namespace: "test",
				},
			},
		},
		{
			name:        "ingress-not-found",
			ingressName: "ingress-test",
			namespace:   "test",
			ingress: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-ingress",
					Namespace: "another-space",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			client := fake.NewSimpleClientset(tt.ingress)

			err := Destroy(ctx, tt.ingressName, tt.namespace, client)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	ingressName := "ingress-test"
	namespace := "test"

	kubernetesError := "something went wrong in the test"
	client := fake.NewSimpleClientset()
	client.Fake.PrependReactor("delete", "ingresses", func(action k8sTesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(kubernetesError)
	})

	err := Destroy(ctx, ingressName, namespace, client)

	if err == nil {
		t.Fatal("an error was expected but no error was returned")
	}
	if !strings.Contains(err.Error(), kubernetesError) {
		t.Fatalf("Got '%s' error but expected '%s'", err.Error(), kubernetesError)
	}
}
