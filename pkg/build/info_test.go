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

package build

import (
	"fmt"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/okteto/okteto/pkg/cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestExpandBuildArgs(t *testing.T) {
	t.Setenv("KEY", "VALUE")
	tests := []struct {
		buildInfo          *Info
		expected           *Info
		previousImageBuilt map[string]string
		name               string
	}{
		{
			name:               "no build args",
			buildInfo:          &Info{},
			previousImageBuilt: map[string]string{},
			expected:           &Info{},
		},
		{
			name: "only buildInfo without expanding",
			buildInfo: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
			previousImageBuilt: map[string]string{},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name: "only buildInfo expanding",
			buildInfo: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name:      "only previousImageBuilt",
			buildInfo: &Info{},
			previousImageBuilt: map[string]string{
				"KEY": "VALUE",
			},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
		{
			name: "buildInfo args and previousImageBuilt without expanding",
			buildInfo: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY2": "VALUE2",
			},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
					{
						Name:  "KEY2",
						Value: "VALUE2",
					},
				},
			},
		},
		{
			name: "buildInfo args and previousImageBuilt expanding",
			buildInfo: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY2": "VALUE2",
			},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
					{
						Name:  "KEY2",
						Value: "VALUE2",
					},
				},
			},
		},
		{
			name: "buildInfo args only same as previousImageBuilt",
			buildInfo: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "$KEY",
					},
				},
			},
			previousImageBuilt: map[string]string{
				"KEY": "VALUE",
			},
			expected: &Info{
				Args: Args{
					{
						Name:  "KEY",
						Value: "VALUE",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NoError(t, tt.buildInfo.AddArgs(tt.previousImageBuilt))

			assert.Equal(t, tt.expected, tt.buildInfo)
		})
	}
}

func Test_BuildInfoCopy(t *testing.T) {
	b := &Info{
		Context:     "context",
		Dockerfile:  "dockerfile",
		Target:      "target",
		Image:       "image",
		CacheFrom:   []string{"cache"},
		ExportCache: []string{"export"},
		Args: Args{
			Arg{
				Name:  "env",
				Value: "test",
			},
		},
		Secrets: Secrets{
			"sec": "test",
		},
		VolumesToInclude: []VolumeMounts{
			{
				LocalPath:  "local",
				RemotePath: "remote",
			},
		},
		DependsOn: DependsOn{"other"},
	}

	copyB := b.Copy()
	assert.EqualValues(t, b, copyB)

	samePointer := &copyB == &b
	assert.False(t, samePointer)
}

func TestSetBuildDefaults(t *testing.T) {
	info := &Info{
		Context: "",
	}
	expected := &Info{
		Context:    ".",
		Dockerfile: "Dockerfile",
	}
	info.SetBuildDefaults()
	require.Equal(t, info, expected)
}

func TestUnmarshalInfo(t *testing.T) {
	t.Setenv("CONTEXT", "testContext")
	t.Setenv("DOCKERFILE", "dockerfile")
	tests := []struct {
		input       string
		expected    *Info
		name        string
		expectedErr bool
	}{
		{
			name:  "unmarshal string",
			input: "an string value",
			expected: &Info{
				Context: "an string value",
			},
		},
		{
			name: "unmarshal struct",
			input: `
context: testContext
dockerfile: dockerfile
target: testTarget
image: testImage
cache_from:
  - test_cache_from
export_cache:
  - test_export_cache
depends_on:
  - test_depends_on
secrets:
  secretName: secretValue`,
			expected: &Info{
				Context:    "testContext",
				Dockerfile: "dockerfile",
				Target:     "testTarget",
				Image:      "testImage",
				CacheFrom: cache.From{
					"test_cache_from",
				},
				ExportCache: cache.ExportCache{
					"test_export_cache",
				},
				DependsOn: DependsOn{
					"test_depends_on",
				},
				Secrets: Secrets{
					"secretName": "secretValue",
				},
			},
		},
		{
			name: "unmarshal struct with expansion",
			input: `
context: $CONTEXT
dockerfile: $DOCKERFILE
target: testTarget
image: testImage
cache_from:
  - test_cache_from
export_cache:
  - test_export_cache
depends_on:
  - test_depends_on
secrets:
  secretName: secretValue`,
			expected: &Info{
				Context:    "testContext",
				Dockerfile: "dockerfile",
				Target:     "testTarget",
				Image:      "testImage",
				CacheFrom: cache.From{
					"test_cache_from",
				},
				ExportCache: cache.ExportCache{
					"test_export_cache",
				},
				DependsOn: DependsOn{
					"test_depends_on",
				},
				Secrets: Secrets{
					"secretName": "secretValue",
				},
			},
		},
		{
			name:        "error unmarshal string nor struct",
			input:       "- an string value as list",
			expectedErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := &Info{}
			err := yaml.Unmarshal([]byte(tt.input), out)
			if tt.expectedErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tt.expected, out)
			}

		})
	}
}

func TestMarshalInfo(t *testing.T) {
	tests := []struct {
		expected string
		input    *Info
		name     string
	}{
		{
			name:     "unmarshal string",
			expected: "context: an string value\n",
			input: &Info{
				Context: "an string value",
			},
		},
		{
			name:     "unmarshal info with dockerfile",
			expected: "dockerfile: an string value\n",
			input: &Info{
				Dockerfile: "an string value",
			},
		},
		{
			name:     "unmarshal info with target",
			expected: "target: an string value\n",
			input: &Info{
				Target: "an string value",
			},
		},
		{
			name:     "unmarshal info with args",
			expected: "args:\n- name: testName\n  value: testValue\n",
			input: &Info{
				Args: Args{
					{
						Name:  "testName",
						Value: "testValue",
					},
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := yaml.Marshal(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, string(out))
		})
	}
}

func Test_expandSecrets(t *testing.T) {
	homeEnvVar := "HOME"
	if runtime.GOOS == "windows" {
		homeEnvVar = "USERPROFILE"
	}
	t.Setenv(homeEnvVar, filepath.Clean("/home/testuser"))

	tests := []struct {
		input       *Info
		expected    *Info
		setEnvFunc  func(t *testing.T)
		name        string
		expectedErr bool
	}{
		{
			name:     "no secrets",
			input:    &Info{},
			expected: &Info{},
		},
		{
			name: "successfully expand home directory",
			input: &Info{Secrets: map[string]string{
				"path": "~/secret",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": filepath.Clean("/home/testuser/secret"),
			}},
		},
		{
			name: "only replace initial tilde-slash",
			input: &Info{Secrets: map[string]string{
				"path": "~/test/~/secret",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": filepath.Clean("/home/testuser/test/~/secret"),
			}},
		},
		{
			name: "no expansion needed",
			input: &Info{Secrets: map[string]string{
				"path": "/var/log",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": "/var/log",
			}},
		},
		{
			name: "expand HOME env var",
			input: &Info{Secrets: map[string]string{
				"path": filepath.Join(fmt.Sprintf("$%s", homeEnvVar), "secrets"),
			}},
			expected: &Info{Secrets: map[string]string{
				"path": filepath.Clean("/home/testuser/secrets"),
			}},
			setEnvFunc: func(t *testing.T) {
				t.Setenv("TEST_RANDOM_DIR", "/home/testuser")
			},
		},
		{
			name: "expand unset env var",
			input: &Info{Secrets: map[string]string{
				"path": "$TEST_RANDOM_DIR/secrets",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": "/secrets",
			}},
		},
		{
			name: "empty - unset env var",
			input: &Info{Secrets: map[string]string{
				"path": "$TEST_RANDOM_DIR",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": "",
			}},
		},
		{
			name: "broken env var",
			input: &Info{Secrets: map[string]string{
				"path": "${TEST_RANDOM_DIR/secrets",
			}},
			expected: &Info{Secrets: map[string]string{
				"path": "",
			}},
			expectedErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.setEnvFunc != nil {
				tc.setEnvFunc(t)
			}
			err := tc.input.expandSecrets()
			if tc.expectedErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, tc.expected, tc.input)
		})
	}
}

func TestAddArgsWithDependsOnFiltering(t *testing.T) {
	tests := []struct {
		name              string
		buildInfo         *Info
		previousImageArgs map[string]string
		expectedArgsCount int
		expectedArgs      []string
		shouldNotContain  []string
	}{
		{
			name: "adds build args for services in depends_on",
			buildInfo: &Info{
				DependsOn: DependsOn{"api", "frontend"},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_API_IMAGE":      "registry.com/api:latest",
				"OKTETO_BUILD_API_REGISTRY":   "registry.com",
				"OKTETO_BUILD_FRONTEND_IMAGE": "registry.com/frontend:latest",
			},
			expectedArgsCount: 3,
			expectedArgs: []string{
				"OKTETO_BUILD_API_IMAGE",
				"OKTETO_BUILD_API_REGISTRY",
				"OKTETO_BUILD_FRONTEND_IMAGE",
			},
		},
		{
			name: "skips build args for services not in depends_on",
			buildInfo: &Info{
				DependsOn: DependsOn{"api"},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_API_IMAGE":      "registry.com/api:latest",
				"OKTETO_BUILD_FRONTEND_IMAGE": "registry.com/frontend:latest",
				"OKTETO_BUILD_DATABASE_IMAGE": "registry.com/database:latest",
			},
			expectedArgsCount: 1,
			expectedArgs: []string{
				"OKTETO_BUILD_API_IMAGE",
			},
			shouldNotContain: []string{
				"OKTETO_BUILD_FRONTEND_IMAGE",
				"OKTETO_BUILD_DATABASE_IMAGE",
			},
		},
		{
			name: "handles services with hyphens in depends_on",
			buildInfo: &Info{
				DependsOn: DependsOn{"svc-1", "my-service"},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_SVC_1_IMAGE":      "registry.com/svc-1:latest",
				"OKTETO_BUILD_SVC_1_REGISTRY":   "registry.com",
				"OKTETO_BUILD_MY_SERVICE_IMAGE": "registry.com/my-service:latest",
				"OKTETO_BUILD_OTHER_IMAGE":      "registry.com/other:latest",
			},
			expectedArgsCount: 3,
			expectedArgs: []string{
				"OKTETO_BUILD_SVC_1_IMAGE",
				"OKTETO_BUILD_SVC_1_REGISTRY",
				"OKTETO_BUILD_MY_SERVICE_IMAGE",
			},
			shouldNotContain: []string{
				"OKTETO_BUILD_OTHER_IMAGE",
			},
		},
		{
			name: "adds non-build environment variables regardless of depends_on",
			buildInfo: &Info{
				DependsOn: DependsOn{"api"},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_API_IMAGE":   "registry.com/api:latest",
				"OKTETO_BUILD_OTHER_IMAGE": "registry.com/other:latest",
				"REGULAR_ENV_VAR":          "some-value",
				"ANOTHER_ENV_VAR":          "another-value",
			},
			expectedArgsCount: 3,
			expectedArgs: []string{
				"OKTETO_BUILD_API_IMAGE",
				"REGULAR_ENV_VAR",
				"ANOTHER_ENV_VAR",
			},
			shouldNotContain: []string{
				"OKTETO_BUILD_OTHER_IMAGE",
			},
		},
		{
			name: "empty depends_on skips all build args",
			buildInfo: &Info{
				DependsOn: DependsOn{},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_API_IMAGE":      "registry.com/api:latest",
				"OKTETO_BUILD_FRONTEND_IMAGE": "registry.com/frontend:latest",
				"REGULAR_ENV_VAR":             "some-value",
			},
			expectedArgsCount: 1,
			expectedArgs: []string{
				"REGULAR_ENV_VAR",
			},
			shouldNotContain: []string{
				"OKTETO_BUILD_API_IMAGE",
				"OKTETO_BUILD_FRONTEND_IMAGE",
			},
		},
		{
			name: "handles all build env var types",
			buildInfo: &Info{
				DependsOn: DependsOn{"api"},
			},
			previousImageArgs: map[string]string{
				"OKTETO_BUILD_API_REGISTRY":   "registry.com",
				"OKTETO_BUILD_API_REPOSITORY": "myorg/api",
				"OKTETO_BUILD_API_IMAGE":      "registry.com/myorg/api:latest",
				"OKTETO_BUILD_API_TAG":        "latest",
				"OKTETO_BUILD_API_SHA":        "abc123",
			},
			expectedArgsCount: 5,
			expectedArgs: []string{
				"OKTETO_BUILD_API_REGISTRY",
				"OKTETO_BUILD_API_REPOSITORY",
				"OKTETO_BUILD_API_IMAGE",
				"OKTETO_BUILD_API_TAG",
				"OKTETO_BUILD_API_SHA",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.buildInfo.AddArgs(tt.previousImageArgs)
			require.NoError(t, err)

			// Check expected args count
			assert.Equal(t, tt.expectedArgsCount, len(tt.buildInfo.Args))

			// Check that expected args are present
			argNames := make([]string, len(tt.buildInfo.Args))
			for i, arg := range tt.buildInfo.Args {
				argNames[i] = arg.Name
			}

			for _, expectedArg := range tt.expectedArgs {
				assert.Contains(t, argNames, expectedArg, "Expected arg %s not found", expectedArg)
			}

			// Check that unwanted args are not present
			for _, unwantedArg := range tt.shouldNotContain {
				assert.NotContains(t, argNames, unwantedArg, "Unwanted arg %s found", unwantedArg)
			}
		})
	}
}

func TestNormalizeServiceName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"API", "api"},
		{"SVC_1", "svc-1"},
		{"MY_SERVICE", "my-service"},
		{"FRONTEND", "frontend"},
		{"BACK_END_SERVICE", "back-end-service"},
		{"SERVICE_WITH_MULTIPLE_UNDERSCORES", "service-with-multiple-underscores"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeServiceName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsServiceInDependencies(t *testing.T) {
	info := &Info{
		DependsOn: DependsOn{"api", "svc-1", "my-service"},
	}

	tests := []struct {
		serviceName string
		expected    bool
	}{
		{"API", true},
		{"api", true},
		{"SVC_1", true},
		{"svc-1", true},
		{"MY_SERVICE", true},
		{"my-service", true},
		{"FRONTEND", false},
		{"frontend", false},
		{"DATABASE", false},
		{"database", false},
	}

	for _, tt := range tests {
		t.Run(tt.serviceName, func(t *testing.T) {
			result := info.isServiceInDependencies(tt.serviceName)
			assert.Equal(t, tt.expected, result)
		})
	}
}
