// Copyright 2020 The Okteto Authors
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

package build

import (
	"testing"

	"github.com/okteto/okteto/pkg/model"
	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func Test_GetImageTag(t *testing.T) {
	var tests = []struct {
		name              string
		dev               *model.Dev
		imageTag          string
		d                 *appsv1.Deployment
		isOktetoNamespace bool
		expected          string
	}{
		{
			name:              "imageTag-not-in-okteto",
			dev:               &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:          "imageTag",
			d:                 &appsv1.Deployment{},
			isOktetoNamespace: false,
			expected:          "imageTag",
		},
		{
			name:              "imageTag-in-okteto",
			dev:               &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:          "imageTag",
			d:                 &appsv1.Deployment{},
			isOktetoNamespace: true,
			expected:          "imageTag",
		},
		{
			name:              "okteto",
			dev:               &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag:          "",
			d:                 &appsv1.Deployment{},
			isOktetoNamespace: true,
			expected:          "registry.okteto.net/ns/dev:okteto",
		},
		{
			name:     "not-in-okteto",
			dev:      &model.Dev{Name: "dev", Namespace: "ns"},
			imageTag: "",
			d: &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					UID: types.UID("uuid"),
				},
				Spec: appsv1.DeploymentSpec{
					Template: apiv1.PodTemplateSpec{
						Spec: apiv1.PodSpec{
							Containers: []apiv1.Container{
								{
									Image: "okteto/test:2",
								},
							},
						},
					},
				},
			},
			isOktetoNamespace: false,
			expected:          "okteto/test:uuid",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetImageTag(tt.dev, tt.imageTag, tt.d, tt.isOktetoNamespace)
			if tt.expected != result {
				t.Errorf("expected %s got %s in test %s", tt.expected, result, tt.name)
			}
		})
	}
}
