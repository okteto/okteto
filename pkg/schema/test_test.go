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

package schema

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_Test(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
test: {}`,
		},
		{
			name: "valid minimal test",
			manifest: `
test:
  unit:
    commands:
      - go test ./...`,
		},
		{
			name: "valid full test",
			manifest: `
test:
  unit:
    image: okteto/golang:1
    artifacts:
      - coverage.out
    caches:
      - /go
      - /root/.cache
    commands:
      - "go test . -v"

  integration:
    depends_on:
      - unit
    image: okteto/golang:1
    context: integration
    commands:
      - make tests
    hosts:
      - "localhost:127.0.0.1"`,
		},
		{
			name: "valid command formats",
			manifest: `
test:
  mixed:
    commands:
      - simple command
      - name: named command
        command: echo test`,
		},
		{
			name: "valid hosts formats",
			manifest: `
test:
  hosts:
    commands: [echo test]
    hosts:
      - simple.host:127.0.0.1
      - hostname: detailed.host
        ip: 10.0.0.1`,
		},
		{
			name: "invalid - missing commands",
			manifest: `
test:
  invalid:
    image: golang:1.20`,
			expectErr: true,
		},
		{
			name: "invalid - additional properties",
			manifest: `
test:
  invalid:
    commands: [echo test]
    invalid: true`,
			expectErr: true,
		},
		{
			name: "invalid - malformed hosts",
			manifest: `
test:
  invalid:
    commands: [echo test]
    hosts:
      - invalid:host:format`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOktetoManifest(tt.manifest)
			if tt.expectErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func manifestWithProfiles(profiles string) string {
	return fmt.Sprintf(`
test:
  unit:
    profiles: %s
    commands: [echo test]`, profiles)
}

func Test_TestProfiles(t *testing.T) {
	require.NoError(t, validateOktetoManifest(manifestWithProfiles("[pre-commit, release, a, Pre-Commit, e2e_v1.2]")))
}

func Test_TestRejectsInvalidProfiles(t *testing.T) {
	tests := []struct {
		name     string
		profiles string
	}{
		{
			name:     "not a list",
			profiles: "pre-commit",
		},
		{
			name:     "empty",
			profiles: `[""]`,
		},
		{
			name:     "blank",
			profiles: `["  "]`,
		},
		{
			name:     "leading space",
			profiles: `[" pre-commit"]`,
		},
		{
			name:     "comma",
			profiles: `["pre-commit,release"]`,
		},
		{
			name:     "space",
			profiles: `["long running"]`,
		},
		{
			name:     "starts with a dash",
			profiles: `["-release"]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Error(t, validateOktetoManifest(manifestWithProfiles(tt.profiles)))
		})
	}
}

func Test_Test_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema := NewJsonSchema()

	b, err := json.Marshal(oktetoJsonSchema)
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(b, &s)
	assert.NoError(t, err)

	testSchema := s["properties"].(map[string]interface{})["test"].(map[string]interface{})
	testPatternProps := testSchema["patternProperties"].(map[string]interface{})
	testProperties := testPatternProps[".*"].(map[string]interface{})["properties"].(map[string]interface{})

	testPropKeys := make([]string, 0, len(testProperties))
	for k := range testProperties {
		testPropKeys = append(testPropKeys, k)
	}

	manifestKeys := model.GetStructKeys(model.Manifest{})
	assert.ElementsMatch(t, manifestKeys["model.Test"], testPropKeys, "JSON Schema Test section should match Manifest Test section")
}
