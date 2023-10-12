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

package suggest

import (
	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGetStructKeys(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected map[string][]string
	}{
		{
			name:     "int",
			input:    1,
			expected: map[string][]string{},
		},
		{
			name:     "string",
			input:    "field1",
			expected: map[string][]string{},
		},
		{
			name:     "bool",
			input:    true,
			expected: map[string][]string{},
		},
		{
			name:     "map",
			input:    map[string]string{},
			expected: map[string][]string{},
		},
		{
			name:     "pointer",
			input:    &struct{}{},
			expected: map[string][]string{},
		},
		{
			name: "map with struct values no yaml tags",
			input: map[string]struct {
				field1 string
				field2 int
			}{
				"key1": {"value1", 1},
			},
			expected: map[string][]string{},
		},
		{
			name: "map with struct values with yaml tags",
			input: map[string]struct {
				field1 string `yaml:"field1"`
				field2 int    `yaml:"field2"`
			}{
				"key1": {"value1", 1},
			},
			expected: map[string][]string{"_": []string{"field1", "field2"}},
		},
		{
			name:     "string",
			input:    "not a struct",
			expected: map[string][]string{},
		},
		{
			name:     "struct with no fields",
			input:    struct{}{},
			expected: map[string][]string{},
		},
		{
			name: "anonymous struct with fields but no yaml tags",
			input: struct {
				field1 string
				field2 string
			}{},
			expected: map[string][]string{},
		},
		{
			name: "anonymous struct with fields",
			input: struct {
				field1 string `yaml:"field1"`
				field2 string
			}{},
			expected: map[string][]string{"_": []string{"field1"}},
		},
		{
			name: "anonymous struct with nested struct",
			input: struct {
				field1 string `yaml:"field1"`
				nested struct {
					field2 string `yaml:"field2"`
				}
			}{},
			expected: map[string][]string{"_": []string{"field1", "field2"}},
		},
		{
			name: "anonymous struct with nested struct with no yaml tags",
			input: struct {
				field1 string `yaml:"field1"`
				nested struct {
					field2 string
				}
			}{},
			expected: map[string][]string{"_": []string{"field1"}},
		},
		{
			name: "anonymous struct with nested struct with pointer",
			input: struct {
				field1 string `yaml:"field1"`

				nested *struct {
					field2 string `yaml:"field2"`
				}
			}{},
			expected: map[string][]string{"_": []string{"field1", "field2"}},
		},
		{
			name: "anonymous struct with nested struct with pointer with no yaml tags",
			input: struct {
				field1 string `yaml:"field1"`
				nested *struct {
					field2 string
				}
			}{},
			expected: map[string][]string{"_": []string{"field1"}},
		},
		{
			name:  "okteto manifest",
			input: model.Manifest{},
			expected: map[string][]string{
				"forward.Forward":            []string{"localPort", "remotePort", "name", "labels"},
				"forward.GlobalForward":      []string{"localPort", "remotePort", "name", "labels"},
				"model.BuildInfo":            []string{"name", "context", "dockerfile", "cache_from", "target", "image", "export_cache", "depends_on", "secrets"},
				"model.Capabilities":         []string{"add", "drop"},
				"model.ComposeInfo":          []string{"file", "services"},
				"model.Dependency":           []string{"repository", "manifest", "branch", "wait", "timeout", "namespace"},
				"model.DeployCommand":        []string{"name", "command"},
				"model.DeployInfo":           []string{"image", "endpoints", "remote"},
				"model.DestroyInfo":          []string{"image", "remote"},
				"model.Dev":                  []string{"name", "selector", "annotations", "context", "namespace", "container", "imagePullPolicy", "workdir", "serviceAccount", "remote", "sshServerPort", "interface", "services", "initFromImage", "nodeSelector", "autocreate", "envFiles", "mode", "replicas", "healthchecks", "labels"},
				"model.DivertDeploy":         []string{"driver", "namespace", "service", "port", "deployment"},
				"model.DivertHost":           []string{"virtualService", "namespace"},
				"model.DivertVirtualService": []string{"name", "namespace", "routes"},
				"model.EnvVar":               []string{"name", "value"},
				"model.HTTPHealtcheck":       []string{"path", "port"},
				"model.HealthCheck":          []string{"test", "interval", "timeout", "retries", "start_period", "disable", "x-okteto-liveness", "x-okteto-readiness"},
				"model.InitContainer":        []string{"image"},
				"model.Lifecycle":            []string{"postStart", "postStop"},
				"model.Manifest":             []string{"name", "namespace", "context", "icon", "dev", "build", "dependencies", "external"},
				"model.Metadata":             []string{"labels", "annotations"},
				"model.PersistentVolumeInfo": []string{"enabled", "storageClass", "size"},
				"model.Probes":               []string{"liveness", "readiness", "startup"},
				"model.ResourceRequirements": []string{"limits", "requests"},
				"model.SecurityContext":      []string{"runAsUser", "runAsGroup", "fsGroup", "runAsNonRoot", "allowPrivilegeEscalation"},
				"model.Service":              []string{"cap_add", "cap_drop", "env_file", "depends_on", "image", "labels", "annotations", "x-node-selector", "restart", "stop_grace_period", "workdir", "max_attempts", "public", "replicas"},
				"model.Stack":                []string{"name", "volumes", "namespace", "context", "services", "endpoints"},
				"model.StackSecurityContext": []string{"runAsUser", "runAsGroup"},
				"model.StorageResource":      []string{"class"},
				"model.Sync":                 []string{"compression", "verbose", "rescanInterval"},
				"model.Timeout":              []string{"default", "resources"},
				"model.VolumeSpec":           []string{"labels", "annotations", "class"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := getStructKeys(tt.input)
			assert.Equal(t, tt.expected, keys)
		})
	}
}
