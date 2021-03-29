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

func Test_PortUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		portRaw       string
		expected      Port
		expectedError bool
	}{
		{
			name:          "singlePort",
			portRaw:       "3000",
			expected:      Port{Port: 3000, Public: true},
			expectedError: false,
		},
		{
			name:          "singleRange",
			portRaw:       "3000-3005",
			expected:      Port{Port: 0, Public: false},
			expectedError: true,
		},
		{
			name:          "singlePortForwarding",
			portRaw:       "8000:8000",
			expected:      Port{Port: 8000, Public: true},
			expectedError: false,
		},
		{
			name:          "RangeForwarding",
			portRaw:       "9090-9091:8080-8081",
			expected:      Port{Port: 0, Public: false},
			expectedError: true,
		},
		{
			name:          "DifferentPort",
			portRaw:       "49100:22",
			expected:      Port{Port: 22, Public: false},
			expectedError: false,
		},
		{
			name:          "LocalhostForwarding",
			portRaw:       "127.0.0.1:8001:8001",
			expected:      Port{Port: 8001, Public: true},
			expectedError: false,
		},
		{
			name:          "Localhost Range",
			portRaw:       "127.0.0.1:5000-5010:5000-5010",
			expected:      Port{Port: 0, Public: false},
			expectedError: true,
		},
		{
			name:          "Protocol",
			portRaw:       "6060:6060/udp",
			expected:      Port{Port: 6060, Public: false},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result Port
			if err := yaml.Unmarshal([]byte(tt.portRaw), &result); err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if result.Port != tt.expected.Port {
				t.Errorf("didn't unmarshal correctly Port. Actual %d, Expected %d", result.Port, tt.expected.Port)
			}
			if result.Public != tt.expected.Public {
				t.Errorf("didn't unmarshal correctly Public. Actual %t, Expected %t", result.Public, tt.expected.Public)
			}

			_, err := yaml.Marshal(&result)
			if err != nil {
				t.Fatalf("error marshaling %s: %s", tt.name, err)
			}
		})
	}
}

func Test_DurationUnmarshalling(t *testing.T) {

	tests := []struct {
		name     string
		duration []byte
		expected int64
	}{
		{
			name:     "string-no-units",
			duration: []byte("12"),
			expected: 12,
		},
		{
			name:     "string-no-units",
			duration: []byte("12s"),
			expected: 12,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg *RawMessage
			if err := yaml.Unmarshal(tt.duration, &msg); err != nil {
				t.Fatal(err)
			}

			duration, err := unmarshalDuration(msg)

			if err != nil {
				t.Fatal(err)
			}

			if duration != tt.expected {
				t.Errorf("didn't unmarshal correctly. Actual %+v, Expected %+v", duration, tt.expected)
			}
		})
	}
}
