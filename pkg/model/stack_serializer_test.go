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
	v1 "k8s.io/api/core/v1"
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
		expected      PortRaw
		expectedError bool
	}{
		{
			name:          "singlePort",
			portRaw:       "3000",
			expected:      PortRaw{ContainerPort: 3000, Protocol: v1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "singleRange",
			portRaw:       "3000-3005",
			expected:      PortRaw{ContainerPort: 0, Protocol: v1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "singlePortForwarding",
			portRaw:       "8000:8000",
			expected:      PortRaw{HostPort: 8000, ContainerPort: 8000, Protocol: v1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "RangeForwarding",
			portRaw:       "9090-9091:8080-8081",
			expected:      PortRaw{ContainerPort: 0, Protocol: v1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "DifferentPort",
			portRaw:       "49100:22",
			expected:      PortRaw{HostPort: 49100, ContainerPort: 22, Protocol: v1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "LocalhostForwarding",
			portRaw:       "127.0.0.1:8000:8001",
			expected:      PortRaw{HostPort: 8000, ContainerPort: 8001, Protocol: v1.ProtocolTCP},
			expectedError: false,
		},
		{
			name:          "Localhost Range",
			portRaw:       "127.0.0.1:5000-5010:5000-5010",
			expected:      PortRaw{ContainerPort: 0, Protocol: v1.ProtocolTCP},
			expectedError: true,
		},
		{
			name:          "Protocol",
			portRaw:       "6060:6060/udp",
			expected:      PortRaw{HostPort: 6060, ContainerPort: 6060, Protocol: v1.ProtocolUDP},
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
	tests := []struct {
		name          string
		manifest      []byte
		expectedError bool
	}{
		{
			name:          "correct-volume",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - redpanda:/var/lib/redpanda/data\n    image: okteto/vote:1\nvolumes:\n  redpanda:\n"),
			expectedError: false,
		},
		{
			name:          "volume-not-declared-in-volumes-top-level-section",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - redpanda:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: true,
		},
		{
			name:          "volume-absolute-path",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - /var/lib/redpanda/:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: false,
		},
		{
			name:          "volume-relative-path",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - /var/lib/redpanda:/var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: false,
		},
		{
			name:          "pv",
			manifest:      []byte("services:\n  app:\n    volumes: \n    - /var/lib/redpanda/data\n    image: okteto/vote:1\n"),
			expectedError: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ReadStack(tt.manifest, true)
			if err != nil && !tt.expectedError {
				t.Fatal(err)
			}
		})
	}
}

func Test_validateIngressCreationPorts(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		isPublic bool
	}{
		{
			name:     "Public-service",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			isPublic: true,
		},
		{
			name:     "not-public-service",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    image: okteto/vote:1"),
			isPublic: false,
		},
		{
			name:     "not-public-port-but-with-assignation",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213:9213\n    image: okteto/vote:1"),
			isPublic: true,
		},
		{
			name:     "mysql-port-forwarding",
			manifest: []byte("services:\n  app:\n    ports:\n    - 3306:3306\n    image: okteto/vote:1"),
			isPublic: false,
		},
		{
			name:     "mysql-port-forwarding-and-public",
			manifest: []byte("services:\n  app:\n    ports:\n    - 3306:3306\n    image: okteto/vote:1\n    public: true"),
			isPublic: true,
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
			name:               "volume-name-with-space",
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

func Test_restartFile(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		isInList bool
	}{
		{
			name:     "no-restart-field",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    public: true\n    image: okteto/vote:1"),
			isInList: false,
		},
		{
			name:     "restart-field-always",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213\n    image: okteto/vote:1\n    restart: always"),
			isInList: false,
		},
		{
			name:     "restart-field-not-always",
			manifest: []byte("services:\n  app:\n    ports:\n    - 9213:9213\n    image: okteto/vote:1\n    restart: never"),
			isInList: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
			if len(s.Warnings) == 0 && tt.isInList {
				t.Fatalf("Expected to see a warning but there is no warning")
			}
		})
	}
}

func Test_DeployLabels(t *testing.T) {
	tests := []struct {
		name     string
		manifest []byte
		labels   Labels
	}{
		{
			name:     "deploy labels",
			manifest: []byte("services:\n  app:\n    deploy:\n      labels:\n        env: production\n    image: okteto/vote:1"),
			labels:   Labels{"env": "production"},
		},
		{
			name:     "no labels",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1"),
			labels:   Labels{},
		},
		{
			name:     "labels on service",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      env: production"),
			labels:   Labels{"env": "production"},
		},
		{
			name:     "labels on deploy and service",
			manifest: []byte("services:\n  app:\n    image: okteto/vote:1\n    labels:\n      app: main\n    deploy:\n      labels:\n        env: production\n"),
			labels:   Labels{"env": "production", "app": "main"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
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
					Labels:      Labels{"key": "value"},
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
  annotations:
    key: value
  labels:
    key: value
  rules:
  - path: /
    service: app
    port: 9213`),
			expected: EndpointSpec{
				"test": Endpoint{
					Annotations: Annotations{"key": "value"},
					Labels:      Labels{"key": "value"},
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
				"test": Endpoint{
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
			name:        "noenvs",
			manifest:    []byte("services:\n  app:\n    image: okteto/vote:1"),
			environment: Environment{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			s, err := ReadStack(tt.manifest, false)
			if err != nil {
				t.Fatal(err)
			}
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
				Endpoints: EndpointSpec{},
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
				Endpoints: EndpointSpec{},
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
				Endpoints: EndpointSpec{
					"app-8080": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8080,
							},
						},
					},
					"app-80": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8081,
							},
						},
					},
				},
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
				Endpoints: EndpointSpec{},
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
				Endpoints: EndpointSpec{},
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
				Endpoints: EndpointSpec{
					"app-8080": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8080,
							},
						},
					},
					"app-80": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8081,
							},
						},
					},
				},
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
				Endpoints: EndpointSpec{
					"app-8080": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8080,
							},
						},
					},
					"app-8081": Endpoint{
						Rules: []EndpointRule{
							{
								Path:    "/",
								Service: "app",
								Port:    8081,
							},
						},
					},
				},
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
