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
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/okteto/okteto/pkg/cache"
	"github.com/spf13/afero"
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

func TestBuildInfo_GetDockerfilePath(t *testing.T) {
	dir := t.TempDir()

	dockerfilePath := filepath.Join(dir, "Dockerfile")
	dockerfiledevPath := filepath.Join(dir, "Dockerfile.dev")
	assert.NoError(t, os.WriteFile(dockerfilePath, []byte(`FROM alpine`), 0600))
	assert.NoError(t, os.WriteFile(dockerfiledevPath, []byte(`FROM alpine`), 0600))
	tests := []struct {
		name       string
		context    string
		dockerfile string
		want       string
	}{
		{
			name:       "with-context",
			context:    dir,
			dockerfile: "Dockerfile",
			want:       filepath.Join(dir, "Dockerfile"),
		},
		{
			name:       "with-context-and-non-dockerfile",
			context:    dir,
			dockerfile: "Dockerfile.dev",
			want:       filepath.Join(dir, "Dockerfile.dev"),
		},
		{
			name:       "empty",
			context:    "",
			dockerfile: "",
			want:       "",
		},
		{
			name:       "default",
			context:    "",
			dockerfile: "Dockerfile",
			want:       "Dockerfile",
		},

		{
			name:       "with-context-and-dockerfile-expanded",
			context:    "api",
			dockerfile: "api/Dockerfile.dev",
			want:       "api/Dockerfile.dev",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Info{
				Context:    tt.context,
				Dockerfile: tt.dockerfile,
			}
			if got := b.GetDockerfilePath(afero.NewOsFs()); got != tt.want {
				t.Errorf("Info.GetDockerfilePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_BuildInfoCopy(t *testing.T) {
	b := &Info{
		Name:        "test",
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
				Name: "an string value",
			},
		},
		{
			name: "unmarshal struct",
			input: `
name: default
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
				Name:       "default",
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
			expected: "an string value\n",
			input: &Info{
				Name: "an string value",
			},
		},
		{
			name:     "unmarshal info with context",
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
