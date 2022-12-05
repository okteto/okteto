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

package diverts

import (
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/pointer"
)

func Test_translateIngress(t *testing.T) {
	in := &networkingv1.Ingress{
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
	}
	expected := &networkingv1.Ingress{
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
	result := translateIngress(m, in)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateEmptyIngress(t *testing.T) {
	in := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "staging",
		},
		Spec: networkingv1.IngressSpec{
			Rules: []networkingv1.IngressRule{},
			TLS:   []networkingv1.IngressTLS{},
		},
	}
	expected := &networkingv1.Ingress{
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
	result := translateIngress(m, in)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateService(t *testing.T) {
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "name",
			Namespace:   "staging",
			Labels:      map[string]string{"l1": "v1"},
			Annotations: map[string]string{"a1": "v1"},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port",
					Port: 8080,
				},
			},
			ClusterIP:  "my-ip",
			ClusterIPs: []string{"my-ip"},
			Selector:   map[string]string{"label": "value"},
		},
	}
	expected := &apiv1.Service{
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
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name: "port",
					Port: 8080,
				},
			},
			ClusterIP:  apiv1.ClusterIPNone,
			ClusterIPs: nil,
			Selector:   nil,
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
	result, _ := translateService(m, s)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateDivertedService(t *testing.T) {
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "staging",
			Labels:    map[string]string{"l1": "v1"},
			Annotations: map[string]string{
				"a1":                                "v1",
				model.OktetoDivertServiceAnnotation: "{\"proxy_port\":1024,\"original_port\":3000,\"original_target_port\":3000}",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name:       "port1",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
				{
					Name:       "port2",
					Port:       3000,
					TargetPort: intstr.IntOrString{IntVal: 1024},
				},
			},
			ClusterIP:  "my-ip",
			ClusterIPs: []string{"my-ip"},
			Selector:   map[string]string{"label": "value"},
		},
	}
	expected := &apiv1.Service{
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
			},
		},
		Spec: apiv1.ServiceSpec{
			Type: apiv1.ServiceTypeClusterIP,
			Ports: []apiv1.ServicePort{
				{
					Name:       "port1",
					Port:       8080,
					TargetPort: intstr.IntOrString{IntVal: 8080},
				},
				{
					Name:       "port2",
					Port:       3000,
					TargetPort: intstr.IntOrString{IntVal: 3000},
				},
			},
			ClusterIP:  apiv1.ClusterIPNone,
			ClusterIPs: nil,
			Selector:   nil,
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
	result, _ := translateService(m, s)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateEmptyService(t *testing.T) {
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "staging",
		},
		Spec: apiv1.ServiceSpec{
			Type:       apiv1.ServiceTypeClusterIP,
			ClusterIP:  "my-ip",
			ClusterIPs: []string{"my-ip"},
			Selector:   map[string]string{"label": "value"},
		},
	}
	expected := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "name",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: apiv1.ServiceSpec{
			Type:       apiv1.ServiceTypeClusterIP,
			ClusterIP:  apiv1.ClusterIPNone,
			ClusterIPs: nil,
			Selector:   nil,
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
	result, _ := translateService(m, s)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateEndpoints(t *testing.T) {
	s := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			UID:         types.UID("my-uid"),
			Name:        "name",
			Namespace:   "staging",
			Labels:      map[string]string{"l1": "v1"},
			Annotations: map[string]string{"a1": "v1"},
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "my-ip",
			Ports: []apiv1.ServicePort{
				{
					Name:        "port1",
					Port:        8080,
					TargetPort:  intstr.IntOrString{IntVal: 9090},
					Protocol:    apiv1.ProtocolTCP,
					AppProtocol: pointer.StringPtr("tcp"),
				},
				{
					Name:        "port2",
					Port:        8081,
					TargetPort:  intstr.IntOrString{IntVal: 9091},
					Protocol:    apiv1.ProtocolTCP,
					AppProtocol: pointer.StringPtr("tcp"),
				},
			},
		},
	}
	expected := &apiv1.Endpoints{
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
			},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "my-ip",
						TargetRef: &apiv1.ObjectReference{
							Kind:            "Service",
							Namespace:       s.Namespace,
							Name:            s.Name,
							UID:             s.UID,
							APIVersion:      "v1",
							ResourceVersion: s.ResourceVersion,
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
	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace: "staging",
			},
		},
	}
	result := translateEndpoints(m, s)
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateDivertCRD(t *testing.T) {
	m := &model.Manifest{
		Name:      "test",
		Namespace: "cindy",
		Deploy: &model.DeployInfo{
			Divert: &model.DivertDeploy{
				Namespace:  "staging",
				Service:    "service",
				Deployment: "deployment",
				Port:       8080,
			},
		},
	}
	expected := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-service",
			Namespace: "cindy",
			Labels: map[string]string{
				model.DeployedByLabel:    "test",
				"dev.okteto.com/version": "0.1.9",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
		},
		Spec: DivertSpec{
			Ingress: IngressDivertSpec{
				Value: "cindy",
			},
			FromService: ServiceDivertSpec{
				Name:      "service",
				Namespace: "staging",
				Port:      8080,
			},
			ToService: ServiceDivertSpec{
				Name:      "service",
				Namespace: "cindy",
				Port:      8080,
			},
			Deployment: DeploymentDivertSpec{
				Name:      "deployment",
				Namespace: "staging",
			},
		},
	}
	result := translateDivertCRD(m)
	assert.True(t, reflect.DeepEqual(result, expected))
}
