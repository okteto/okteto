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
	"fmt"
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
			Name: "name",
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
	result := translateIngress(m, in, "")
	fmt.Println(result)
	fmt.Println(expected)
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
			Name: "name",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
			ResourceVersion: "version",
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
	result := translateIngress(m, in, "version")
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
			Name: "name",
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
	result := translateService(m, s, "")
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
			Name: "name",
			Labels: map[string]string{
				model.DeployedByLabel: "test",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
			ResourceVersion: "version",
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
	result := translateService(m, s, "version")
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateEndpoints(t *testing.T) {
	e := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			UID:         types.UID("my-uid"),
			Name:        "name",
			Namespace:   "staging",
			Labels:      map[string]string{"l1": "v1"},
			Annotations: map[string]string{"a1": "v1"},
		},
		Subsets: []apiv1.EndpointSubset{
			{
				Addresses: []apiv1.EndpointAddress{
					{
						IP: "my-ip",
					},
				},
			},
		},
	}
	expected := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "name",
			Labels: map[string]string{
				model.DeployedByLabel:         "test",
				model.OktetoDivertedFromLabel: "my-uid",
				"l1":                          "v1"},
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
	result := translateEndpoints(m, e, "")
	assert.True(t, reflect.DeepEqual(result, expected))
}

func Test_translateEmptyEndpoints(t *testing.T) {
	e := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("my-uid"),
			Name:      "name",
			Namespace: "staging",
		},
	}
	expected := &apiv1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "name",
			Labels: map[string]string{
				model.DeployedByLabel:         "test",
				model.OktetoDivertedFromLabel: "my-uid",
			},
			Annotations: map[string]string{
				model.OktetoAutoCreateAnnotation: "true",
			},
			ResourceVersion: "version",
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
	result := translateEndpoints(m, e, "version")
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
	in := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "ingress",
			Namespace: "staging",
		},
	}
	expected := &Divert{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Divert",
			APIVersion: "weaver.okteto.com/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        "test-ingress",
			Namespace:   "cindy",
			Labels:      map[string]string{model.DeployedByLabel: "test"},
			Annotations: map[string]string{model.OktetoAutoCreateAnnotation: "true"},
		},
		Spec: DivertSpec{
			Ingress: IngressDivertSpec{
				Name:      "ingress",
				Namespace: "cindy",
				Value:     "cindy",
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
	result := translateDivertCRD(m, in)
	assert.True(t, reflect.DeepEqual(result, expected))
}
