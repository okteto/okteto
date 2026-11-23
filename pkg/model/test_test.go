// Copyright 2024 The Okteto Authors
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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v3"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		expectedError error
		tests         ManifestTests
		name          string
		expectAnError bool
	}{
		{
			name:          "nil",
			tests:         nil,
			expectedError: ErrNoTestsDefined,
		},
		{
			name:          "empty",
			tests:         ManifestTests{},
			expectedError: ErrNoTestsDefined,
		},
		{
			name: "one test without commands",
			tests: ManifestTests{
				"one": &Test{},
			},
			expectAnError: true,
		},
		{
			name: "one test",
			tests: ManifestTests{
				"one": &Test{
					Commands: []TestCommand{
						{Command: "echo 'hello'"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.tests.Validate()
			if tt.expectedError != nil {
				require.ErrorIs(t, err, tt.expectedError)
			} else if tt.expectAnError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}

}

func newTestWithProfiles(profiles ...string) ManifestTests {
	return ManifestTests{
		"one": &Test{
			Commands: []TestCommand{{Command: "echo 'hello'"}},
			Profiles: profiles,
		},
	}
}

func TestValidateProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles []string
	}{
		{
			name:     "lowercase names with separators",
			profiles: []string{"pre-commit", "e2e_v1.2"},
		},
		{
			name:     "single character",
			profiles: []string{"a"},
		},
		{
			name:     "uppercase letters and numbers",
			profiles: []string{"Pre-Commit", "2026"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, newTestWithProfiles(tt.profiles...).Validate())
		})
	}
}

func TestValidateRejectsInvalidProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles []string
	}{
		{
			name:     "empty",
			profiles: []string{"pre-commit", ""},
		},
		{
			name:     "blank",
			profiles: []string{"  "},
		},
		{
			name:     "ends with a space",
			profiles: []string{"pre-commit "},
		},
		{
			name:     "comma",
			profiles: []string{"pre-commit,release"},
		},
		{
			name:     "starts with a dash",
			profiles: []string{"-release"},
		},
		{
			name:     "starts with an underscore",
			profiles: []string{"_release"},
		},
		{
			name:     "starts with a dot",
			profiles: []string{".release"},
		},
		{
			name:     "non-ASCII letter",
			profiles: []string{"pré"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := newTestWithProfiles(tt.profiles...).Validate()

			require.EqualError(t, err, fmt.Sprintf("test 'one' is invalid: profile '%s' must start with a letter or a number and only contain letters, numbers, '_', '.' or '-'", tt.profiles[len(tt.profiles)-1]))
		})
	}
}

func TestReadTestProfiles(t *testing.T) {
	manifest := []byte(`
test:
  unit:
    profiles: [pre-commit, release]
    commands: ["go test ./..."]
  e2e:
    commands: ["make e2e"]`)

	m, err := Read(manifest)

	require.NoError(t, err)
	require.Equal(t, []string{"pre-commit", "release"}, m.Test["unit"].Profiles)
	require.Empty(t, m.Test["e2e"].Profiles)
}

func TestHostUnmarshalYAML(t *testing.T) {
	t.Setenv("IP", "192.179.1.1")
	tests := []struct {
		expectedError error
		expectedHost  Host
		name          string
		bytes         []byte
	}{
		{
			name:  "ipv4",
			bytes: []byte("main.example.com:192.168.1.100"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.168.1.100",
			},
		},
		{
			name:  "ipv4 with expansion",
			bytes: []byte("main.example.com:${IP}"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.179.1.1",
			},
		},
		{
			name:          "ipv4 with expansion to non-existent env var",
			bytes:         []byte("main.example.com:${NON_EXISTENT}"),
			expectedError: ErrInvalidIp,
		},
		{
			name:  "ipv6",
			bytes: []byte("main.example.com:::1"),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "::1",
			},
		},
		{
			name:          "malformed - no ip",
			bytes:         []byte("main.example.com:"),
			expectedError: ErrInvalidHostName,
		},
		{
			name:          "malformed - no hostname",
			bytes:         []byte(":192.168.1.1"),
			expectedError: ErrInvalidHostName,
		},
		{
			name:          "malformed - no separator",
			bytes:         []byte("main.example.com192.168.1.1"),
			expectedError: ErrHostMalformed,
		},
		{
			name:          "extended without ip",
			bytes:         []byte("hostname: main.example.com"),
			expectedError: ErrInvalidIp,
		},
		{
			name:          "extended without hostname",
			bytes:         []byte("ip: 192.168.1.1"),
			expectedError: ErrInvalidHostName,
		},
		{
			name: "extended",
			bytes: []byte(`hostname: main.example.com
ip: 192.168.1.1`),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.168.1.1",
			},
		},
		{
			name: "extended with expansion",
			bytes: []byte(`hostname: main.example.com
ip: ${IP}`),
			expectedHost: Host{
				Hostname: "main.example.com",
				IP:       "192.179.1.1",
			},
		},
		{
			name: "extended with expansion to non-existent env var",
			bytes: []byte(`hostname: main.example.com
ip: ${NON_EXISTENT}`),
			expectedError: ErrInvalidIp,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var host Host
			err := yaml.Unmarshal(tt.bytes, &host)
			if tt.expectedError != nil {
				assert.ErrorIs(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedHost, host)
			}
		})
	}
}
