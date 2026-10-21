// Copyright 2024 The Okteto Authors
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

package ingresses

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	networkingv1beta1 "k8s.io/api/networking/v1beta1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_translateV1(t *testing.T) {
	pathType := networkingv1.PathTypeImplementationSpecific
	tests := []struct {
		name                       string
		endpointName               string
		endpointSpec               model.EndpointSpec
		opts                       *TranslateOptions
		expectedIngressName        string
		expectedIngressAnnotations map[string]string
		expectedIngressLabels      map[string]string
		expectedIngressPaths       []networkingv1.HTTPIngressPath
	}{
		{
			name:         "is-compose",
			endpointName: "endpoint1",
			endpointSpec: model.EndpointSpec{
				"endpoint1": {
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
			},
			opts: &TranslateOptions{
				Name:      "stackName",
				Namespace: "",
			},
			expectedIngressName: "endpoint1",
			expectedIngressAnnotations: map[string]string{
				model.OktetoIngressAutoGenerateHost: "true",
				"annotation1":                       "value1",
			},
			expectedIngressLabels: map[string]string{
				model.DeployedByLabel:        "stackname",
				model.StackNameLabel:         "stackname",
				model.StackEndpointNameLabel: "endpoint1",
				"label1":                     "value1",
			},
			expectedIngressPaths: []networkingv1.HTTPIngressPath{
				{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "svcName",
							Port: networkingv1.ServiceBackendPort{
								Number: 80,
							},
						},
					},
				},
			},
		},
		{
			name:         "is-okteto-manifest",
			endpointName: "endpoint1",
			endpointSpec: model.EndpointSpec{
				"endpoint1": {
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
			},
			opts: &TranslateOptions{
				Name:      "manifestName",
				Namespace: "",
			},
			expectedIngressName: "endpoint1",
			expectedIngressAnnotations: map[string]string{
				model.OktetoIngressAutoGenerateHost: "true",
				"annotation1":                       "value1",
			},
			expectedIngressLabels: map[string]string{
				model.DeployedByLabel: "manifestname",
				"label1":              "value1",
			},
			expectedIngressPaths: []networkingv1.HTTPIngressPath{
				{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{
						Service: &networkingv1.IngressServiceBackend{
							Name: "svcName",
							Port: networkingv1.ServiceBackendPort{
								Number: 80,
							},
						},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateV1(tt.endpointName, tt.endpointSpec[tt.endpointName], tt.opts)
			if result.Name != "endpoint1" {
				t.Errorf("Wrong ingress name: '%s'", result.Name)
			}

			if !reflect.DeepEqual(result.Annotations, tt.expectedIngressAnnotations) {
				t.Errorf("Wrong ingress annotations: '%s'", result.Annotations)
			}
			if !reflect.DeepEqual(result.Spec.Rules[0].HTTP.Paths, tt.expectedIngressPaths) {
				t.Errorf("Wrong ingress spec paths: '%v'", result.Spec.Rules[0].HTTP.Paths)
			}
			if !reflect.DeepEqual(result.Labels, tt.expectedIngressLabels) {
				t.Errorf("Wrong ingress labels: '%s'", result.Labels)
			}
		})
	}
}

func Test_translateV1BetaV1(t *testing.T) {
	tests := []struct {
		name                       string
		endpointName               string
		endpointSpec               model.EndpointSpec
		opts                       *TranslateOptions
		expectedIngressName        string
		expectedIngressAnnotations map[string]string
		expectedIngressLabels      map[string]string
		expectedIngressPaths       []networkingv1beta1.HTTPIngressPath
	}{
		{
			name:         "is-compose",
			endpointName: "endpoint1",
			endpointSpec: model.EndpointSpec{
				"endpoint1": {
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
			},
			opts: &TranslateOptions{
				Name:      "stackName",
				Namespace: "",
			},
			expectedIngressName: "endpoint1",
			expectedIngressAnnotations: map[string]string{
				model.OktetoIngressAutoGenerateHost: "true",
				"annotation1":                       "value1",
			},
			expectedIngressLabels: map[string]string{
				model.DeployedByLabel:        "stackname",
				model.StackNameLabel:         "stackname",
				model.StackEndpointNameLabel: "endpoint1",
				"label1":                     "value1",
			},
			expectedIngressPaths: []networkingv1beta1.HTTPIngressPath{
				{
					Path: "/",
					Backend: networkingv1beta1.IngressBackend{
						ServiceName: "svcName",
						ServicePort: intstr.IntOrString{IntVal: 80},
					},
				},
			},
		},
		{
			name:         "is-okteto-manifest",
			endpointName: "endpoint1",
			endpointSpec: model.EndpointSpec{
				"endpoint1": {
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
			},
			opts: &TranslateOptions{
				Name:      "manifestName",
				Namespace: "",
			},
			expectedIngressName: "endpoint1",
			expectedIngressAnnotations: map[string]string{
				model.OktetoIngressAutoGenerateHost: "true",
				"annotation1":                       "value1",
			},
			expectedIngressLabels: map[string]string{
				model.DeployedByLabel: "manifestname",
				"label1":              "value1",
			},
			expectedIngressPaths: []networkingv1beta1.HTTPIngressPath{
				{
					Path: "/",
					Backend: networkingv1beta1.IngressBackend{
						ServiceName: "svcName",
						ServicePort: intstr.IntOrString{IntVal: 80},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateV1Beta1(tt.endpointName, tt.endpointSpec[tt.endpointName], tt.opts)
			if result.Name != "endpoint1" {
				t.Errorf("Wrong service name: '%s'", result.Name)
			}
			if !reflect.DeepEqual(result.Annotations, tt.expectedIngressAnnotations) {
				t.Errorf("Wrong service annotations: '%s'", result.Annotations)
			}

			if !reflect.DeepEqual(result.Labels, tt.expectedIngressLabels) {
				t.Errorf("Wrong labels: '%s'", result.Labels)
			}

			if !reflect.DeepEqual(result.Spec.Rules[0].HTTP.Paths, tt.expectedIngressPaths) {
				t.Errorf("Wrong ingress: '%v'", result.Spec.Rules[0].HTTP.Paths)
			}

		})
	}
}
