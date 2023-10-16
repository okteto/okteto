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

package model

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
	apiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/utils/pointer"
)

func TestTranslateHealthcheckCurlToHttp(t *testing.T) {
	for _, tc := range []struct {
		name         string
		test         []string
		expectedTest HealtcheckTest
		expectedHTTP HTTPHealtcheck
	}{
		{
			name:         "RegularCurlWithZeros",
			test:         []string{"curl http://0.0.0.0:4572"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/", Port: int32(4572)},
		},
		{
			name:         "RegularCurlWithLocalhost",
			test:         []string{"curl http://localhost:4572"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/", Port: int32(4572)},
		},
		{
			name:         "RegularCurlTrailingSlash",
			test:         []string{"curl http://0.0.0.0:4572/"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/", Port: int32(4572)},
		},
		{
			name:         "MissingScheme",
			test:         []string{"curl 0.0.0.0:4572/readiness"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/readiness", Port: int32(4572)},
		},
		{
			name:         "MissingSchemeWithF",
			test:         []string{"curl -f localhost:8080/"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/", Port: int32(8080)},
		},
		{
			name:         "NoTest",
			test:         []string{"curl --fail 0.0.0.0:8080"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/", Port: int32(8080)},
		},
		{
			name:         "Readiness",
			test:         []string{"curl https://0.0.0.0:8080/readiness"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/readiness", Port: int32(8080)},
		},
		{
			name:         "RegularCurlWithPath",
			test:         []string{"curl http://0.0.0.0:4572/a/path/exists"},
			expectedTest: HealtcheckTest{},
			expectedHTTP: HTTPHealtcheck{Path: "/a/path/exists", Port: int32(4572)},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			check := HealthCheck{
				Test: tc.test,
			}
			translateHealtcheckCurlToHTTP(&check)
			if !reflect.DeepEqual(check.Test, tc.expectedTest) {
				t.Fatalf("expected %v but got %v", tc.expectedTest, check.Test)
			}
			if !reflect.DeepEqual(check.Test, tc.expectedTest) {
				t.Fatalf("expected %v but got %v", tc.expectedHTTP, check.HTTP)
			}
		})
	}
}

func Test_DeployReplicasUnmarshalling(t *testing.T) {
	tests := []struct {
		name      string
		deployRaw *DeployInfoRaw
		scale     *int32
		replicas  *int32
		expected  int32
	}{
		{
			name:      "empty with other deploy values",
			deployRaw: &DeployInfoRaw{},
			scale:     nil,
			replicas:  nil,
			expected:  1,
		},
		{
			name:      "empty",
			deployRaw: nil,
			scale:     nil,
			replicas:  nil,
			expected:  1,
		},
		{
			name:      "deploy-replicas-set",
			deployRaw: &DeployInfoRaw{Replicas: pointer.Int32Ptr(4)},
			scale:     nil,
			replicas:  nil,
			expected:  4,
		},
		{
			name:      "scale",
			deployRaw: &DeployInfoRaw{},
			scale:     pointer.Int32Ptr(3),
			replicas:  nil,
			expected:  3,
		},
		{
			name:      "replicas",
			deployRaw: &DeployInfoRaw{},
			scale:     nil,
			replicas:  pointer.Int32Ptr(2),
			expected:  2,
		},
		{
			name:      "replicas priority",
			deployRaw: &DeployInfoRaw{Replicas: pointer.Int32Ptr(1)},
			scale:     pointer.Int32Ptr(2),
			replicas:  pointer.Int32Ptr(3),
			expected:  3,
		},
		{
			name:      "deploy priority",
			deployRaw: &DeployInfoRaw{Replicas: pointer.Int32Ptr(1)},
			scale:     pointer.Int32Ptr(2),
			replicas:  nil,
			expected:  1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			replicas := unmarshalDeployReplicas(tt.deployRaw, tt.scale, tt.replicas)
			if replicas != tt.expected {
				t.Fatalf("expected %d replicas but got %d", tt.expected, replicas)
			}

		})
	}
}

func Test_DeployResourcesUnmarshalling(t *testing.T) {
	tests := []struct {
		name           string
		deployRaw      *DeployInfoRaw
		resources      *StackResources
		expected       *StackResources
		cpu_count      Quantity
		cpus           Quantity
		memLimit       Quantity
		memReservation Quantity
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
		{
			name: "deprecated-volumes-not-set-if-they-already-set",
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
			resources:      &StackResources{},
			cpu_count:      Quantity{resource.MustParse("3")},
			cpus:           Quantity{resource.MustParse("2")},
			memLimit:       Quantity{resource.MustParse("2Gi")},
			memReservation: Quantity{resource.MustParse("2Gi")},
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
		{
			name:           "set-deprecated-volumes-if-they-already-set",
			deployRaw:      nil,
			resources:      nil,
			cpu_count:      Quantity{resource.MustParse("3")},
			cpus:           Quantity{resource.MustParse("2")},
			memLimit:       Quantity{resource.MustParse("2Gi")},
			memReservation: Quantity{resource.MustParse("1Gi")},
			expected: &StackResources{
				Limits: ServiceResources{
					CPU:    Quantity{resource.MustParse("3")},
					Memory: Quantity{resource.MustParse("2Gi")},
				},
				Requests: ServiceResources{
					CPU:    Quantity{resource.MustParse("2")},
					Memory: Quantity{resource.MustParse("1Gi")},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resources := unmarshalDeployResources(tt.deployRaw, tt.resources, tt.cpu_count, tt.cpus, tt.memLimit, tt.memReservation)
			if !reflect.DeepEqual(tt.expected, resources) {
				t.Fatalf("expected %v but got %v", tt.expected, resources)
			}

		})
	}
}

func Test_HealthcheckUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		manifest      []byte
		expected      *HealthCheck
		expectedError bool
	}{
		{
			name:          "healthcheck http through test with https",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl https://0.0.0.0:8080/readiness\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/readiness", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck disable",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      disable: true\n      test: cat file.txt\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: false,
		},
		{
			name:          "healthcheck none",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: [\"NONE\"]\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: false,
		},
		{
			name:          "just healthcheck command",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      test: cat file.txt\n    image: okteto/vote:1"),
			expected:      &HealthCheck{Test: []string{"cat", "file.txt"}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "normal healthcheck",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: cat file.txt\n    image: okteto/vote:1"),
			expected:      &HealthCheck{Test: []string{"cat", "file.txt"}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck without test",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "healthcheck with test and http",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: cat file.txt\n      http:\n        path: /\n        port: 8080\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "healthcheck http path not starting with /",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      http:\n        path: db\n        port: 8080\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "healthcheck http",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      http:\n        path: /\n        port: 8080\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck http through test without failing flag",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl 0.0.0.0:8080/readiness\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/readiness", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck http through test with -f",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl -f localhost:8080/\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck http through test with --fail",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080/\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck http through test without /",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck readiness=false liveness=true",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      x-okteto-readiness: false\n      x-okteto-liveness: true\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: false, Liveness: true},
			expectedError: false,
		},
		{
			name:          "healthcheck readiness=false unset",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      x-okteto-readiness: false\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "healthcheck readiness=false liveness=false",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      x-okteto-readiness: false\n      x-okteto-liveness: false\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
		{
			name:          "healthcheck readiness=true liveness=false",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      x-okteto-readiness: true\n      x-okteto-liveness: false\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true, Liveness: false},
			expectedError: false,
		},
		{
			name:          "healthcheck readiness=true liveness=true",
			manifest:      []byte("services:\n  app:\n    healthcheck:\n      interval: 10s\n      x-okteto-readiness: true\n      x-okteto-liveness: true\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: curl --fail 0.0.0.0:8080\n    image: okteto/vote:1"),
			expected:      &HealthCheck{HTTP: &HTTPHealtcheck{Path: "/", Port: 8080}, Interval: 10 * time.Second, Timeout: 10 * time.Minute, Retries: 5, StartPeriod: 30 * time.Second, Test: []string{}, Readiness: true, Liveness: true},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, true)
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			} else if err == nil && tt.expectedError {
				t.Fatal("error not thrown")
			}

			if !tt.expectedError {
				assert.Equal(t, tt.expected, s.Services["app"].Healtcheck)
			}

		})
	}
}

func Test_NodeSelectorUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		manifest      []byte
		expected      Selector
		expectedError bool
	}{
		{
			name:          "empty node selector",
			manifest:      []byte("services:\n  app:\n\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: false,
		},
		{
			name:          "node selector",
			manifest:      []byte("services:\n  app:\n    image: okteto/vote:1\n    x-node-selector:\n      label: value"),
			expected:      Selector{"label": "value"},
			expectedError: false,
		},
		{
			name:          "wrong selector",
			manifest:      []byte("services:\n  app:\n    image: okteto/vote:1\n    x-node-selector: value"),
			expected:      nil,
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, true)
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			} else if err == nil && tt.expectedError {
				t.Fatal("error not thrown")
			}

			if !tt.expectedError {
				assert.Equal(t, tt.expected, s.Services["app"].NodeSelector)
			}

		})
	}
}

func TestComposeBuildSectionUnmarshalling(t *testing.T) {
	tests := []struct {
		name     string
		bytes    []byte
		expected *composeBuildInfo
	}{
		{
			name:     "with depends on fail",
			bytes:    []byte(`depends_on: a`),
			expected: nil,
		},
		{
			name:  "context",
			bytes: []byte(`context: .`),
			expected: &composeBuildInfo{
				Context: ".",
			},
		},
		{
			name:  "image direct",
			bytes: []byte(`nginx`),
			expected: &composeBuildInfo{
				Name: "nginx",
			},
		},
		{
			name: "complete",
			bytes: []byte(`context: .
dockerfile: Dockerfile
image: nginx`),
			expected: &composeBuildInfo{
				Image:      "nginx",
				Context:    ".",
				Dockerfile: "Dockerfile",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result composeBuildInfo
			err := yaml.UnmarshalStrict(tt.bytes, &result)
			if err != nil && tt.expected == nil {
				return
			}

			assert.Equal(t, tt.expected, &result)
		})
	}
}

func Test_HealthcheckTestUnmarshalling(t *testing.T) {
	tests := []struct {
		name            string
		healthcheckTest string
		expected        HealtcheckTest
		expectedError   bool
	}{
		{
			name:            "empty list",
			healthcheckTest: `[]`,
			expected:        []string{},
			expectedError:   true,
		},
		{
			name:            "NONE simple",
			healthcheckTest: `["NONE"]`,
			expected:        []string{"NONE"},
			expectedError:   false,
		},
		{
			name:            "NONE with args",
			healthcheckTest: `["NONE", "curl -f localhost:5000"]`,
			expected:        []string{"NONE"},
			expectedError:   false,
		},
		{
			name:            "other than the three expected",
			healthcheckTest: `["TEST", "curl -f localhost:5000"]`,
			expected:        []string{},
			expectedError:   true,
		},
		{
			name:            "CMDSHELL",
			healthcheckTest: `["CMD-SHELL", "curl -f localhost:5000"]`,
			expected:        []string{"curl", "-f", "localhost:5000"},
			expectedError:   false,
		},
		{
			name:            "CMD",
			healthcheckTest: `["CMD", "curl", "-f", "localhost:5000"]`,
			expected:        []string{"curl", "-f", "localhost:5000"},
			expectedError:   false,
		},
		{
			name:            "direct",
			healthcheckTest: `curl -f localhost:5000`,
			expected:        []string{"curl", "-f", "localhost:5000"},
			expectedError:   false,
		},
		{
			name: "list",
			healthcheckTest: `- CMD
- curl
- -f
- localhost:5000`,
			expected:      []string{"curl", "-f", "localhost:5000"},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result HealtcheckTest
			if err := yaml.Unmarshal([]byte(tt.healthcheckTest), &result); err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly healthcheck test. Actual %v, Expected %v", result, tt.expected)
			}
		})
	}
}

func Test_PortUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		portRaw       string
		expected      PortRaw
		expectedError bool
	}{
		{
			name:          "singlePort",
			portRaw:       "3000",
			expected:      PortRaw{ContainerPort: 3000, Protocol: apiv1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "singleRange",
			portRaw:       "3000-3005",
			expected:      PortRaw{ContainerFrom: 3000, ContainerTo: 3005, Protocol: apiv1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "singlePortForwarding",
			portRaw:       "8000:8000",
			expected:      PortRaw{HostPort: 8000, ContainerPort: 8000, Protocol: apiv1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "RangeForwarding",
			portRaw:       "9090-9091:8080-8081",
			expected:      PortRaw{ContainerFrom: 8080, ContainerTo: 8081, HostFrom: 9090, HostTo: 9091, Protocol: apiv1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "RangeForwardingNotSameLength",
			portRaw:       "9090-9092:8080-8081",
			expected:      PortRaw{ContainerFrom: 8080, ContainerTo: 8081, HostFrom: 9090, HostTo: 9091, Protocol: apiv1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "DifferentPort",
			portRaw:       "49100:22",
			expected:      PortRaw{HostPort: 49100, ContainerPort: 22, Protocol: apiv1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "LocalhostForwarding",
			portRaw:       "127.0.0.1:8000:8001",
			expected:      PortRaw{HostPort: 8000, ContainerPort: 8001, Protocol: apiv1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "Localhost Range",
			portRaw:       "127.0.0.1:5000-5010:5000-5010",
			expected:      PortRaw{ContainerPort: 0, Protocol: apiv1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "Protocol",
			portRaw:       "6060:6060/udp",
			expected:      PortRaw{HostPort: 6060, ContainerPort: 6060, Protocol: apiv1.ProtocolUDP},
			expectedError: false,
		},
		{
			name:          "ProtocolWithoutMapping",
			portRaw:       "6060/udp",
			expected:      PortRaw{ContainerPort: 6060, Protocol: apiv1.ProtocolUDP},
			expectedError: false,
		},
		{
			name:          "RangeProtocol",
			portRaw:       "6060-6061:6060-6061/udp",
			expected:      PortRaw{ContainerFrom: 6060, ContainerTo: 6061, HostFrom: 6060, HostTo: 6061, Protocol: apiv1.ProtocolUDP},
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result PortRaw
			if err := yaml.Unmarshal([]byte(tt.portRaw), &result); err != nil {
				if !tt.expectedError {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.expectedError {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if result.ContainerPort != tt.expected.ContainerPort {
				t.Errorf("didn't unmarshal correctly Port. Actual %d, Expected %d", result.ContainerPort, tt.expected.ContainerPort)
			}

			if result.HostPort != tt.expected.HostPort {
				t.Errorf("didn't unmarshal correctly Port. Actual %d, Expected %d", result.HostPort, tt.expected.HostPort)
			}

			if result.Protocol != tt.expected.Protocol {
				t.Errorf("didn't unmarshal correctly Port protocol. Actual %s, Expected %s", result.Protocol, tt.expected.Protocol)
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
		{
			name:     "two-into-one",
			input:    []string{"service[app].cpus", "service[db].cpus"},
			expected: []string{"service[app, db].cpus"},
		},
		{
			name:     "only-one",
			input:    []string{"service[app].cpus"},
			expected: []string{"service[app].cpus"},
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

func Test_validateCommandArgs(t *testing.T) {
	tests := []struct {
		name        string
		manifest    []byte
		isCompose   bool
		Entrypoint  Entrypoint
		Command     Command
		expectedErr bool
	}{
		{
			name:        "COMPOSE-only-entrypoint",
			manifest:    []byte("services:\n  app:\n    entrypoint: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   true,
			Entrypoint:  Entrypoint{Values: []string{"/usr/bin/rpk", "redpanda"}},
			Command:     Command{},
			expectedErr: false,
		},
		{
			name:        "COMPOSE-service-empty",
			manifest:    []byte("services:\n  app:\n"),
			isCompose:   true,
			expectedErr: true,
		},
		{
			name:        "STACK-only-entrypoint",
			manifest:    []byte("services:\n  app:\n    entrypoint: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   false,
			Entrypoint:  Entrypoint{},
			Command:     Command{},
			expectedErr: true,
		},
		{
			name:        "COMPOSE-entrypoint-command",
			manifest:    []byte("services:\n  app:\n    entrypoint: [\"entrypoint.sh\"]\n    command: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   true,
			Command:     Command{Values: []string{"/usr/bin/rpk", "redpanda"}},
			Entrypoint:  Entrypoint{Values: []string{"entrypoint.sh"}},
			expectedErr: false,
		},
		{
			name:        "STACK-entrypoint-command",
			manifest:    []byte("services:\n  app:\n    entrypoint: [\"entrypoint.sh\"]\n    command: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   false,
			Command:     Command{},
			Entrypoint:  Entrypoint{},
			expectedErr: true,
		},
		{
			name:        "COMPOSE-only-args",
			manifest:    []byte("services:\n  app:\n    args: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   true,
			Command:     Command{},
			Entrypoint:  Entrypoint{},
			expectedErr: true,
		},
		{
			name:        "STACK-only-args",
			manifest:    []byte("services:\n  app:\n    args: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   false,
			Command:     Command{Values: []string{"/usr/bin/rpk", "redpanda"}},
			Entrypoint:  Entrypoint{},
			expectedErr: false,
		},
		{
			name:        "COMPOSE-command-args",
			manifest:    []byte("services:\n  app:\n    command: [\"entrypoint.sh\"]\n    args: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   true,
			Command:     Command{},
			Entrypoint:  Entrypoint{},
			expectedErr: true,
		},
		{
			name:        "STACK-command-args",
			manifest:    []byte("services:\n  app:\n    command: [\"entrypoint.sh\"]\n    args: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   false,
			Command:     Command{Values: []string{"/usr/bin/rpk", "redpanda"}},
			Entrypoint:  Entrypoint{Values: []string{"entrypoint.sh"}},
			expectedErr: false,
		},
		{
			name:        "COMPOSE-only-command",
			manifest:    []byte("services:\n  app:\n    command: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   true,
			Command:     Command{Values: []string{"/usr/bin/rpk", "redpanda"}},
			Entrypoint:  Entrypoint{},
			expectedErr: false,
		},
		{
			name:        "STACK-only-command",
			manifest:    []byte("services:\n  app:\n    command: [\"/usr/bin/rpk\", \"redpanda\"]\n    image: okteto/vote:1"),
			isCompose:   false,
			Command:     Command{},
			Entrypoint:  Entrypoint{Values: []string{"/usr/bin/rpk", "redpanda"}},
			expectedErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, tt.isCompose)
			if err != nil && !tt.expectedErr {
				t.Fatal(err)
			}
			if !tt.expectedErr {
				if svc, ok := s.Services["app"]; ok {
					if !reflect.DeepEqual(svc.Command, tt.Command) {
						t.Fatalf("Expected %v but got %v", tt.Command, svc.Command)
					}
					if !reflect.DeepEqual(svc.Entrypoint, tt.Entrypoint) {
						t.Fatalf("Expected %v but got %v", tt.Command, svc.Command)
					}
				}
			}
		})
	}
}

func Test_validateVolumesUnmarshalling(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	relativePathExpanded := filepath.Join(wd, "test_volume_relative_path_found")
	relativePathExpandedFile := filepath.Join(wd, "test-file")
	err = os.Mkdir("test_volume_relative_path_found", 0750)
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll("test_volume_relative_path_found")
	file, err := os.Create("test-file")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(file.Name())
	tests := []struct {
		name                 string
		manifest             []byte
		expectedVolumes      []StackVolume
		expectedVolumesMount []StackVolume
		expectedError        bool
	}{
		{
			name:     "volume-relative-path-found",
			manifest: []byte("services:\n  app:\n    volumes: \n    - test_volume_relative_path_found:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedVolumesMount: []StackVolume{
				{
					LocalPath:  relativePathExpanded,
					RemotePath: "/var/lib/redpanda/data",
				},
			},
			expectedVolumes: []StackVolume{},
			expectedError:   false,
		},
		{
			name:     "volume-absolute-path",
			manifest: []byte(fmt.Sprintf("services:\n  app:\n    volumes: \n    - %s:/var/lib/redpanda/data\n    image: okteto/vote:1\n", relativePathExpanded)),
			expectedVolumesMount: []StackVolume{
				{
					LocalPath:  relativePathExpanded,
					RemotePath: "/var/lib/redpanda/data",
				},
			},
			expectedVolumes: []StackVolume{},
			expectedError:   false,
		},
		{
			name:     "correct-volume",
			manifest: []byte("services:\n  app:\n    volumes: \n    - redpanda:/var/lib/redpanda/data\n    image: okteto/vote:1\nvolumes:\n  redpanda:\n"),
			expectedVolumes: []StackVolume{
				{
					LocalPath:  "redpanda",
					RemotePath: "/var/lib/redpanda/data",
				},
			},
			expectedVolumesMount: []StackVolume{},
			expectedError:        false,
		},
		{
			name:          "volume-not-declared-in-volumes-top-level-section",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - redpanda:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: true,
		},
		{
			name:     "absolute path",
			manifest: []byte(fmt.Sprintf("services:\n  app:\n    image: okteto/vote:1\n    volumes:\n      - %s:/var/run/docker.sock", relativePathExpandedFile)),
			expectedVolumesMount: []StackVolume{
				{
					LocalPath:  relativePathExpandedFile,
					RemotePath: "/var/run/docker.sock",
				},
			},
			expectedVolumes: []StackVolume{},
			expectedError:   false,
		},
		{
			name:          "volume-relative-path-not-found",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - test:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: true,
		},
		{
			name:                 "pv",
			manifest:             []byte("services:\n  app:\n    volumes: \n    - /var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedVolumesMount: []StackVolume{},
			expectedVolumes: []StackVolume{
				{
					RemotePath: "/var/lib/redpanda/data",
				},
			},
			expectedError: false,
		},
		{
			name:     "volume-with-underscores",
			manifest: []byte("services:\n  app:\n    volumes: \n    - redpanda_a:/var/lib/redpanda/data\n    image: okteto/vote:1\nvolumes:\n  redpanda_a:\n"),
			expectedVolumes: []StackVolume{
				{
					LocalPath:  "redpanda-a",
					RemotePath: "/var/lib/redpanda/data",
				},
			},
			expectedVolumesMount: []StackVolume{},
			expectedError:        false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stack, err := ReadStack(tt.manifest, true)
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			} else if err == nil && !tt.expectedError {
				assert.Equal(t, stack.Services["app"].Volumes, tt.expectedVolumes)
				assert.Equal(t, stack.Services["app"].VolumeMounts, tt.expectedVolumesMount)
			}
		})
	}
}

func Test_validateIngressCreationPorts(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		isPublic bool
		ports    []Port
	}{
		{
			name:     "expose-range-and-ports-range",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213-9215\n    expose:\n    - 8213-8215\n    image: okteto/vote:1"),
			isPublic: false,
			ports: []Port{
				{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9214, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9215, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 8213, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 8214, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 8215, Protocol: apiv1.ProtocolTCP},
			},
		},
		{
			name:     "not-public-service-with-expose-and-ports",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    expose:\n    - 8213\n    image: okteto/vote:1"),
			isPublic: false,
			ports: []Port{
				{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 8213, Protocol: apiv1.ProtocolTCP},
			},
		},
		{
			name:     "not-public-port-but-with-assignation",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213:9213\n    image: okteto/vote:1"),
			isPublic: true,
			ports:    []Port{{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP}},
		},
		{
			name:     "not-public-range-but-with-assignation",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213-9215:9213-9215\n    image: okteto/vote:1"),
			isPublic: false,
			ports: []Port{
				{HostPort: 9213, ContainerPort: 9213, Protocol: apiv1.ProtocolTCP},
				{HostPort: 9213, ContainerPort: 9214, Protocol: apiv1.ProtocolTCP},
				{HostPort: 9213, ContainerPort: 9215, Protocol: apiv1.ProtocolTCP},
			},
		},
		{
			name:     "Public-service",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			isPublic: true,
			ports:    []Port{{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP}},
		},
		{
			name:     "Public-service-with-range",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213-9215\n    public: true\n    image: okteto/vote:1"),
			isPublic: true,
			ports: []Port{
				{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9214, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9215, Protocol: apiv1.ProtocolTCP},
			},
		},
		{
			name:     "not-public-service",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    image: okteto/vote:1"),
			isPublic: false,
			ports:    []Port{{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP}},
		},
		{
			name:     "not-public-service",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213-9215\n    image: okteto/vote:1"),
			isPublic: false,
			ports: []Port{
				{ContainerPort: 9213, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9214, Protocol: apiv1.ProtocolTCP},
				{ContainerPort: 9215, Protocol: apiv1.ProtocolTCP},
			},
		},

		{
			name:     "mysql-port-forwarding",
			manifest: []byte("services:\n  app:\n    ports:\n    - 3306:3306\n    image: okteto/vote:1"),
			isPublic: false,
			ports:    []Port{{ContainerPort: 3306, Protocol: apiv1.ProtocolTCP}},
		},
		{
			name:     "mysql-port-forwarding-and-public",
			manifest: []byte("services:\n  app:\n    ports:\n    - 3306:3306\n    image: okteto/vote:1\n    public: true"),
			isPublic: true,
			ports:    []Port{{ContainerPort: 3306, Protocol: apiv1.ProtocolTCP}},
		},
		{
			name:     "mysql-expose-forwarding-and-public",
			manifest: []byte("services:\n  app:\n    expose:\n    - 3306:3306\n    image: okteto/vote:1\n    public: true"),
			isPublic: false,
			ports:    []Port{{ContainerPort: 3306, Protocol: apiv1.ProtocolTCP}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			if svc, ok := s.Services["app"]; ok {
				if svc.Public != tt.isPublic {
					t.Fatalf("Expected %v but got %v", tt.isPublic, svc.Public)
				}
			}
			if len(tt.ports) != len(s.Services["app"].Ports) {
				t.Fatalf("Not unmarshalled ports correctly")
			}
		})
	}
}

func Test_unmarshalVolumes(t *testing.T) {
	tests := []struct {
		name           string
		manifest       []byte
		expectedVolume *VolumeSpec
	}{
		{
			name:           "simple volume",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("1Gi")}, Labels: make(map[string]string), Annotations: make(map[string]string)},
		},
		{
			name:           "volume with size",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    size: 2Gi"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("2Gi")}, Labels: make(map[string]string), Annotations: make(map[string]string)},
		},
		{
			name:           "volume with driver_opts.size",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    driver_opts:\n      size: 2Gi"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("2Gi")}, Labels: make(map[string]string), Annotations: make(map[string]string)},
		},
		{
			name:           "volume with driver_opts.class",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    driver_opts:\n      class: standard"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("1Gi")}, Class: "standard", Labels: make(map[string]string), Annotations: make(map[string]string)},
		},
		{
			name:           "volume with class",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    class: standard"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("1Gi")}, Class: "standard", Labels: make(map[string]string), Annotations: make(map[string]string)},
		},
		{
			name:           "volume with labels",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    labels:\n      env: test"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("1Gi")}, Labels: Labels{}, Annotations: Annotations{"env": "test"}},
		},
		{
			name:           "volume with annotations",
			manifest:       []byte("services:\n  app:\n    image: okteto/vote:1\nvolumes:\n  apiv1:\n    annotations:\n      env: test"),
			expectedVolume: &VolumeSpec{Size: Quantity{resource.MustParse("1Gi")}, Annotations: map[string]string{"env": "test"}, Labels: make(map[string]string)},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatalf("Unmarshal failed: %s", err.Error())
			}
			if !reflect.DeepEqual(s.Volumes["apiv1"], tt.expectedVolume) {
				t.Fatalf("Expected %v but got %v", tt.expectedVolume, s.Volumes["apiv1"])
			}
		})
	}
}

func Test_sanitizeVolumeName(t *testing.T) {
	tests := []struct {
		name               string
		volumeName         string
		expectedVolumeName string
	}{
		{
			name:               "correct-volume",
			volumeName:         "redpanda",
			expectedVolumeName: "redpanda",
		},
		{
			name:               "volume-name-with-_",
			volumeName:         "db_postgres",
			expectedVolumeName: "db-postgres",
		},
		{
			name:               "volume-name-with-whitespace",
			volumeName:         "db postgres",
			expectedVolumeName: "db-postgres",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			name := sanitizeName(tt.volumeName)
			if name != tt.expectedVolumeName {
				t.Fatalf("Expected '%s', but got %s", tt.expectedVolumeName, name)
			}
		})
	}
}

func Test_UnmarshalRestart(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		result   apiv1.RestartPolicy
		throwErr bool
	}{
		{
			name:     "Not-supported-policy",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: always\n    deploy:\n      restart_policy:\n        condition: aaa"),
			result:   apiv1.RestartPolicyAlways,
			throwErr: true,
		},
		{
			name:     "restart-always",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: always"),
			result:   apiv1.RestartPolicyAlways,
			throwErr: false,
		},
		{
			name:     "restart-always-default",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1"),
			result:   apiv1.RestartPolicyAlways,
			throwErr: false,
		},
		{
			name:     "restart-always-by-deploy",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    deploy:\n      restart_policy:\n        condition: any"),
			result:   apiv1.RestartPolicyAlways,
			throwErr: false,
		},
		{
			name:     "restart-always",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: on-failure"),
			result:   apiv1.RestartPolicyOnFailure,
			throwErr: false,
		},
		{
			name:     "restart-on-failure-by-deploy",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    deploy:\n      restart_policy:\n        condition: on-failure"),
			result:   apiv1.RestartPolicyOnFailure,
			throwErr: false,
		},
		{
			name:     "restart-always",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: never"),
			result:   apiv1.RestartPolicyNever,
			throwErr: false,
		},
		{
			name:     "restart-never-by-deploy",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    deploy:\n      restart_policy:\n        condition: no"),
			result:   apiv1.RestartPolicyNever,
			throwErr: false,
		},
		{
			name:     "deploy over direct restart",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: always\n    deploy:\n      restart_policy:\n        condition: on-failure"),
			result:   apiv1.RestartPolicyOnFailure,
			throwErr: false,
		},
		{
			name:     "Not-supported-policy",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    restart: always\n    deploy:\n      restart_policy:\n        condition: aaa"),
			result:   apiv1.RestartPolicyAlways,
			throwErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, false)
			if tt.throwErr && err == nil {
				t.Fatal("Not threw error")
			} else if err != nil && !tt.throwErr {
				t.Fatalf("Threw error when no error needed: %s", err.Error())
			}
			if err == nil && s.Services["app"].RestartPolicy != tt.result {
				t.Fatal("Wrong unmarshal")
			}

		})
	}
}

func Test_UnmarshalSvcName(t *testing.T) {
	tests := []struct {
		name            string
		manifest        []byte
		svcName         string
		isSvcNameChange bool
	}{
		{
			name:            "with underscore",
			manifest:        []byte("services:\n  app_1:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			svcName:         "app-1",
			isSvcNameChange: true,
		},
		{
			name:            "with whitespace",
			manifest:        []byte("services:\n  app 1:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			svcName:         "app-1",
			isSvcNameChange: true,
		},
		{
			name:            "with whitespace and underscore",
			manifest:        []byte("services:\n  app_ 1:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			svcName:         "app--1",
			isSvcNameChange: true,
		},
		{
			name:            "without whitespace or underscore",
			manifest:        []byte("services:\n  app:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			svcName:         "app",
			isSvcNameChange: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := s.Services[tt.svcName]; !ok {
				t.Fatal("Service name not sanitized")
			}
			if tt.isSvcNameChange && len(s.Warnings.SanitizedServices) == 0 {
				t.Fatal("Warning have not been recovered")
			}
		})
	}
}

func Test_DeployLabels(t *testing.T) {
	tests := []struct {
		name        string
		manifest    []byte
		annotations Annotations
		labels      Labels
		isCompose   bool
	}{

		{
			name:        "deploy labels",
			manifest:    []byte("services:\n  app:\n    deploy:\n      labels:\n        env: production\n    image: okteto/vote:1"),
			annotations: Annotations{"env": "production"},
			labels:      Labels{},
			isCompose:   true,
		},
		{
			name:        "no labels",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1"),
			annotations: Annotations{},
			labels:      Labels{},
			isCompose:   true,
		},
		{
			name:        "labels on service",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      env: production"),
			annotations: Annotations{"env": "production"},
			labels:      Labels{},
			isCompose:   true,
		},
		{
			name:        "labels on deploy and service",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      app: main\n    deploy:\n      labels:\n        env: production\n"),
			annotations: Annotations{"env": "production", "app": "main"},
			labels:      Labels{},
			isCompose:   true,
		},

		{
			name:        "labels on deploy and service",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      app: main\n    deploy:\n      labels:\n        env: production\n"),
			annotations: Annotations{},
			labels:      Labels{"app": "main", "env": "production"},
			isCompose:   false,
		},
		{
			name:        "labels on service",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      env: production"),
			annotations: Annotations{},
			labels:      Labels{"env": "production"},
			isCompose:   false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, tt.isCompose)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Services["app"].Annotations) != len(tt.annotations) {
				t.Fatalf("Bad deployment annotations")
			}
			if len(s.Services["app"].Labels) != len(tt.labels) {
				t.Fatalf("Bad deployment labels")
			}
		})
	}
}

func Test_endpoints(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		expected EndpointSpec
	}{
		{
			name: "rule with name",
			manifest: []byte(`name: test
services:
  app:
    ports:
      - 9213
    image: okteto/vote:1
endpoints:
  app:
    - path: /
      service: app
      port: 9213`),
			expected: EndpointSpec{
				"app": Endpoint{
					Annotations: make(Annotations),
					Labels:      make(Labels),
					Rules: []EndpointRule{
						{
							Service: "app",
							Path:    "/",
							Port:    9213,
						},
					},
				},
			},
		},
		{
			name: "rule with name and annotations/labels",
			manifest: []byte(`name: test
services:
  app:
    ports:
    - 9213
    image: okteto/vote:1
endpoints:
  app:
    annotations:
      key: value
    labels:
      key: value
    rules:
    - path: /
      service: app
      port: 9213`),
			expected: EndpointSpec{
				"app": Endpoint{
					Annotations: Annotations{"key": "value"},
					Labels:      Labels{},
					Rules: []EndpointRule{
						{
							Service: "app",
							Path:    "/",
							Port:    9213,
						},
					},
				},
			},
		},
		{
			name: "direct rules with labels/annotations",
			manifest: []byte(`name: test
services:
  app:
    ports:
    - 9213
    image: okteto/vote:1
endpoints:
  labels:
    key: value
  rules:
  - path: /
    service: app
    port: 9213`),
			expected: EndpointSpec{
				"": Endpoint{
					Annotations: Annotations{"key": "value"},
					Labels:      Labels{},
					Rules: []EndpointRule{
						{
							Service: "app",
							Path:    "/",
							Port:    9213,
						},
					},
				},
			},
		},
		{
			name: "direct rules",
			manifest: []byte(`name: test
services:
  app:
    ports:
    - 9213
    image: okteto/vote:1
endpoints:
  - path: /
    service: app
    port: 9213`),
			expected: EndpointSpec{
				"": Endpoint{
					Annotations: make(Annotations),
					Labels:      make(Labels),
					Rules: []EndpointRule{
						{
							Service: "app",
							Path:    "/",
							Port:    9213,
						},
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tt.expected, s.Endpoints) {
				t.Fatalf("Expected %v, but got %v", tt.expected, s.Endpoints)
			}
		})
	}
}

func Test_validateEnvFiles(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		EnvFiles EnvFiles
	}{
		{
			name:     "sneak case single file",
			manifest: []byte("services:\n  app:\n    env_file: .env\n    public: true\n    image: okteto/vote:1"),
			EnvFiles: EnvFiles{".env"},
		},
		{
			name:     "sneak case list",
			manifest: []byte("services:\n  app:\n    env_file:\n    - .env\n    - .env2\n    image: okteto/vote:1"),
			EnvFiles: EnvFiles{".env", ".env2"},
		},
		{
			name:     "camel case single file",
			manifest: []byte("services:\n  app:\n    envFile: .env\n    image: okteto/vote:1"),
			EnvFiles: EnvFiles{".env"},
		},
		{
			name:     "camel case list",
			manifest: []byte("services:\n  app:\n    envFile:\n    - .env\n    - .env2\n    image: okteto/vote:1"),
			EnvFiles: EnvFiles{".env", ".env2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}

			if svc, ok := s.Services["app"]; ok {
				if !reflect.DeepEqual(tt.EnvFiles, svc.EnvFiles) {
					t.Fatalf("expected %v but got %v", tt.EnvFiles, svc.EnvFiles)
				}
			}
		})
	}
}

func Test_Environment(t *testing.T) {
	tests := []struct {
		name        string
		manifest    []byte
		environment Environment
	}{
		{
			name:        "envs",
			manifest:    []byte("services:\n  app:\n    environment:\n        env: production\n    image: okteto/vote:1"),
			environment: Environment{EnvVar{Name: "env", Value: "production"}},
		},
		{
			name:        "empty envs",
			manifest:    []byte("services:\n  app:\n    environment:\n      - env\n    image: okteto/vote:1"),
			environment: Environment{},
		},
		{
			name:        "empty envs - exists envar",
			manifest:    []byte("services:\n  app:\n    environment:\n        OKTETO_ENVTEST:\n    image: okteto/vote:1"),
			environment: Environment{EnvVar{Name: "OKTETO_ENVTEST", Value: "myvalue"}},
		},
		{
			name:        "empty list envs - exists envar",
			manifest:    []byte("services:\n  app:\n    environment:\n      - OKTETO_ENVTEST\n    image: okteto/vote:1"),
			environment: Environment{EnvVar{Name: "OKTETO_ENVTEST", Value: "myvalue"}},
		},
		{
			name:        "noenvs",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1"),
			environment: Environment{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			t.Setenv("OKTETO_ENVTEST", "myvalue")

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			assert.Equal(t, tt.environment, s.Services["app"].Environment)
			if len(s.Services["app"].Environment) != len(tt.environment) {
				t.Fatalf("Bad unmarshal of envs")
			}
		})
	}
}

func Test_MultipleEndpoints(t *testing.T) {
	tests := []struct {
		name          string
		manifest      []byte
		expectedStack *Stack
		svcPublic     bool
	}{
		{
			name:     "no-ports",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: false,
		},
		{
			name:     "one-port-that-should-not-be-skipped",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 8080:8080"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: true,
		},
		{
			name:     "two-port-that-should-not-be-skipped",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 8080:8080\n    - 80:8081"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: false,
		},
		{
			name:     "one-port-that-should-be-skipped",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 3306:3306"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: false,
		},
		{
			name:     "two-ports-one-skippable-and-one-not",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 8080:8080\n    - 3306:3306"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: true,
		},
		{
			name:     "three-ports-one-skippable-and-two-not",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 8080:8080\n    - 80:8081\n    - 3306:3306"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: false,
		},
		{
			name:     "two-ports-not-skippable",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    ports:\n    - 8080:8080\n    - 8081:8081"),
			expectedStack: &Stack{
				Services: map[string]*Service{
					"app": {Image: "okteto/vote:1"},
				},
				Endpoints: nil,
			},
			svcPublic: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Endpoints) != len(tt.expectedStack.Endpoints) {
				t.Fatal("The number of created endpoints is incorrect")
			}
			if !reflect.DeepEqual(s.Endpoints, tt.expectedStack.Endpoints) {
				t.Fatal("The endpoints have not been created properly")
			}
			if s.Services["app"].Public != tt.svcPublic {
				t.Fatal("Public property was not set properly")
			}
		})
	}
}

func Test_CreateJobPVCs(t *testing.T) {
	manifest := []byte(`name: test
services:
  app:
    ports:
      - 9213
    image: okteto/vote:1
    volumes:
      - /usr/var/lib
    restart: never
`)
	s, err := ReadStack(manifest, true)
	if err != nil {
		t.Fatal("Could not read stack")
	}
	if len(s.Volumes) == 0 {
		t.Fatal("PVCs not created")
	}

}

func Test_TestJobCreation(t *testing.T) {
	manifest := []byte(`name: test
services:
  app:
    image: okteto/vote:1
    deploy:
      restart_policy:
        max_attempts: 3
        condition: on-failure
`)
	s, err := ReadStack(manifest, true)
	if err != nil {
		t.Fatalf("Could not read stack: %s", err.Error())
	}
	if s.Services["app"].BackOffLimit != 3 {
		t.Fatal("Could not read the job properly")
	}
	if s.Services["app"].RestartPolicy != apiv1.RestartPolicyOnFailure {
		t.Fatal("Could not read the job properly")
	}
}

func Test_ExtensionUnmarshalling(t *testing.T) {
	tests := []struct {
		name          string
		manifest      []byte
		expected      *Service
		expectedError bool
	}{
		{
			name:     "test anchors expansion",
			manifest: []byte("x-env: &env\n  environment:\n  - SOME_ENV_VAR=123\nservices:\n  app:\n    <<: *env"),
			expected: &Service{
				Environment: Environment{
					EnvVar{
						Name:  "SOME_ENV_VAR",
						Value: "123",
					},
				},
			},
			expectedError: false,
		},
		{
			name:     "extension not anchor result in no error",
			manifest: []byte("x-endpoints:\n  path: /\n  port: 8080\n  service: app\nservices:\n  app:\n    command: bash"),
			expected: &Service{
				Command: Command{
					Values: []string{"bash"},
				},
				Environment: Environment{},
			},
			expectedError: false,
		},
		{
			name:          "not valid first class field",
			manifest:      []byte("test:\n  hello-world: echo\nservices:\n  app:\n    healthcheck:\n      interval: 10s\n      timeout: 10m\n      retries: 5\n      start_period: 30s\n      test: [\"NONE\"]\n    image: okteto/vote:1"),
			expected:      nil,
			expectedError: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := ReadStack(tt.manifest, true)
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			} else if err == nil && tt.expectedError {
				t.Fatal("error not thrown")
			}
			if !tt.expectedError {
				assert.Equal(t, tt.expected.Environment, s.Services["app"].Environment)
			}

		})
	}
}

func Test_UnmarshalStackUser(t *testing.T) {
	tests := []struct {
		name          string
		manifest      []byte
		errorExpected bool
		expected      *StackSecurityContext
	}{
		{
			name:     "expanded form",
			manifest: []byte("runAsUser: 1000\nrunAsGroup: 2000"),
			expected: &StackSecurityContext{RunAsUser: pointer.Int64(1000), RunAsGroup: pointer.Int64(2000)},
		},
		{
			name:     "runAsUserOnly",
			manifest: []byte("1000"),
			expected: &StackSecurityContext{RunAsUser: pointer.Int64(1000)},
		},
		{
			name:     "runAsUser and runAsGroup",
			manifest: []byte("1000:2000"),
			expected: &StackSecurityContext{RunAsUser: pointer.Int64(1000), RunAsGroup: pointer.Int64(2000)},
		},
		{
			name:          "not a number",
			manifest:      []byte("test"),
			errorExpected: true,
			expected:      nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result *StackSecurityContext
			if err := yaml.Unmarshal([]byte(tt.manifest), &result); err != nil {
				if !tt.errorExpected {
					t.Fatalf("unexpected error unmarshaling %s: %s", tt.name, err.Error())
				}
				return
			}
			if tt.errorExpected {
				t.Fatalf("expected error unmarshaling %s not thrown", tt.name)
			}
			if !reflect.DeepEqual(result, tt.expected) {
				t.Errorf("didn't unmarshal correctly healthcheck test. Actual %v, Expected %v", result, tt.expected)
			}
		})
	}
}

func Test_TranslateOktetoStackPortsToComposePorts(t *testing.T) {
	tests := []struct {
		name     string
		ports    []PortRaw
		expected []PortRaw
	}{
		{
			name: "containerPort only",
			ports: []PortRaw{
				{
					ContainerPort: 8080,
				},
				{
					ContainerPort: 3000,
				},
			},
			expected: []PortRaw{
				{
					ContainerPort: 8080,
					HostPort:      8080,
				},
				{
					ContainerPort: 3000,
					HostPort:      3000,
				},
			},
		},
		{
			name: "hostPort:containerPort only",
			ports: []PortRaw{
				{
					ContainerPort: 8080,
					HostPort:      8081,
				},
				{
					ContainerPort: 3000,
					HostPort:      2345,
				},
			},
			expected: []PortRaw{
				{
					ContainerPort: 8080,
					HostPort:      8081,
				},
				{
					ContainerPort: 3000,
					HostPort:      2345,
				},
			},
		},
		{
			name: "hostPort:containerPort and containerPort",
			ports: []PortRaw{
				{
					ContainerPort: 8080,
				},
				{
					ContainerPort: 3000,
					HostPort:      2345,
				},
			},
			expected: []PortRaw{
				{
					ContainerPort: 8080,
					HostPort:      8080,
				},
				{
					ContainerPort: 3000,
					HostPort:      2345,
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := translateOktetoStacksPortsIntoComposeSyntax(tt.ports)
			assert.Equal(t, tt.expected, result)
		})
	}
}
