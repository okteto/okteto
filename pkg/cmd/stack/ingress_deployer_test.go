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
	"testing"

	"github.com/okteto/okteto/pkg/k8s/ingresses"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func TestDeployK8sEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		stack     *model.Stack
		ingresses []runtime.Object
	}{
		{
			name: "deploy public endpoints",
			stack: &model.Stack{
				Namespace: "test",
				Services: model.ComposeServices{
					"test": &model.Service{},
				},
			},
		},
		{
			name: "deploy private endpoints",
			stack: &model.Stack{
				Namespace: "test",
				Services: model.ComposeServices{
					"test": &model.Service{
						Annotations: model.Annotations{
							"dev.okteto.com/private": "true",
						},
					},
				},
			},
		},
		{
			name: "skip deploy endpoint 1",
			stack: &model.Stack{
				Namespace: "test",
				Services: model.ComposeServices{
					"test": &model.Service{
						Annotations: model.Annotations{
							"dev.okteto.com/private": "true",
						},
					},
				},
			},
			ingresses: []runtime.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							model.StackNameLabel: "",
						},
					},
				},
			},
		},
		{
			name: "skip deploy endpoint 2",
			stack: &model.Stack{
				Name:      "test",
				Namespace: "test",
				Services: model.ComposeServices{
					"test": &model.Service{
						Annotations: model.Annotations{
							"dev.okteto.com/private": "true",
						},
					},
				},
			},
			ingresses: []runtime.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							model.StackNameLabel: "test",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.ingresses...)
			c := ingresses.NewIngressClient(fakeClient, true)
			err := deployK8sEndpoint(context.Background(), "test", "test", model.Port{ContainerPort: 80}, tt.stack, c)
			assert.NoError(t, err)

			obj, err := c.Get(context.Background(), "test", "test")
			assert.NoError(t, err)
			assert.NotNil(t, obj)
		})
	}
}

func TestIngressDeployer_DeployServiceEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		stack     *model.Stack
		ingresses []runtime.Object
	}{
		{
			name: "deploy new ingress endpoint",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "test-stack",
				Services: model.ComposeServices{
					"test-service": &model.Service{},
				},
			},
		},
		{
			name: "update existing ingress endpoint",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "test-stack",
				Services: model.ComposeServices{
					"test-service": &model.Service{},
				},
			},
			ingresses: []runtime.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test",
						Labels: map[string]string{
							model.StackNameLabel: "test-stack",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.ingresses...)
			c := ingresses.NewIngressClient(fakeClient, true)
			deployer := &ingressDeployer{
				client:    c,
				stackName: tt.stack.Name,
				namespace: tt.stack.Namespace,
			}

			err := deployer.DeployServiceEndpoint(context.Background(), "test-endpoint", "test-service", model.Port{ContainerPort: 80}, tt.stack)
			assert.NoError(t, err)

			// Verify the ingress was created/updated
			obj, err := c.Get(context.Background(), "test-endpoint", "test")
			assert.NoError(t, err)
			assert.NotNil(t, obj)
		})
	}
}

func TestIngressDeployer_DeployComposeEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		stack     *model.Stack
		endpoint  model.Endpoint
		ingresses []runtime.Object
	}{
		{
			name: "deploy new compose endpoint",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "test-stack",
				Services: model.ComposeServices{
					"test-service": &model.Service{},
				},
			},
			endpoint: model.Endpoint{
				Rules: []model.EndpointRule{
					{
						Service: "test-service",
						Port:    8080,
						Path:    "/",
					},
				},
			},
		},
		{
			name: "update existing compose endpoint",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "test-stack",
				Services: model.ComposeServices{
					"test-service": &model.Service{},
				},
			},
			endpoint: model.Endpoint{
				Rules: []model.EndpointRule{
					{
						Service: "test-service",
						Port:    8080,
						Path:    "/api",
					},
				},
			},
			ingresses: []runtime.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test",
						Labels: map[string]string{
							model.StackNameLabel: "test-stack",
						},
					},
				},
			},
		},
		{
			name: "skip deploy when stack name label differs",
			stack: &model.Stack{
				Namespace: "test",
				Name:      "test-stack",
				Services: model.ComposeServices{
					"test-service": &model.Service{},
				},
			},
			endpoint: model.Endpoint{
				Rules: []model.EndpointRule{
					{
						Service: "test-service",
						Port:    8080,
						Path:    "/",
					},
				},
			},
			ingresses: []runtime.Object{
				&networkingv1.Ingress{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-endpoint",
						Namespace: "test",
						Labels: map[string]string{
							model.StackNameLabel: "different-stack",
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewSimpleClientset(tt.ingresses...)
			c := ingresses.NewIngressClient(fakeClient, true)
			deployer := &ingressDeployer{
				client:    c,
				stackName: tt.stack.Name,
				namespace: tt.stack.Namespace,
			}

			err := deployer.DeployComposeEndpoint(context.Background(), "test-endpoint", tt.endpoint, tt.stack)
			assert.NoError(t, err)
		})
	}
}
