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
	"testing"

	"github.com/okteto/okteto/pkg/model"
	"github.com/stretchr/testify/assert"
)

func Test_Build(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty build",
			manifest: `
build: {}`,
		},
		{
			name: "basic build",
			manifest: `
build:
  base:
    context: .`,
		},
		{
			name: "full build",
			manifest: `
build:
  withImage:
    image: my-registry/base:latest
  base:
    context: . 
  api:
    context: api
    depends_on: ["base"]
  frontend:
    context: frontend
    dockerfile: Dockerfile
    target: dev
    depends_on: ["base"]
    args:
      VERSION: v1.0.0
      DEBUG: "true"
      SOURCE_IMAGE: ${OKTETO_BUILD_BASE_IMAGE}
    secrets:
      npmrc: .npmrc`,
		},
		{
			name: "invalid build type",
			manifest: `
build: invalid`,
			expectErr: true,
		},
		{
			name: "invalid args type",
			manifest: `
build:
  api:
    context: .
    args: "invalid"`,
			expectErr: true,
		},
		{
			name: "invalid depends_on type",
			manifest: `
build:
  api:
    context: .
    depends_on: "base"`,
			expectErr: true,
		},
		{
			name: "invalid secrets type",
			manifest: `
build:
  api:
    context: .
    secrets: "invalid"`,
			expectErr: true,
		},
		{
			name: "additional properties",
			manifest: `
build:
  api:
    context: .
    invalid: value`,
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

func Test_Build_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema, err := NewJsonSchema().ToJSON()
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(oktetoJsonSchema, &s)
	assert.NoError(t, err)

	buildSchema := s["properties"].(map[string]interface{})["build"].(map[string]interface{})
	buildPatternProps := buildSchema["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})
	buildProperties := buildPatternProps["properties"].(map[string]interface{})

	buildPropKeys := make([]string, 0, len(buildProperties))
	ignoredProps := map[string]bool{
		"cache_from":   true,
		"export_cache": true,
	}
	for k := range buildProperties {
		if !ignoredProps[k] {
			buildPropKeys = append(buildPropKeys, k)
		}
	}

	filteredManifestKeys := make([]string, 0)
	manifestKeys := model.GetStructKeys(model.Manifest{})
	for _, key := range manifestKeys["build.Info"] {
		if !ignoredProps[key] {
			filteredManifestKeys = append(filteredManifestKeys, key)
		}
	}
	assert.ElementsMatch(t, filteredManifestKeys, buildPropKeys, "JSON Schema Build section should match Manifest Build section")
}
