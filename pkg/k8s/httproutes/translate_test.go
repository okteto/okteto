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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/require"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestTranslate(t *testing.T) {
	pathMatchType := gatewayv1.PathMatchPathPrefix
	port80 := gatewayv1.PortNumber(80)
	port8080 := gatewayv1.PortNumber(8080)
	gatewayNamespace := gatewayv1.Namespace("gateway-ns")

	tests := []struct {
		name                         string
		endpointName                 string
		endpoint                     model.Endpoint
		opts                         *TranslateOptions
		expectedHTTPRouteName        string
		expectedHTTPRouteAnnotations map[string]string
		expectedHTTPRouteLabels      map[string]string
		expectedHTTPRouteRules       []gatewayv1.HTTPRouteRule
		expectedParentRefs           []gatewayv1.ParentReference
	}{
		{
			name:         "is-compose",
			endpointName: "endpoint1",
			endpoint: model.Endpoint{
				Labels: model.Labels{
					"label1":                     "value1",
					model.StackNameLabel:         "stackname",
					model.StackEndpointNameLabel: "endpoint1",
				},
				Annotations: model.Annotations{"annotation1": "value1"},
				Rules: []model.EndpointRule{
					{
						Path:    "/",
						Service: "svcName",
						Port:    80,
					},
				},
			},
			opts: &TranslateOptions{
				Name:             "stackName",
				Namespace:        "test-ns",
				GatewayName:      "test-gateway",
				GatewayNamespace: "gateway-ns",
			},
			expectedHTTPRouteName: "endpoint1",
			expectedHTTPRouteAnnotations: map[string]string{
				"annotation1": "value1",
			},
			expectedHTTPRouteLabels: map[string]string{
				model.DeployedByLabel:        "stackname",
				model.StackNameLabel:         "stackname",
				model.StackEndpointNameLabel: "endpoint1",
				"label1":                     "value1",
			},
			expectedHTTPRouteRules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatchType,
								Value: ptrString("/"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "svcName",
									Port: &port80,
								},
							},
						},
					},
				},
			},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "test-gateway",
					Namespace: &gatewayNamespace,
				},
			},
		},
		{
			name:         "is-okteto-manifest",
			endpointName: "endpoint1",
			endpoint: model.Endpoint{
				Labels:      model.Labels{"label1": "value1"},
				Annotations: model.Annotations{"annotation1": "value1"},
				Rules: []model.EndpointRule{
					{
						Path:    "/",
						Service: "svcName",
						Port:    80,
					},
				},
			},
			opts: &TranslateOptions{
				Name:             "manifestName",
				Namespace:        "test-ns",
				GatewayName:      "test-gateway",
				GatewayNamespace: "gateway-ns",
			},
			expectedHTTPRouteName: "endpoint1",
			expectedHTTPRouteAnnotations: map[string]string{
				"annotation1": "value1",
			},
			expectedHTTPRouteLabels: map[string]string{
				model.DeployedByLabel: "manifestname",
				"label1":              "value1",
			},
			expectedHTTPRouteRules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatchType,
								Value: ptrString("/"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "svcName",
									Port: &port80,
								},
							},
						},
					},
				},
			},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "test-gateway",
					Namespace: &gatewayNamespace,
				},
			},
		},
		{
			name:         "multiple-rules",
			endpointName: "endpoint-multi",
			endpoint: model.Endpoint{
				Labels:      model.Labels{"label1": "value1"},
				Annotations: model.Annotations{},
				Rules: []model.EndpointRule{
					{
						Path:    "/api",
						Service: "api-service",
						Port:    8080,
					},
					{
						Path:    "/web",
						Service: "web-service",
						Port:    80,
					},
				},
			},
			opts: &TranslateOptions{
				Name:             "app",
				Namespace:        "test-ns",
				GatewayName:      "test-gateway",
				GatewayNamespace: "gateway-ns",
			},
			expectedHTTPRouteName:        "endpoint-multi",
			expectedHTTPRouteAnnotations: map[string]string{},
			expectedHTTPRouteLabels: map[string]string{
				model.DeployedByLabel: "app",
				"label1":              "value1",
			},
			expectedHTTPRouteRules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatchType,
								Value: ptrString("/api"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "api-service",
									Port: &port8080,
								},
							},
						},
					},
				},
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatchType,
								Value: ptrString("/web"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "web-service",
									Port: &port80,
								},
							},
						},
					},
				},
			},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "test-gateway",
					Namespace: &gatewayNamespace,
				},
			},
		},
		{
			name:         "empty-endpoint-name-uses-opts-name",
			endpointName: "",
			endpoint: model.Endpoint{
				Labels:      model.Labels{},
				Annotations: model.Annotations{},
				Rules: []model.EndpointRule{
					{
						Path:    "/",
						Service: "svc",
						Port:    80,
					},
				},
			},
			opts: &TranslateOptions{
				Name:             "defaultName",
				Namespace:        "test-ns",
				GatewayName:      "test-gateway",
				GatewayNamespace: "gateway-ns",
			},
			expectedHTTPRouteName:        "defaultname",
			expectedHTTPRouteAnnotations: map[string]string{},
			expectedHTTPRouteLabels: map[string]string{
				model.DeployedByLabel: "defaultname",
			},
			expectedHTTPRouteRules: []gatewayv1.HTTPRouteRule{
				{
					Matches: []gatewayv1.HTTPRouteMatch{
						{
							Path: &gatewayv1.HTTPPathMatch{
								Type:  &pathMatchType,
								Value: ptrString("/"),
							},
						},
					},
					BackendRefs: []gatewayv1.HTTPBackendRef{
						{
							BackendRef: gatewayv1.BackendRef{
								BackendObjectReference: gatewayv1.BackendObjectReference{
									Name: "svc",
									Port: &port80,
								},
							},
						},
					},
				},
			},
			expectedParentRefs: []gatewayv1.ParentReference{
				{
					Name:      "test-gateway",
					Namespace: &gatewayNamespace,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Translate(tt.endpointName, tt.endpoint, tt.opts)

			require.Equal(t, tt.expectedHTTPRouteName, result.Name)
			require.Equal(t, tt.opts.Namespace, result.Namespace)
			require.Equal(t, tt.expectedHTTPRouteAnnotations, result.Annotations)
			require.Equal(t, tt.expectedHTTPRouteLabels, result.Labels)
			require.Equal(t, tt.expectedHTTPRouteRules, result.Spec.Rules)
			require.Equal(t, tt.expectedParentRefs, result.Spec.ParentRefs)
		})
	}
}

func TestSetLabels(t *testing.T) {
	tests := []struct {
		name           string
		endpoint       model.Endpoint
		opts           *TranslateOptions
		expectedLabels map[string]string
	}{
		{
			name: "with custom labels",
			endpoint: model.Endpoint{
				Labels: model.Labels{
					"custom-label": "custom-value",
					"app":          "myapp",
				},
			},
			opts: &TranslateOptions{
				Name: "testStack",
			},
			expectedLabels: map[string]string{
				model.DeployedByLabel: "teststack",
				"custom-label":        "custom-value",
				"app":                 "myapp",
			},
		},
		{
			name: "without custom labels",
			endpoint: model.Endpoint{
				Labels: model.Labels{},
			},
			opts: &TranslateOptions{
				Name: "testStack",
			},
			expectedLabels: map[string]string{
				model.DeployedByLabel: "teststack",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setLabels(tt.endpoint, tt.opts)
			require.Equal(t, tt.expectedLabels, result)
		})
	}
}

func TestSetAnnotations(t *testing.T) {
	tests := []struct {
		name                string
		endpoint            model.Endpoint
		expectedAnnotations map[string]string
	}{
		{
			name: "with custom annotations",
			endpoint: model.Endpoint{
				Annotations: model.Annotations{
					"custom-annotation":                        "custom-value",
					"nginx.ingress.kubernetes.io/ssl-redirect": "true",
				},
			},
			expectedAnnotations: map[string]string{
				"custom-annotation":                        "custom-value",
				"nginx.ingress.kubernetes.io/ssl-redirect": "true",
			},
		},
		{
			name: "without custom annotations",
			endpoint: model.Endpoint{
				Annotations: model.Annotations{},
			},
			expectedAnnotations: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := setAnnotations(tt.endpoint)
			require.Equal(t, tt.expectedAnnotations, result)
		})
	}
}

func ptrString(s string) *string {
	return &s
}
