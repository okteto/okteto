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

package model

import (
	"testing"
	"time"

	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_DeployUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		deployRaw     *DeployInfoRaw
		scale         int32
		replicas      int32
		resources     ServiceResources
		expected      DeployInfo
		expectedError bool
	}{
		{
			name:          "empty",
			deployRaw:     &DeployInfoRaw{},
			scale:         0,
			replicas:      0,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 1, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "deploy-replicas-set",
			deployRaw:     &DeployInfoRaw{Replicas: 4},
			scale:         0,
			replicas:      0,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 4, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "scale",
			deployRaw:     &DeployInfoRaw{},
			scale:         3,
			replicas:      0,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 3, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "replicas",
			deployRaw:     &DeployInfoRaw{},
			scale:         0,
			replicas:      2,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 2, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "replicas-and-deploy-replicas",
			deployRaw:     &DeployInfoRaw{Replicas: 3},
			scale:         0,
			replicas:      2,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 3, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "resources",
			deployRaw:     &DeployInfoRaw{Resources: ResourcesRaw{Limits: DeployComposeResources{Cpus: Quantity{resource.MustParse("1")}, Memory: Quantity{resource.MustParse("1Gi")}}}},
			scale:         0,
			replicas:      0,
			resources:     ServiceResources{},
			expected:      DeployInfo{Replicas: 1, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: ResourceList{}}},
			expectedError: false,
		},
		{
			name:          "resources-by-stack-resources",
			deployRaw:     &DeployInfoRaw{},
			scale:         0,
			replicas:      0,
			resources:     ServiceResources{Memory: Quantity{resource.MustParse("1Gi")}},
			expected:      DeployInfo{Replicas: 1, Resources: ResourceRequirements{Limits: ResourceList{}, Requests: map[apiv1.ResourceName]resource.Quantity{apiv1.ResourceMemory: resource.MustParse("1Gi")}}},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deployInfo, err := unmarshalDeploy(tt.deployRaw, tt.scale, tt.replicas, tt.resources)
			if deployInfo.Replicas != tt.expected.Replicas {
				t.Fatalf("expected %d replicas but got %d", tt.expected.Replicas, deployInfo.Replicas)
			}

			if err == nil && tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
		})
	}
}

