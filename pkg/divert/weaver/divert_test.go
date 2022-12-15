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
	"reflect"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

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
