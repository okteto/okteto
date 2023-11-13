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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStructKeys(t *testing.T) {
	tests := []struct {
		input    interface{}
		expected map[string][]string
		name     string
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
			expected: map[string][]string{"_": {"field1", "field2"}},
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
			expected: map[string][]string{"_": {"field1"}},
		},
		{
			name: "anonymous struct with nested struct",
			input: struct {
				field1 string `yaml:"field1"`
				nested struct {
					field2 string `yaml:"field2"`
				}
			}{},
			expected: map[string][]string{"_": {"field1", "field2"}},
		},
		{
			name: "anonymous struct with nested struct with no yaml tags",
			input: struct {
				field1 string `yaml:"field1"`
				nested struct {
					field2 string
				}
			}{},
			expected: map[string][]string{"_": {"field1"}},
		},
		{
			name: "anonymous struct with nested struct with pointer",
			input: struct {
				nested *struct {
					field2 string `yaml:"field2"`
				}
				field1 string `yaml:"field1"`
			}{},
			expected: map[string][]string{"_": {"field2", "field1"}},
		},
		{
			name: "anonymous struct with nested struct with pointer with no yaml tags",
			input: struct {
				nested *struct {
					field2 string
				}
				field1 string `yaml:"field1"`
			}{},
			expected: map[string][]string{"_": {"field1"}},
		},
		{
			name:  "okteto manifest",
			input: Manifest{},
			expected: map[string][]string{
				"forward.Forward":            {"labels", "name", "localPort", "remotePort"},
				"forward.GlobalForward":      {"labels", "name", "localPort", "remotePort"},
				"model.BuildInfo":            {"secrets", "name", "context", "dockerfile", "target", "image", "cache_from", "export_cache", "depends_on"},
				"model.Capabilities":         {"add", "drop"},
				"model.ComposeInfo":          {"file", "services"},
				"model.Dependency":           {"repository", "manifest", "branch", "namespace", "timeout", "wait"},
				"model.DeployCommand":        {"name", "command"},
				"model.DeployInfo":           {"endpoints", "image", "remote"},
				"model.DestroyInfo":          {"image", "remote"},
				"model.Dev":                  {"selector", "annotations", "labels", "nodeSelector", "replicas", "workdir", "name", "context", "namespace", "container", "serviceAccount", "interface", "mode", "imagePullPolicy", "envFiles", "services", "remote", "sshServerPort", "initFromImage", "autocreate", "healthchecks"},
				"model.DivertDeploy":         {"driver", "namespace", "service", "deployment", "port"},
				"model.DivertHost":           {"virtualService", "namespace"},
				"model.DivertVirtualService": {"name", "namespace", "routes"},
				"model.EnvVar":               {"name", "value"},
				"model.HTTPHealtcheck":       {"path", "port"},
				"model.HealthCheck":          {"test", "interval", "timeout", "retries", "start_period", "disable", "x-okteto-liveness", "x-okteto-readiness"},
				"model.InitContainer":        {"image"},
				"model.Lifecycle":            {"postStart", "postStop"},
				"model.Manifest":             {"name", "namespace", "context", "icon", "dev", "build", "dependencies", "external"},
				"model.Metadata":             {"labels", "annotations"},
				"model.PersistentVolumeInfo": {"storageClass", "size", "enabled"},
				"model.Probes":               {"liveness", "readiness", "startup"},
				"model.ResourceRequirements": {"limits", "requests"},
				"model.SecurityContext":      {"runAsUser", "runAsGroup", "fsGroup", "runAsNonRoot", "allowPrivilegeEscalation"},
				"model.Service":              {"labels", "x-node-selector", "depends_on", "workdir", "image", "restart", "cap_add", "cap_drop", "env_file", "annotations", "stop_grace_period", "replicas", "max_attempts", "public"},
				"model.Stack":                {"volumes", "services", "endpoints", "name", "namespace", "context"},
				"model.StackSecurityContext": {"runAsUser", "runAsGroup"},
				"model.StorageResource":      {"class"},
				"model.Sync":                 {"rescanInterval", "compression", "verbose"},
				"model.Timeout":              {"default", "resources"},
				"model.VolumeSpec":           {"labels", "annotations", "class"},
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

func Test_isYamlErrorWithoutLinkToDocs(t *testing.T) {
	tests := []struct {
		input    error
		name     string
		expected bool
	}{
		{
			name:     "random error",
			input:    errors.New("random error"),
			expected: false,
		},
		{
			name:     "nil",
			input:    nil,
			expected: false,
		},
		{
			name:     "yaml error with link to docs",
			input:    errors.New("yaml: some random error. See https://www.okteto.com/docs"),
			expected: false,
		},
		{
			name:     "yaml error without link to docs",
			input:    errors.New("yaml: some random error"),
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := isYamlErrorWithoutLinkToDocs(tt.input)
			assert.Equal(t, tt.expected, res)
		})
	}
}

func TestUserFriendlyError(t *testing.T) {
	tests := []struct {
		name     string
		input    error
		expected string
	}{
		{
			name:  "yaml errors with heading and link to docs",
			input: errors.New("yaml: some random error"),
			expected: `your okteto manifest is not valid, please check the following errors:
yaml: some random error
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
		{
			name:  "yaml errors with heading and link to docs",
			input: errors.New("yaml: unmarshal errors:\n  line 4: field contest not found in type model.manifestRaw"),
			expected: `your okteto manifest is not valid, please check the following errors:
     - line 4: field 'contest' is not a property of the okteto manifest. Did you mean "context"?
    Check out the okteto manifest docs at: https://www.okteto.com/docs/reference/manifest`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newManifestFriendlyError(tt.input)
			assert.Equal(t, tt.expected, err.Error())
		})
	}
}
