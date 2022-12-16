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

package weaver

import (
	"context"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
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
	expectedI1.Annotations[model.OktetoDivertIngressInjectionAnnotation] = "cindy"
	expectedI1.Annotations[model.OktetoNginxConfigurationSnippetAnnotation] = divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;")
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
				model.OktetoDivertIngressInjectionAnnotation:    "cindy",
				model.OktetoNginxConfigurationSnippetAnnotation: divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;"),
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
				model.OktetoDivertIngressInjectionAnnotation:    "cindy",
				model.OktetoNginxConfigurationSnippetAnnotation: divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;"),
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
				model.OktetoDivertIngressInjectionAnnotation:    "cindy",
				model.OktetoNginxConfigurationSnippetAnnotation: divertTextBlockParser.WriteBlock("proxy_set_header x-okteto-dvrt cindy;"),
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
				Namespace:  "staging",
				Service:    "s1",
				Deployment: "d1",
				Port:       8080,
			},
		},
	}

	d := &Driver{Client: c, Manifest: m}
	d.divertIngresses(ctx)

	resultI1, _ := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i1", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedI1, resultI1) {
		t.Fatalf("Didn't compute i1 correctly: \n%v\n%v", expectedI1, resultI1)
	}

	resultS1, _ := c.CoreV1().Services("cindy").Get(ctx, "s1", metav1.GetOptions{})
	if !reflect.DeepEqual(s1, resultS1) {
		t.Fatalf("Didn't compute s1 correctly: \n%v\n%v", s1, resultS1)
	}

	resultE1, _ := c.CoreV1().Endpoints("cindy").Get(ctx, "s1", metav1.GetOptions{})
	if !reflect.DeepEqual(e1, resultE1) {
		t.Fatalf("Didn't compute e1 correctly: \n%v\n%v", e1, resultE1)
	}

	resultI2, _ := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i2", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedI2, resultI2) {
		t.Fatalf("Didn't compute i2 correctly: \n%v\n%v", expectedI2, resultI2)
	}

	resultS2, _ := c.CoreV1().Services("cindy").Get(ctx, "s2", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedS2, resultS2) {
		t.Fatalf("Didn't compute s2 correctly: \n%v\n%v", expectedS2, resultS2)
	}

	resultE2, _ := c.CoreV1().Endpoints("cindy").Get(ctx, "s2", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedE2, resultE2) {
		t.Fatalf("Didn't compute e2 correctly: \n%v\n%v", expectedE2, resultE2)
	}

	resultI3, _ := c.NetworkingV1().Ingresses("cindy").Get(ctx, "i3", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedI3, resultI3) {
		t.Fatalf("Didn't compute i3 correctly: \n%v\n%v", expectedI3, resultI3)
	}

	resultS3, _ := c.CoreV1().Services("cindy").Get(ctx, "s3", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedS3, resultS3) {
		t.Fatalf("Didn't compute s3 correctly: \n%v\n%v", expectedS3, resultS3)
	}

	resultE3, _ := c.CoreV1().Endpoints("cindy").Get(ctx, "s3", metav1.GetOptions{})
	if !reflect.DeepEqual(expectedE3, resultE3) {
		t.Fatalf("Didn't compute e3 correctly: \n%v\n%v", expectedE3, resultE3)
	}

}

func Test_applyDivertToDeployment(t *testing.T) {
	var tests = []struct {
		name     string
		d        *appsv1.Deployment
		old      *appsv1.Deployment
		expected map[string]string
	}{
		{
			name: "no-divert",
			d: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
				},
			},
			old:      &appsv1.Deployment{},
			expected: map[string]string{"key1": "value1"},
		},
		{
			name: "divert",
			d: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"key1": "value1",
							},
						},
					},
				},
			},
			old: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								model.OktetoDivertInjectSidecarLabel: "cindy",
								"key2":                               "value2",
							},
						},
					},
				},
			},
			expected: map[string]string{
				"key1":                               "value1",
				model.OktetoDivertInjectSidecarLabel: "cindy",
			},
		},
		{
			name: "empty-divert",
			d:    &appsv1.Deployment{},
			old: &appsv1.Deployment{
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								model.OktetoDivertInjectSidecarLabel: "cindy",
								"key2":                               "value2",
							},
						},
					},
				},
			},
			expected: map[string]string{
				model.OktetoDivertInjectSidecarLabel: "cindy",
			},
		},
	}

	driver := Driver{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver.ApplyToDeployment(tt.d, tt.old)
			if !reflect.DeepEqual(tt.d.Spec.Template.Labels, tt.expected) {
				t.Fatalf("Didn't updated template labels correctly")
			}
		})
	}
}

func Test_applyDivertToService(t *testing.T) {
	var tests = []struct {
		name     string
		s        *apiv1.Service
		old      *apiv1.Service
		expected []apiv1.ServicePort
	}{
		{
			name: "no-divert",
			s: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "web1",
							Port:       8080,
							TargetPort: intstr.IntOrString{IntVal: 80},
						},
						{
							Name:       "web2",
							Port:       8081,
							TargetPort: intstr.IntOrString{IntVal: 81},
						},
					},
				},
			},
			old: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"key1": "value1",
					},
				},
			},
			expected: []apiv1.ServicePort{
				{
					Name:       "web1",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
				{
					Name:       "web2",
					Port:       8081,
					TargetPort: intstr.IntOrString{IntVal: 81},
				},
			},
		},
		{
			name: "divert",
			s: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "web1",
							Port:       8080,
							TargetPort: intstr.IntOrString{IntVal: 80},
						},
						{
							Name:       "web2",
							Port:       8081,
							TargetPort: intstr.IntOrString{IntVal: 81},
						},
					},
				},
			},
			old: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						model.OktetoDivertServiceAnnotation: "{\"proxy_port\":1024,\"original_port\":8081,\"original_target_port\":81}",
						"key1":                              "value1",
					},
				},
			},
			expected: []apiv1.ServicePort{
				{
					Name:       "web1",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
				{
					Name:       "web2",
					Port:       8081,
					TargetPort: intstr.IntOrString{IntVal: 1024},
				},
			},
		},
		{
			name: "divert-with-autocreate",
			s: &apiv1.Service{
				Spec: apiv1.ServiceSpec{
					Ports: []apiv1.ServicePort{
						{
							Name:       "web1",
							Port:       8080,
							TargetPort: intstr.IntOrString{IntVal: 80},
						},
						{
							Name:       "web2",
							Port:       8081,
							TargetPort: intstr.IntOrString{IntVal: 81},
						},
					},
				},
			},
			old: &apiv1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						model.OktetoDivertServiceAnnotation: "{\"proxy_port\":1024,\"original_port\":8081,\"original_target_port\":81}",
						"key1":                              "value1",
						model.OktetoAutoCreateAnnotation:    "true",
					},
				},
			},
			expected: []apiv1.ServicePort{
				{
					Name:       "web1",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 80},
				},
				{
					Name:       "web2",
					Port:       8081,
					TargetPort: intstr.IntOrString{IntVal: 81},
				},
			},
		},
	}

	driver := Driver{}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			driver.ApplyToService(tt.s, tt.old)
			if !reflect.DeepEqual(tt.s.Spec.Ports, tt.expected) {
				t.Fatalf("Didn't updated ports correctly")
			}
		})
	}
}
