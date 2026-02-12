// Copyright 2023-2025 The Okteto Authors
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

package httproutes

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8sTesting "k8s.io/client-go/testing"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
	gatewayfake "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned/fake"
)

func TestCreate(t *testing.T) {
	ctx := context.Background()
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := gatewayfake.NewSimpleClientset()
	hrClient := Client{
		gatewayClient: clientset,
	}
	err := hrClient.Create(ctx, httpRoute)
	if err != nil {
		t.Fatal(err)
	}
	retrieved, err := clientset.GatewayV1().HTTPRoutes(httpRoute.Namespace).Get(ctx, httpRoute.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, httpRoute) {
		t.Fatalf("Didn't created correctly")
	}
}

func TestUpdate(t *testing.T) {
	ctx := context.Background()
	labels := map[string]string{"key": "value"}
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    labels,
		},
	}

	clientset := gatewayfake.NewSimpleClientset(httpRoute)
	hrClient := Client{
		gatewayClient: clientset,
	}

	updatedLabels := map[string]string{"key": "value", "key2": "value2"}
	updatedHTTPRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
			Labels:    updatedLabels,
		},
	}
	err := hrClient.Update(ctx, updatedHTTPRoute)
	if err != nil {
		t.Fatal(err)
	}

	retrieved, err := clientset.GatewayV1().HTTPRoutes(httpRoute.Namespace).Get(ctx, httpRoute.Name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, updatedHTTPRoute) {
		t.Fatalf("Didn't updated correctly")
	}
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := gatewayfake.NewSimpleClientset(httpRoute)
	hrClient := Client{
		gatewayClient: clientset,
	}

	retrieved, err := hrClient.Get(ctx, httpRoute.Name, httpRoute.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(retrieved, httpRoute) {
		t.Fatalf("Didn't get correctly")
	}
}

func TestList(t *testing.T) {
	ctx := context.Background()
	httpRoute := &gatewayv1.HTTPRoute{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "fake",
			Namespace: "test",
		},
	}

	clientset := gatewayfake.NewSimpleClientset(httpRoute)
	hrClient := Client{
		gatewayClient: clientset,
	}
	hrList, err := hrClient.List(ctx, httpRoute.Namespace, "")
	if err != nil {
		t.Fatal(err)
	}

	if len(hrList) != 1 {
		t.Fatal(fmt.Errorf("Expected 1 httproute, found %d", len(hrList)))
	}
}

func TestDestroy(t *testing.T) {
	var tests = []struct {
		httpRoute *gatewayv1.HTTPRoute
		name      string
		hrName    string
		namespace string
	}{
		{
			name:      "existent-httproute",
			hrName:    "httproute-test",
			namespace: "test",
			httpRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "httproute-test",
					Namespace: "test",
				},
			},
		},
		{
			name:      "httproute-not-found",
			hrName:    "httproute-test",
			namespace: "test",
			httpRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "non-existent-httproute",
					Namespace: "another-space",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clientset := gatewayfake.NewSimpleClientset(tt.httpRoute)
			hrClient := Client{
				gatewayClient: clientset,
			}

			err := hrClient.Destroy(ctx, tt.hrName, tt.namespace)

			if err != nil {
				t.Fatalf("unexpected error '%s'", err)
			}
		})
	}
}

func TestDestroyWithError(t *testing.T) {
	ctx := context.Background()
	httpRouteName := "httproute-test"
	namespace := "test"

	kubernetesError := "something went wrong in the test"
	clientset := gatewayfake.NewSimpleClientset()
	clientset.PrependReactor("delete", "httproutes", func(action k8sTesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New(kubernetesError)
	})
	hrClient := Client{
		gatewayClient: clientset,
	}

	err := hrClient.Destroy(ctx, httpRouteName, namespace)

	if err == nil {
		t.Fatal("an error was expected but no error was returned")
	}
	if !strings.Contains(err.Error(), kubernetesError) {
		t.Fatalf("Got '%s' error but expected '%s'", err.Error(), kubernetesError)
	}
}

func TestDeploy(t *testing.T) {
	var tests = []struct {
		name            string
		existingRoute   *gatewayv1.HTTPRoute
		httpRoute       *gatewayv1.HTTPRoute
		expectCreate    bool
		expectedLabels  map[string]string
	}{
		{
			name: "create new httproute",
			httpRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "test",
					Labels:    map[string]string{"key": "value"},
				},
			},
			expectCreate:   true,
			expectedLabels: map[string]string{"key": "value"},
		},
		{
			name: "update existing httproute",
			existingRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "test-route",
					Namespace:       "test",
					Labels:          map[string]string{"key": "value"},
					ResourceVersion: "1",
				},
			},
			httpRoute: &gatewayv1.HTTPRoute{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-route",
					Namespace: "test",
					Labels:    map[string]string{"key": "value", "key2": "value2"},
				},
			},
			expectCreate:   false,
			expectedLabels: map[string]string{"key": "value", "key2": "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			var clientset *gatewayfake.Clientset
			if tt.existingRoute != nil {
				clientset = gatewayfake.NewSimpleClientset(tt.existingRoute)
			} else {
				clientset = gatewayfake.NewSimpleClientset()
			}

			hrClient := Client{
				gatewayClient: clientset,
			}

			err := hrClient.Deploy(ctx, tt.httpRoute)
			if err != nil {
				t.Fatal(err)
			}

			retrieved, err := clientset.GatewayV1().HTTPRoutes(tt.httpRoute.Namespace).Get(ctx, tt.httpRoute.Name, metav1.GetOptions{})
			if err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(retrieved.Labels, tt.expectedLabels) {
				t.Fatalf("Expected labels %v, got %v", tt.expectedLabels, retrieved.Labels)
			}
		})
	}
}
