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
	"reflect"
	"testing"

	yaml "gopkg.in/yaml.v2"
	"k8s.io/apimachinery/pkg/api/resource"
)

func Test_DeployReplicasUnmarshalling(t *testing.T) {
	tests := []struct {
		name      string
		deployRaw *DeployInfoRaw
		scale     int32
		replicas  int32
		expected  int32
	}{
		{
			name:      "empty",
			deployRaw: &DeployInfoRaw{},
			scale:     0,
			replicas:  0,
			expected:  1,
		},
		{
			name:      "deploy-replicas-set",
			deployRaw: &DeployInfoRaw{Replicas: 4},
			scale:     0,
			replicas:  0,
			expected:  4,
		},
		{
			name:      "scale",
			deployRaw: &DeployInfoRaw{},
			scale:     3,
			replicas:  0,
			expected:  3,
		},
		{
			name:      "replicas",
			deployRaw: &DeployInfoRaw{},
			scale:     0,
			replicas:  2,
			expected:  2,
		},
		{
			name:      "replicas-and-deploy-replicas",
			deployRaw: &DeployInfoRaw{Replicas: 3},
			scale:     0,
			replicas:  2,
			expected:  3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas, _ := unmarshalDeployReplicas(tt.deployRaw, tt.scale, tt.replicas)
			if replicas != tt.expected {
				t.Fatalf("expected %d replicas but got %d", tt.expected, replicas)
			}

		})
	}
}

func Test_DeployResourcesUnmarshalling(t *testing.T) {
	tests := []struct {
		name      string
		deployRaw *DeployInfoRaw
		resources *StackResources
		expected  *StackResources
	}{
		{
			name:      "both-nil",
			deployRaw: nil,
			resources: nil,
			expected:  &StackResources{},
		},
		{
			name: "deploy-resources-only-limits",
			deployRaw: &DeployInfoRaw{Resources: ResourcesRaw{
				Limits: DeployComposeResources{
					Cpus:   Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
			},
			},
			resources: &StackResources{},
			expected: &StackResources{
				Limits: ServiceResources{
					CPU:    Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
			},
		},
		{
			name:      "resources",
			deployRaw: nil,
			resources: &StackResources{Limits: ServiceResources{
				CPU:    Quantity{resource.MustParse("1")},
				Memory: Quantity{resource.MustParse("1Gi")},
			}},
			expected: &StackResources{
				Limits: ServiceResources{
					CPU:    Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
			},
		},
		{
			name: "deploy-resources",
			deployRaw: &DeployInfoRaw{Resources: ResourcesRaw{
				Limits: DeployComposeResources{
					Cpus:   Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
				Reservations: DeployComposeResources{
					Cpus:   Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("2Gi")},
				},
			},
			},
			resources: &StackResources{},
			expected: &StackResources{
				Limits: ServiceResources{
					CPU:    Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
				Requests: ServiceResources{
					CPU:    Quantity{resource.MustParse("1")},
					Memory: Quantity{resource.MustParse("2Gi")},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources, _ := unmarshalDeployResources(tt.deployRaw, tt.resources)
			if !reflect.DeepEqual(tt.expected, resources) {
				t.Fatalf("expected %v but got %v", tt.expected, resources)
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

func Test_GroupNotSupportedFields(t *testing.T) {

	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no-need-to-group",
			input:    []string{"volumes", "networks", "service[app].cpus"},
			expected: []string{"volumes", "networks", "service[app].cpus"},
		},
		{
			name:     "string-no-units",
			input:    []string{"volumes", "networks", "service[app].cpus", "service[db].cpus"},
			expected: []string{"volumes", "networks", "service[app, db].cpus"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			output := GroupWarningsBySvc(tt.input)
			if !reflect.DeepEqual(tt.expected, output) {
				t.Errorf("didn't group correctly. Actual %+v, Expected %+v", output, tt.expected)
			}
		})
	}
}

func TestStackResourcesUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		expected StackResources
	}{
		{
			name: "limits-requests",
			data: []byte("limits:\n  cpu: 100m\n  memory: 100Gi\nrequests:\n  cpu: 200m\n  memory: 200Gi\n"),
			expected: StackResources{
				Limits: ServiceResources{
					CPU: Quantity{
						Value: resource.MustParse("100m"),
					},
					Memory: Quantity{
						Value: resource.MustParse("100Gi"),
					},
				},
				Requests: ServiceResources{
					CPU: Quantity{
						Value: resource.MustParse("200m"),
					},
					Memory: Quantity{
						Value: resource.MustParse("200Gi"),
					},
				},
			},
		},
		{
			name: "simple-resources",
			data: []byte("cpu: 100m\nmemory: 100Gi\n"),
			expected: StackResources{
				Limits: ServiceResources{
					CPU: Quantity{
						Value: resource.MustParse("100m"),
					},
					Memory: Quantity{
						Value: resource.MustParse("100Gi"),
					},
				},
			},
		},
		{
			name: "cpu-memory-and-storage",
			data: []byte("cpu: 100m\nmemory: 100Gi\nstorage:\n  size: 5Gi\n  class: standard\n"),
			expected: StackResources{
				Limits: ServiceResources{
					CPU: Quantity{
						Value: resource.MustParse("100m"),
					},
					Memory: Quantity{
						Value: resource.MustParse("100Gi"),
					},
				},
				Requests: ServiceResources{
					Storage: StorageResource{
						Size: Quantity{
							Value: resource.MustParse("5Gi"),
						},
						Class: "standard",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			var stackResources StackResources
			if err := yaml.UnmarshalStrict(tt.data, &stackResources); err != nil {
				t.Fatal(err)
			}

			if !reflect.DeepEqual(stackResources, tt.expected) {
				t.Errorf("didn't unmarshal correctly. Actual %v, Expected %v", stackResources, tt.expected)
			}

		})
	}
}
