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

package weaver

import (
	"context"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/pointer"
)

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
			Annotations: map[string]string{"a1": "v1"},
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
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v1",
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
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
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
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
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
			Annotations: map[string]string{"a1": "v1"},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
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
			Ports: []apiv1.ServicePort{
				{
					Name: "port-staging",
					Port: 8080,
				},
			},
			ClusterIP:  "staging-ip",
			ClusterIPs: []string{"staging-ip"},
			Selector:   map[string]string{"l1": "v2"},
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
	expectedS2 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s2",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port-staging",
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
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port-staging",
					Port: 8080,
				},
			},
			ClusterIP:  "staging-ip",
			ClusterIPs: []string{"staging-ip"},
			Selector:   map[string]string{"l1": "v2"},
		},
	}

	expectedS3 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port-staging",
					Port: 8080,
				},
			},
			ClusterIP:  apiv1.ClusterIPNone,
			ClusterIPs: nil,
			Selector:   nil,
		},
	}
	ds3 := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3",
			Namespace: "staging",
			Labels: map[string]string{
				model.DeployedByLabel: "staging",
				"l1":                  "v2",
			},
			Annotations: map[string]string{"a1": "v2"},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port-staging",
					Port: 8080,
				},
			},
			ClusterIP:  "staging-ip",
			ClusterIPs: []string{"staging-ip"},
			Selector:   map[string]string{"l1": "v2"},
		},
	}

	e1 := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s1",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v1",
			},
			Annotations: map[string]string{"a1": "v1"},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "my-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:       "Pod",
							Namespace:  "cindy",
							Name:       "s1",
							APIVersion: "v1",
						},
					},
				},
				Ports: []apiv1.EndpointPort{
					{
						Name:        "port1",
						Port:        8080,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.StringPtr("tcp"),
					},
					{
						Name:        "port2",
						Port:        8081,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.StringPtr("tcp"),
					},
				},
			},
		},
	}

	e2 := &apiv1.Endpoints{
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
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "my-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:       "Service",
							Namespace:  "staging",
							Name:       "s2",
							APIVersion: "v1",
						},
					},
				},
				Ports: []apiv1.EndpointPort{
					{
						Name:        "port1",
						Port:        8080,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.StringPtr("tcp"),
					},
					{
						Name:        "port2",
						Port:        8081,
						Protocol:    apiv1.ProtocolTCP,
						AppProtocol: pointer.StringPtr("tcp"),
					},
				},
			},
		},
	}
	expectedE2 := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s2",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
			},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "staging-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:       "Service",
							Namespace:  "staging",
							Name:       "s2",
							APIVersion: "v1",
						},
					},
				},
				Ports: []apiv1.EndpointPort{
					{
						Name: "port-staging",
						Port: 8080,
					},
				},
			},
		},
	}

	expectedE3 := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "s3",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
				"l1":                  "v2",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
				"a1":                             "v2",
			},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "staging-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:       "Service",
							Namespace:  "staging",
							Name:       "s3",
							APIVersion: "v1",
						},
					},
				},
				Ports: []apiv1.EndpointPort{
					{
						Name: "port-staging",
						Port: 8080,
					},
				},
			},
		},
	}
	c := fake.NewSimpleClientset(i1, i2, di1, di2, di3, s1, s2, ds1, ds2, ds3, e1, e2)
	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}

	d := &Driver{client: c, name: m.Name, namespace: m.Namespace, divert: *m.Deploy.Divert}
	err := d.Deploy(ctx)
	assert.NoError(t, err)

	resultI1, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI1, resultI1)
	resultS1, err := c.CoreV1().Services("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s1, resultS1)
	resultE1, err := c.CoreV1().Endpoints("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, e1, resultE1)
	resultI2, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI2, resultI2)
	resultS2, err := c.CoreV1().Services("cindy").Get(ctx, "s2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedS2, resultS2)
	resultE2, err := c.CoreV1().Endpoints("cindy").Get(ctx, "s2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedE2, resultE2)
	resultI3, err := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI3, resultI3)
	resultS3, err := c.CoreV1().Services("cindy").Get(ctx, "s3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedS3, resultS3)
	resultE3, err := c.CoreV1().Endpoints("cindy").Get(ctx, "s3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedE3, resultE3)

	// Eliminate elements from the cache to force RCs
	d.cache.developerIngresses = map[string]*networkingv1.Ingress{}
	d.cache.developerServices = map[string]*apiv1.Service{}
	d.cache.developerEndpoints = map[string]*apiv1.Endpoints{}
	err = d.Deploy(ctx)
	assert.NoError(t, err)

	resultI1, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI1, resultI1)
	resultS1, err = c.CoreV1().Services("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, s1, resultS1)
	resultE1, err = c.CoreV1().Endpoints("cindy").Get(ctx, "s1", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, e1, resultE1)
	resultI2, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI2, resultI2)
	resultS2, err = c.CoreV1().Services("cindy").Get(ctx, "s2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedS2, resultS2)
	resultE2, err = c.CoreV1().Endpoints("cindy").Get(ctx, "s2", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedE2, resultE2)
	resultI3, err = c.NetworkingV1().Ingresses("cindy").Get(ctx, "i3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedI3, resultI3)
	resultS3, err = c.CoreV1().Services("cindy").Get(ctx, "s3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedS3, resultS3)
	resultE3, err = c.CoreV1().Endpoints("cindy").Get(ctx, "s3", metav1.GetOptions{})
	assert.NoError(t, err)
	assert.Equal(t, expectedE3, resultE3)
}
