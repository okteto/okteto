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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_translateService(t *testing.T) {
	var tests = []struct {
		s        *apiv1.Service
		expected *apiv1.Service
		name     string
	}{
		{
			name: "ok",
			s: &apiv1.Service{
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
			},
			expected: &apiv1.Service{
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
			},
		},
		{
			name: "empty",
			s: &apiv1.Service{
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
			},
			expected: &apiv1.Service{
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateService("test", "cindy", tt.s)
			assert.Equal(t, result, tt.expected)
		})
	}
}
