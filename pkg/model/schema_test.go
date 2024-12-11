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
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_mergeUnique(t *testing.T) {
	tests := []struct {
		name     string
		s1       []string
		s2       []string
		expected []string
	}{
		{
			name: "empty slices",
		},
		{
			name:     "empty s2",
			s1:       []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "empty s1",
			s2:       []string{"a"},
			expected: []string{"a"},
		},
		{
			name:     "unique slices",
			s1:       []string{"a"},
			s2:       []string{"b"},
			expected: []string{"a", "b"},
		},
		{
			name:     "successfully removing duplicates",
			s1:       []string{"a", "a", "b", "c", "c"},
			s2:       []string{"c", "c", "b", "a"},
			expected: []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeAndSortUnique(tt.s1, tt.s2)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func Test_getStructKeys(t *testing.T) {
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
				"build.Info":                        {"secrets", "context", "dockerfile", "target", "image", "cache_from", "args", "export_cache", "depends_on"},
				"build.VolumeMounts":                {"local_path", "remote_path"},
				"deps.Dependency":                   {"repository", "manifest", "branch", "variables", "timeout", "wait"},
				"env.Var":                           {"name", "value"},
				"externalresource.ExternalResource": {"icon", "notes", "endpoints"},
				"forward.Forward":                   {"labels", "name", "localPort", "remotePort"},
				"forward.GlobalForward":             {"labels", "name", "localPort", "remotePort"},
				"model.Artifact":                    {"path", "destination"},
				"model.Capabilities":                {"add", "drop"},
				"model.ComposeInfo":                 {"file", "services"},
				"model.ComposeSectionInfo":          {"manifest"},
				"model.DeployCommand":               {"name", "command"},
				"model.DeployInfo":                  {"compose", "endpoints", "divert", "image", "commands", "remote", "context"},
				"model.DestroyInfo":                 {"image", "commands", "remote", "context"},
				"model.Dev":                         {"resources", "selector", "persistentVolume", "securityContext", "probes", "nodeSelector", "metadata", "affinity", "image", "lifecycle", "replicas", "initContainer", "workdir", "name", "container", "serviceAccount", "priorityClassName", "interface", "mode", "imagePullPolicy", "tolerations", "command", "forward", "reverse", "externalVolumes", "secrets", "volumes", "envFiles", "environment", "services", "args", "sync", "timeout", "remote", "sshServerPort", "autocreate"},
				"model.DivertDeploy":                {"driver", "namespace", "service", "deployment", "virtualServices", "hosts", "port"},
				"model.DivertHost":                  {"virtualService", "namespace"},
				"model.DivertVirtualService":        {"name", "namespace", "routes"},
				"model.HealthCheck":                 {"http", "test", "interval", "timeout", "retries", "start_period", "disable", "x-okteto-liveness", "x-okteto-readiness"},
				"model.Host":                        {"hostname", "ip"},
				"model.HTTPHealtcheck":              {"path", "port"},
				"model.InitContainer":               {"resources", "image"},
				"model.Lifecycle":                   {"postStart", "preStop"},
				"model.LifecycleHandler":            {"command", "enabled"},
				"model.Manifest":                    {"name", "icon", "dev", "build", "deploy", "destroy", "dependencies", "external", "forward", "test"},
				"model.Metadata":                    {"labels", "annotations"},
				"model.PersistentVolumeInfo":        {"accessMode", "volumeMode", "annotations", "labels", "storageClass", "size", "enabled"},
				"model.Probes":                      {"liveness", "readiness", "startup"},
				"model.ResourceRequirements":        {"limits", "requests"},
				"model.SecurityContext":             {"runAsUser", "runAsGroup", "fsGroup", "capabilities", "runAsNonRoot", "allowPrivilegeEscalation"},
				"model.Service":                     {"healthcheck", "labels", "resources", "x-node-selector", "user", "depends_on", "build", "workdir", "image", "restart", "environment", "ports", "volumes", "cap_add", "cap_drop", "env_file", "command", "annotations", "entrypoint", "stop_grace_period", "replicas", "max_attempts", "public"},
				"model.ServiceResources":            {"cpu", "memory", "storage"},
				"model.Stack":                       {"volumes", "services", "endpoints", "name", "namespace", "context"},
				"model.StackResources":              {"limits", "requests"},
				"model.StackSecurityContext":        {"runAsUser", "runAsGroup"},
				"model.StorageResource":             {"size", "class"},
				"model.Sync":                        {"folders", "rescanInterval", "compression", "verbose"},
				"model.SyncFolder":                  {"localPath", "remotePath"},
				"model.Test":                        {"image", "context", "commands", "depends_on", "caches", "artifacts", "hosts", "skipIfNoFileChanges"},
				"model.TestCommand":                 {"name", "command"},
				"model.Timeout":                     {"default", "resources"},
				"model.VolumeSpec":                  {"labels", "annotations", "size", "class"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := GetStructKeys(tt.input)

			for k, v := range keys {
				assert.ElementsMatch(t, tt.expected[k], v, "unexpected keys for %s", k)
			}

			for k, v := range tt.expected {
				assert.ElementsMatch(t, v, keys[k])
			}

			assert.Equal(t, len(tt.expected), len(keys))
		})
	}
}
