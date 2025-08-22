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

package nginx

import (
	"context"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/divert/k8s"
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeDivertManager struct {
	mock.Mock
}

func (f *fakeDivertManager) CreateOrUpdate(ctx context.Context, d *k8s.Divert) error {
	args := f.Called(ctx, d)
	return args.Error(0)
}

func Test_updateEnvVar(t *testing.T) {
	tests := []struct {
		name     string
		envVars  []apiv1.EnvVar
		envName  string
		envValue string
		expected []apiv1.EnvVar
	}{
		{
			name:     "empty-env-vars",
			envVars:  []apiv1.EnvVar{},
			envName:  "TEST_VAR",
			envValue: "test-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "TEST_VAR",
					Value: "test-value",
				},
			},
		},
		{
			name: "update-existing-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "old-value",
				},
			},
			envName:  "TEST_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "TEST_VAR",
					Value: "new-value",
				},
			},
		},
		{
			name: "add-new-var",
			envVars: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
			},
			envName:  "NEW_VAR",
			envValue: "new-value",
			expected: []apiv1.EnvVar{
				{
					Name:  "EXISTING_VAR",
					Value: "existing-value",
				},
				{
					Name:  "NEW_VAR",
					Value: "new-value",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			envVars := tt.envVars
			updateEnvVar(&envVars, tt.envName, tt.envValue)
			assert.Equal(t, tt.expected, envVars)
		})
	}
}

func Test_UpdatePod(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  apiv1.PodSpec
		expected apiv1.PodSpec
		driver   *Driver
	}{
		{
			name: "empty-pod",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-existing-env-vars",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "old-value",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"existing-search"},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local", "existing-search"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-multiple-containers",
			podSpec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-init-containers",
			podSpec: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
			},
			expected: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
		{
			name: "pod-with-init-containers-and-main-containers",
			podSpec: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name: "app3",
					},
				},
			},
			expected: apiv1.PodSpec{
				InitContainers: []apiv1.Container{
					{
						Name: "app1",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
					{
						Name: "app2",
						Env: []apiv1.EnvVar{
							{
								Name:  "EXISTING_VAR",
								Value: "value",
							},
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name: "app3",
						Env: []apiv1.EnvVar{
							{
								Name:  "OKTETO_SHARED_ENVIRONMENT",
								Value: "staging",
							},
							{
								Name:  "OKTETO_DIVERTED_ENVIRONMENT",
								Value: "cindy",
							},
						},
					},
				},
				DNSConfig: &apiv1.PodDNSConfig{
					Searches: []string{"staging.svc.cluster.local"},
				},
			},
			driver: &Driver{
				name:      "test",
				namespace: "cindy",
				divert: model.DivertDeploy{
					Namespace: "staging",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.driver.UpdatePod(tt.podSpec)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func Test_divertIngresses(t *testing.T) {
	ctx := context.Background()
	i1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i1",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{
				"a1":                                    "v1",
				model.OktetoDivertedNamespaceAnnotation: "staging",
				model.OktetoDivertHeaderAnnotation:      "cindy",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i1-cindy.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s1",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i1-cindy.okteto.dev"},
				},
			},
		},
	}
	expectedI1 := i1.DeepCopy()
	di1 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i1",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i1-cstaging.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s1",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i1-staging.okteto.dev"},
				},
			},
		},
	}

	i2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i2",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation:        "true",
				model.OktetoDivertedNamespaceAnnotation: "staging",
				"a1":                                    "v1",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i2-cindy.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s2",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i2-cindy.okteto.dev"},
				},
			},
		},
	}
	expectedI2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i2",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation:        "true",
				model.OktetoDivertedNamespaceAnnotation: "staging",
				model.OktetoDivertHeaderAnnotation:      "cindy",
				"a1":                                    "v2",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i2-cindy.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s2",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i2-cindy.okteto.dev"},
				},
			},
		},
	}
	di2 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i2",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i2-staging.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s2",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i2-staging.okteto.dev"},
				},
			},
		},
	}

	expectedI3 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i3",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation:        "true",
				model.OktetoDivertedNamespaceAnnotation: "staging",
				model.OktetoDivertHeaderAnnotation:      "cindy",
				"a1":                                    "v2",
			},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i3-cindy.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s3",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i3-cindy.okteto.dev"},
				},
			},
		},
	}
	di3 := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "i3",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{
				{
					Host: "i3-staging.okteto.dev",
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "s3",
										},
									},
								},
							},
						},
					},
				},
			},
			TLS: []networkingv1.IngressTLS{
				{
					Hosts: []string{"i3-staging.okteto.dev"},
				},
			},
		},
	}

	s1 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{"a1": "v1", model.OktetoDivertedNamespaceAnnotation: "staging"},
		},
		Spec: apiv1.ServiceSpec{
			Type:         apiv1.ServiceTypeExternalName,
			ExternalName: "s1.staging.svc.cluster.local",
			Ports: []apiv1.ServicePort{
				{
					Name: "port-cindy",
					Port: 8080,
				},
			},
			ClusterIP:  "my-ip",
			ClusterIPs: []string{"my-ip"},
			Selector:   map[string]string{"l1": "v1"},
		},
	}
	ds1 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
		},
	}

	s2 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s2",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v1",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port-cindy",
					Port: 8080,
				},
			},
			ClusterIP:  apiv1.ClusterIPNone,
			ClusterIPs: nil,
			Selector:   nil,
		},
	}

	ds2 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s2",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: apiv1.ServiceSpec{
			Type:         apiv1.ServiceTypeExternalName,
			ExternalName: "s2.staging.svc.cluster.local",
		},
	}

	c := fake.NewSimpleClientset(i1, i2, di1, di2, di3, s1, s2, ds1, ds2)
	m := &model.Manifest{
		Name: "test",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}

	dm := &fakeDivertManager{}

	dm.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(nil).Times(2)

	d := &Driver{client: c, name: m.Name, namespace: "cindy", divert: *m.Deploy.Divert, divertManager: dm}
	err := d.Deploy(ctx)
	assert.NoError(t, err)

	resultI1, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI1, resultI1)
	resultS1, err := c.CoreV1().Services("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s1, resultS1)

	resultI2, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI2, resultI2)

	resultI3, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI3, resultI3)

	// Eliminate elements from the cache to force RCs
	d.cache.developerIngresses = map[string]*networkingv1.Ingress{}
	d.cache.developerServices = map[string]*apiv1.Service{}
	err = d.Deploy(ctx)
	assert.NoError(t, err)

	resultI1, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI1, resultI1)
	resultS1, err = c.CoreV1().Services("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s1, resultS1)

	resultI2, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI2, resultI2)

	resultI3, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI3, resultI3)

	dm.AssertExpectations(t)
}

func Test_deployDivertResources_Success(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
		"svc2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc2",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
		"svc3": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc3",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}
	dm.On("CreateOrUpdate", mock.Anything, mock.MatchedBy(func(d *k8s.Divert) bool {
		return d.Name == "test-svc2" &&
			d.Namespace == "cindy" &&
			d.Spec.Service == "svc2" &&
			d.Spec.SharedNamespace == "staging" &&
			d.Spec.DivertKey == "cindy"
	})).Return(nil).Once()

	dm.On("CreateOrUpdate", mock.Anything, mock.MatchedBy(func(d *k8s.Divert) bool {
		return d.Name == "test-svc3" &&
			d.Namespace == "cindy" &&
			d.Spec.Service == "svc3" &&
			d.Spec.SharedNamespace == "staging" &&
			d.Spec.DivertKey == "cindy"
	})).Return(nil).Once()

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertExpectations(t)
}

func Test_deployDivertResources_NoServices(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}

func Test_deployDivertResources_OnlyAnnotatedServices(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc1",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
		"svc2": {
			ObjectMeta: metav1.ObjectMeta{
				Name:      "svc2",
				Namespace: "cindy",
				Annotations: map[string]string{
					model.OktetoDivertedNamespaceAnnotation: "staging",
				},
			},
		},
	}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.NoError(t, err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}

func Test_deployDivertResources_CreateError(t *testing.T) {
	ctx := context.Background()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc1",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}
	dm.On("CreateOrUpdate", mock.Anything, mock.Anything).Return(fmt.Errorf("failed to create divert")).Once()

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "error diverting service")
	dm.AssertExpectations(t)
}

func Test_deployDivertResources_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	developerServices := map[string]*apiv1.Service{
		"svc1": {
			ObjectMeta: metav1.ObjectMeta{
				Name:        "svc1",
				Namespace:   "cindy",
				Annotations: map[string]string{},
			},
		},
	}

	dm := &fakeDivertManager{}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			Namespace: "staging",
		},
		divertManager: dm,
		cache: &cache{
			developerServices: developerServices,
		},
	}

	err := d.deployDivertResources(ctx)

	assert.Error(t, err)
	assert.Equal(t, ctx.Err(), err)
	dm.AssertNumberOfCalls(t, "CreateOrUpdate", 0)
}
