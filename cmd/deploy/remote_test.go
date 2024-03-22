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

package deploy

import (
	"testing"
	"time"

	"github.com/okteto/okteto/pkg/log/io"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommandFlags(t *testing.T) {
	type config struct {
		opts *Options
	}
	var tests = []struct {
		name      string
		config    config
		expected  []string
		expectErr bool
	}{
		{
			name: "no extra options",
			config: config{
				opts: &Options{
					Timeout: 2 * time.Minute,
					Name:    "test",
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name set",
			config: config{
				opts: &Options{
					Name:    "test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "name multiple words",
			config: config{
				opts: &Options{
					Name:    "this is a test",
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"this is a test\""},
		},
		{
			name: "namespace is not set",
			config: config{
				opts: &Options{
					Name:      "test",
					Namespace: "test",
					Timeout:   5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "manifest path is not set",
			config: config{
				opts: &Options{
					Name:             "test",
					ManifestPathFlag: "/hello/this/is/a/test",
					ManifestPath:     "/hello/this/is/a/test",
					Timeout:          5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "variables set",
			config: config{
				opts: &Options{
					Name: "test",
					Variables: []string{
						"a=b",
						"c=d",
					},
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\"", "--var a=\"b\" --var c=\"d\""},
		},
		{
			name: "wait is not set",
			config: config{
				opts: &Options{
					Name:    "test",
					Wait:    true,
					Timeout: 5 * time.Minute,
				},
			},
			expected: []string{"--name \"test\""},
		},
		{
			name: "multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test=multi word value"},
				},
			},
			expected: []string{"--name \"test\"", "--var test=\"multi word value\""},
		},
		{
			name: "wrong multiword var value",
			config: config{
				opts: &Options{
					Name:      "test",
					Variables: []string{"test -> multi word value"},
				},
			},
			expected:  nil,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flags, err := getCommandFlags(tt.config.opts)
			if tt.expectErr {
				require.Error(t, err)
			}
			assert.Equal(t, tt.expected, flags)
		})
	}
}

func Test_newRemoteDeployer(t *testing.T) {
	getBuildEnvVars := func() map[string]string { return nil }
	getDependencyEnvVars := func(_ environGetter) map[string]string { return nil }
	got := newRemoteDeployer(getBuildEnvVars, io.NewIOController(), getDependencyEnvVars)
	require.IsType(t, &remoteDeployer{}, got)
	require.NotNil(t, got.getBuildEnvVars)
}
