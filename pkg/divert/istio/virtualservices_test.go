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

package istio

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/constants"
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
		vs       *istioV1beta1.VirtualService
		expected *istioV1beta1.VirtualService
		name     string
		routes   []string
	}{
		{
			name: "add-divert-annotation",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
			},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "staging",
					Labels:    map[string]string{"l1": "v1"},
					Annotations: map[string]string{
						"a1": "v1",
						fmt.Sprintf(constants.OktetoDivertAnnotationTemplate, "2615052508acbfaddeba0eeded4131631ea31a02"): `{"namespace":"cindy"}`,
					},
				},
			},
		},
		{
			name: "add-divert-annotation-with-routes",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "service-a",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
			},
			routes: []string{"one-route", "another-route"},
			expected: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "staging",
					Labels:    map[string]string{"l1": "v1"},
					Annotations: map[string]string{
						"a1": "v1",
						fmt.Sprintf(constants.OktetoDivertAnnotationTemplate, "2615052508acbfaddeba0eeded4131631ea31a02"): `{"namespace":"cindy","routes":["one-route","another-route"]}`,
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
			result, err := d.translateDivertVirtualService(tt.vs, tt.routes)
			assert.NoError(t, err)
			assert.Equal(t, result.Annotations, tt.expected.Annotations)
		})
	}
}

func Test_restoreDivertVirtualService(t *testing.T) {
	tests := []struct {
		vs       *istioV1beta1.VirtualService
		expected *istioV1beta1.VirtualService
		name     string
	}{
		{
			name: "clean-divert-annotation",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "staging",
					Annotations: map[string]string{
						"a1": "v1",
						fmt.Sprintf(constants.OktetoDivertAnnotationTemplate, "2615052508acbfaddeba0eeded4131631ea31a02"): `{"namespace":"cindy","header":{"name":"okteto-divert","match":"exact","value":"cindy"},"routes":null}`,
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
			},
		},
		{
			name: "clean-deprecated-divert-annotation",
			vs: &istioV1beta1.VirtualService{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service-a",
					Namespace: "staging",
					Annotations: map[string]string{
						"a1": "v1",
						fmt.Sprintf(constants.OktetoDeprecatedDivertAnnotationTemplate, "cindy", "test"): `{"namespace":"cindy","header":{"name":"okteto-divert","match":"exact","value":"cindy"},"routes":null}`,
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
			assert.Equal(t, result.Annotations, tt.expected.Annotations)
		})
	}
}

func Test_translateDivertHost(t *testing.T) {
	tests := []struct {
		vs       *istioV1beta1.VirtualService
		expected *istioV1beta1.VirtualService
		name     string
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
									Add: map[string]string{constants.OktetoDivertBaggageHeader: "okteto-divert=cindy"},
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
									Add: map[string]string{constants.OktetoDivertBaggageHeader: "okteto-divert=cindy"},
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
