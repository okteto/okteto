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
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_translateIngress(t *testing.T) {
	var tests = []struct {
		name     string
		in       *networkingv1.Ingress
		expected *networkingv1.Ingress
	}{
		{
			name: "ok",
			in: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:        "name",
					Namespace:   "staging",
					Labels:      map[string]string{"l1": "v1"},
					Annotations: map[string]string{"a1": "v1"},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test-staging.okteto.dev",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{"test-staging.okteto.dev"},
						},
					},
				},
			},
			expected: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "cindy",
					Labels: map[string]string{
						model.DeployedByLabel: "test",
						"l1":                  "v1",
					},
					Annotations: map[string]string{
						model.OktetoAutoCreateAnnotation: "true",
						"a1":                             "v1",
						model.OktetoDivertIngressInjectionAnnotation:    "cindy",
						model.OktetoNginxConfigurationSnippetAnnotation: divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;"),
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{
						{
							Host: "test-cindy.okteto.dev",
						},
					},
					TLS: []networkingv1.IngressTLS{
						{
							Hosts: []string{"test-cindy.okteto.dev"},
						},
					},
				},
			},
		},
		{
			name: "empty",
			in: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "staging",
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{},
					TLS:   []networkingv1.IngressTLS{},
				},
			},
			expected: &networkingv1.Ingress{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "name",
					Namespace: "cindy",
					Labels: map[string]string{
						model.DeployedByLabel: "test",
					},
					Annotations: map[string]string{
						model.OktetoAutoCreateAnnotation:                "true",
						model.OktetoDivertIngressInjectionAnnotation:    "cindy",
						model.OktetoNginxConfigurationSnippetAnnotation: divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;"),
					},
				},
				Spec: networkingv1.IngressSpec{
					Rules: []networkingv1.IngressRule{},
					TLS:   []networkingv1.IngressTLS{},
				},
			},
		},
	}

	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateIngress(m, tt.in)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Fatalf("test %s failed: %v", tt.name, result)
			}
		})
	}
}
