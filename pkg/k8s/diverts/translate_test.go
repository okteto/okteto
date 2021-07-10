// Copyright 2021 The Okteto Authors
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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	yaml "gopkg.in/yaml.v2"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
)

func Test_translateDeployment(t *testing.T) {
	original := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			UID:             types.UID("id"),
			Name:            "name",
			Namespace:       "namespace",
			Annotations:     map[string]string{"annotation1": "value1"},
			Labels:          map[string]string{"label1": "value1", model.DeployedByLabel: "cindy"},
			ResourceVersion: "version",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "value",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "value",
					},
				},
			},
		},
	}
	translated := translateDeployment("cindy", original)
	expected := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cindy-name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                    "value1",
				model.OktetoAutoCreateAnnotation: model.OktetoUpCmd,
			},
			Labels: map[string]string{
				model.DeployedByLabel:   "cindy",
				model.OktetoDivertLabel: "cindy",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					model.OktetoDivertLabel: "cindy",
				},
			},
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						model.OktetoDivertLabel: "cindy",
					},
				},
			},
		},
	}
	marshalled, _ := yaml.Marshal(translated)
	marshalledExpected, _ := yaml.Marshal(expected)
	if string(marshalled) != string(marshalledExpected) {
		t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
	}
}

func Test_translateServiceNotDiverted(t *testing.T) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cindy-name",
		},
	}
	original := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("id"),
			Name:      "name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                     "value1",
				model.OktetoAutoIngressAnnotation: "true",
			},
			Labels:          map[string]string{"label1": "value1", model.DeployedByLabel: "cindy"},
			ResourceVersion: "version",
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "10.52.11.123",
			Selector: map[string]string{
				"app": "value",
			},
			Ports: []apiv1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	k8sObject := &model.K8sObject{}
	k8sObject.UpdateDeployment(d)
	translated, err := translateService("cindy", k8sObject, original)
	if err != nil {
		t.Fatalf("error translating service: %s", err.Error())
	}
	expected := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cindy-name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1": "value1",
			},
			Labels: map[string]string{
				model.DeployedByLabel:   "cindy",
				model.OktetoDivertLabel: "cindy",
			},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				model.OktetoDivertLabel:   "cindy",
				model.InteractiveDevLabel: "cindy-name",
			},
			Ports: []apiv1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}
	marshalled, _ := yaml.Marshal(translated)
	marshalledExpected, _ := yaml.Marshal(expected)
	if string(marshalled) != string(marshalledExpected) {
		t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
	}
}

func Test_translateServiceDiverted(t *testing.T) {
	d := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "cindy-name",
		},
	}
	original := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("id"),
			Name:      "name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                                   "value1",
				model.OktetoAutoIngressAnnotation:               "true",
				model.OktetoDivertServiceModificationAnnotation: "{\"proxy_port\":1026,\"original_port\":8080,\"original_target_port\":8080}",
			},
			Labels:          map[string]string{"label1": "value1"},
			ResourceVersion: "version",
		},
		Spec: apiv1.ServiceSpec{
			ClusterIP: "10.52.11.123",
			Selector: map[string]string{
				"app": "value",
			},
			Ports: []apiv1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(1026),
				},
			},
		},
	}

	expected := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cindy-name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1": "value1",
			},
			Labels: map[string]string{model.OktetoDivertLabel: "cindy"},
		},
		Spec: apiv1.ServiceSpec{
			Selector: map[string]string{
				model.OktetoDivertLabel:   "cindy",
				model.InteractiveDevLabel: "cindy-name",
			},
			Ports: []apiv1.ServicePort{
				{
					Port:       8080,
					TargetPort: intstr.FromInt(8080),
				},
			},
		},
	}

	t.Run("IntServiceModValues", func(t *testing.T) {
		k8sObject := &model.K8sObject{}
		k8sObject.UpdateDeployment(d)
		translated, err := translateService("cindy", k8sObject, original)
		if err != nil {
			t.Fatalf("error translating service: %s", err.Error())
		}
		marshalled, _ := yaml.Marshal(translated)
		marshalledExpected, _ := yaml.Marshal(expected)
		if string(marshalled) != string(marshalledExpected) {
			t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
		}
	})

	t.Run("StringServiceModValues", func(t *testing.T) {
		stringMod := "{\"proxy_port\":\"1026\",\"original_port\":\"8080\",\"original_target_port\":\"8080\"}"
		original.ObjectMeta.Annotations[model.OktetoDivertServiceModificationAnnotation] = stringMod

		k8sObject := &model.K8sObject{}
		k8sObject.UpdateDeployment(d)
		translated, err := translateService("cindy", k8sObject, original)
		if err != nil {
			t.Fatalf("error translating service: %s", err.Error())
		}
		marshalled, _ := yaml.Marshal(translated)
		marshalledExpected, _ := yaml.Marshal(expected)
		if string(marshalled) != string(marshalledExpected) {
			t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
		}
	})
}

func Test_translateIngressGenerateHostTrue(t *testing.T) {
	original := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("id"),
			Name:      "name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                       "value1",
				model.OktetoIngressAutoGenerateHost: "true",
			},
			Labels:          map[string]string{"label1": "value1", model.DeployedByLabel: "cindy"},
			ResourceVersion: "version",
		},
	}
	translated := translateIngress("cindy", original)
	expected := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cindy-name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                       "value1",
				model.OktetoIngressAutoGenerateHost: "true",
			},
			Labels: map[string]string{
				model.DeployedByLabel:   "cindy",
				model.OktetoDivertLabel: "cindy",
			},
		},
	}
	marshalled, _ := yaml.Marshal(translated.ObjectMeta)
	marshalledExpected, _ := yaml.Marshal(expected.ObjectMeta)
	if string(marshalled) != string(marshalledExpected) {
		t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
	}
}

func Test_translateIngressCustomGenerateHost(t *testing.T) {
	original := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID("id"),
			Name:      "name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                       "value1",
				model.OktetoIngressAutoGenerateHost: "custom",
			},
			Labels:          map[string]string{"label1": "value1"},
			ResourceVersion: "version",
		},
	}
	translated := translateIngress("cindy", original)
	expected := &apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cindy-name",
			Namespace: "namespace",
			Annotations: map[string]string{
				"annotation1":                       "value1",
				model.OktetoIngressAutoGenerateHost: "cindy-custom",
			},
			Labels: map[string]string{model.OktetoDivertLabel: "cindy"},
		},
	}
	marshalled, _ := yaml.Marshal(translated.ObjectMeta)
	marshalledExpected, _ := yaml.Marshal(expected.ObjectMeta)
	if string(marshalled) != string(marshalledExpected) {
		t.Fatalf("Wrong translation.\nActual %+v, \nExpected %+v", string(marshalled), string(marshalledExpected))
	}
}
