// Copyright 2022 The Okteto Authors
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

package istio

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/okteto/okteto/pkg/okteto"
	"github.com/okteto/okteto/pkg/types"
	"github.com/stretchr/testify/assert"
	istioNetworkingV1beta1 "istio.io/api/networking/v1beta1"
	istioV1beta1 "istio.io/client-go/pkg/apis/networking/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_translateDivertVirtualService(t *testing.T) {
	tests := []struct {
		name     string
		vs       *istioV1beta1.VirtualService
		header   model.DivertHeader
		routes   []string
		expected *istioV1beta1.VirtualService
	}{
		{
			name: "match-default-header-all-routes",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-b",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			header: model.DivertHeader{
				Name:  model.OktetoDivertDefaultHeaderName,
				Match: model.OktetoDivertIstioExactMatch,
				Value: "cindy",
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Headers: map[string]*istioNetworkingV1beta1.StringMatch{
										"x-okteto-divert": {
											MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: "cindy"},
										},
									},
									Port: 80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.cindy.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "okteto-divert-cindy-service-b",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Headers: map[string]*istioNetworkingV1beta1.StringMatch{
										"x-okteto-divert": {
											MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: "cindy"},
										},
									},
									Port: 80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.cindy.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-b",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "match-default-header-single-route",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-b",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			header: model.DivertHeader{
				Name:  model.OktetoDivertDefaultHeaderName,
				Match: model.OktetoDivertIstioExactMatch,
				Value: "cindy",
			},
			routes: []string{"service-a"},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Headers: map[string]*istioNetworkingV1beta1.StringMatch{
										"x-okteto-divert": {
											MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: "cindy"},
										},
									},
									Port: 80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.cindy.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-a",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "service-b",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "match-custom-header",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			header: model.DivertHeader{
				Name:  "custom-name",
				Match: "prefix",
				Value: "custom-prefix",
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Headers: map[string]*istioNetworkingV1beta1.StringMatch{
										"custom-name": {
											MatchType: &istioNetworkingV1beta1.StringMatch_Prefix{Prefix: "custom-prefix"},
										},
									},
									Port: 80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.cindy.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "no-match",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-b",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-b.staging.svc.cluster.local",
						"service-b.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			header: model.DivertHeader{
				Name:  model.OktetoDivertDefaultHeaderName,
				Match: model.OktetoDivertIstioExactMatch,
				Value: "cindy",
			},
			routes: []string{"no-match"},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-b",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-b.staging.svc.cluster.local",
						"service-b.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-b.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
	}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			VirtualServices: []model.DivertVirtualService{
				{
					Name:      "virtual-service-a",
					Namespace: "staging",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d.divert.Header = tt.header
			result := d.translateDivertVirtualService(tt.vs, tt.routes)
			assert.True(t, reflect.DeepEqual(result.ObjectMeta, tt.expected.ObjectMeta))
			assert.True(t, reflect.DeepEqual(result.Spec.Hosts, tt.expected.Spec.Hosts))
			assert.True(t, reflect.DeepEqual(result.Spec.Gateways, tt.expected.Spec.Gateways))
			for i := range tt.expected.Spec.Http {
				for j := range tt.expected.Spec.Http[i].Match {
					assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Match[j].Headers, tt.expected.Spec.Http[i].Match[j].Headers))
				}
				assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Route, tt.expected.Spec.Http[i].Route))
			}
		})
	}
}

func Test_restoreDivertVirtualService(t *testing.T) {
	tests := []struct {
		name     string
		vs       *istioV1beta1.VirtualService
		expected *istioV1beta1.VirtualService
	}{
		{
			name: "clean-divert-routes",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Headers: map[string]*istioNetworkingV1beta1.StringMatch{
										"x-okteto-divert": {
											MatchType: &istioNetworkingV1beta1.StringMatch_Exact{Exact: "cindy"},
										},
									},
									Port: 80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.cindy.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
	}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			VirtualServices: []model.DivertVirtualService{
				{
					Name:      "virtual-service-a",
					Namespace: "staging",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.restoreDivertVirtualService(tt.vs)
			assert.True(t, reflect.DeepEqual(result.ObjectMeta, tt.expected.ObjectMeta))
			assert.True(t, reflect.DeepEqual(result.Spec.Hosts, tt.expected.Spec.Hosts))
			assert.True(t, reflect.DeepEqual(result.Spec.Gateways, tt.expected.Spec.Gateways))
			for i := range tt.expected.Spec.Http {
				assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Headers, tt.expected.Spec.Http[i].Headers))
				assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Route, tt.expected.Spec.Http[i].Route))
			}
		})
	}
}

func Test_translateDivertHost(t *testing.T) {
	tests := []struct {
		name     string
		vs       *istioV1beta1.VirtualService
		expected *istioV1beta1.VirtualService
	}{
		{
			name: "divert-host-service-same-namespace",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "service-a",
					Namespace:       "staging",
					Labels:          map[string]string{"l1": "v1"},
					Annotations:     map[string]string{"a1": "v1"},
					ResourceVersion: "version",
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "cindy",
					Labels: map[string]string{
						"l1":                             "v1",
						"dev.okteto.com/deployed-by":     "test",
						model.OktetoAutoCreateAnnotation: "true",
					},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a-cindy.demo.okteto.dev",
						"service-a.cindy.svc.cluster.local",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Headers: &istioNetworkingV1beta1.Headers{
								Request: &istioNetworkingV1beta1.Headers_HeaderOperations{
									Set: map[string]string{model.OktetoDivertDefaultHeaderName: "cindy"},
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "divert-host-service-different-namespace",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:            "service-a",
					Namespace:       "staging",
					Labels:          map[string]string{"l1": "v1"},
					Annotations:     map[string]string{"a1": "v1"},
					ResourceVersion: "version",
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a.staging.svc.cluster.local",
						"service-a.staging.com",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "okteto-divert-cindy-ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging2.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "cindy",
					Labels: map[string]string{
						"l1":                             "v1",
						"dev.okteto.com/deployed-by":     "test",
						model.OktetoAutoCreateAnnotation: "true",
					},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: istioNetworkingV1beta1.VirtualService{
					Gateways: []string{"ingress-http"},
					Hosts: []string{
						"service-a-cindy.demo.okteto.dev",
						"service-a.cindy.svc.cluster.local",
					},
					Http: []*istioNetworkingV1beta1.HTTPRoute{
						{
							Name: "ingress-gateway-http-app-service",
							Match: []*istioNetworkingV1beta1.HTTPMatchRequest{
								{
									Gateways: []string{"ingress-http"},
									Port:     80,
								},
							},
							Headers: &istioNetworkingV1beta1.Headers{
								Request: &istioNetworkingV1beta1.Headers_HeaderOperations{
									Set: map[string]string{model.OktetoDivertDefaultHeaderName: "cindy"},
								},
							},
							Route: []*istioNetworkingV1beta1.HTTPRouteDestination{
								{
									Destination: &istioNetworkingV1beta1.Destination{
										Host: "service-a.staging2.svc.cluster.local",
										Port: &istioNetworkingV1beta1.PortSelector{
											Number: 80,
										},
										Subset: "stable",
									},
									Weight: 100,
								},
							},
						},
					},
				},
			},
		},
	}

	d := &Driver{
		name:      "test",
		namespace: "cindy",
		divert: model.DivertDeploy{
			VirtualServices: []model.DivertVirtualService{
				{
					Name:      "virtual-service-a",
					Namespace: "staging",
				},
			},
		},
	}
	okteto.AddOktetoContext("test", &types.User{Registry: "registry.demo.okteto.dev"}, "okteto", "cyndy")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.translateDivertHost(tt.vs)
			assert.Equal(t, result.ResourceVersion, "")
			assert.True(t, reflect.DeepEqual(result.ObjectMeta, tt.expected.ObjectMeta))
			assert.True(t, reflect.DeepEqual(result.Spec.Hosts, tt.expected.Spec.Hosts))
			assert.True(t, reflect.DeepEqual(result.Spec.Gateways, tt.expected.Spec.Gateways))
			for i := range tt.expected.Spec.Http {
				assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Headers, tt.expected.Spec.Http[i].Headers))
				assert.True(t, reflect.DeepEqual(result.Spec.Http[i].Route, tt.expected.Spec.Http[i].Route))
			}
		})
	}
}

func Test_matchHTTPRoute(t *testing.T) {
	tests := []struct {
		name   string
		r      *istioNetworkingV1beta1.HTTPRoute
		routes []string
		result bool
	}{
		{
			name: "empty-routes",
			r: &istioNetworkingV1beta1.HTTPRoute{
				Name: "my-route",
			},
			result: true,
		},
		{
			name: "match-routes",
			r: &istioNetworkingV1beta1.HTTPRoute{
				Name: "my-route",
			},
			routes: []string{"other-route", "my-route"},
			result: true,
		},
		{
			name: "no-match-routes",
			r: &istioNetworkingV1beta1.HTTPRoute{
				Name: "my-route",
			},
			routes: []string{"other-route", "another-route"},
			result: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchHTTPRoute(tt.r, tt.routes)
			assert.Equal(t, result, tt.result)
		})
	}
}
