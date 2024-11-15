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
		wantError bool
	}{
		{
			name: "basic build",
			manifest: `
build:
  base:
    context: .`,
			wantError: false,
		},
		{
			name: "full build",
			manifest: `
build:
  api:
    context: api
    dockerfile: Dockerfile.api
    target: dev
    image: my-registry/api:latest
    args:
      VERSION: v1.0.0
      DEBUG: "true"
    secrets:
      npmrc: .npmrc`,
			wantError: false,
		},
		{
			name: "build with dependencies",
			manifest: `
build:
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
      SOURCE_IMAGE: ${OKTETO_BUILD_BASE_IMAGE}
    secrets:
      npmrc: .npmrc`,
			wantError: false,
		},
		{
			name: "build with environment variables",
			manifest: `
build:
  api:
    context: api
    args:
      VERSION: ${VERSION}
      DEBUG: ${DEBUG:-false}
      PORT: "8080"`,
			wantError: false,
		},
		//		{
		//			name: "invalid context type",
		//			manifest: `
		//build:
		//  api:
		//    context: 123`,
		//			wantError: true,
		//		},
		{
			name: "invalid args type",
			manifest: `
build:
  api:
    context: .
    args: "invalid"`,
			wantError: true,
		},
		//		{
		//			name: "invalid args value type",
		//			manifest: `
		//build:
		//  api:
		//    context: .
		//    args:
		//      PORT: 8080`,
		//			wantError: true,
		//		},
		{
			name: "invalid depends_on type",
			manifest: `
build:
  api:
    context: .
    depends_on: "base"`,
			wantError: true,
		},
		{
			name: "invalid secrets type",
			manifest: `
build:
  api:
    context: .
    secrets: "invalid"`,
			wantError: true,
		},
		//		{
		//			name: "invalid secrets value type",
		//			manifest: `
		//build:
		//  api:
		//    context: .
		//    secrets:
		//      npmrc: 123`,
		//			wantError: true,
		//		},
		{
			name: "empty build",
			manifest: `
build: {}`,
			wantError: false,
		},
		{
			name: "build with default values",
			manifest: `
build:
  api: {}`,
			wantError: false,
		},
		{
			name: "additional properties",
			manifest: `
build:
  api:
    context: .
    invalid: value`,
			wantError: true,
		},
		//		{
		//			name: "invalid target type",
		//			manifest: `
		//build:
		//  api:
		//    context: .
		//    target: 123`,
		//			wantError: true,
		//		},
		//		{
		//			name: "invalid image type",
		//			manifest: `
		//build:
		//  api:
		//    context: .
		//    image: 123`,
		//			wantError: true,
		//		},
		//		{
		//			name: "invalid dockerfile type",
		//			manifest: `
		//build:
		//  api:
		//    context: .
		//    dockerfile: 123`,
		//			wantError: true,
		//		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateOktetoManifest(tt.manifest)
			if tt.wantError {
				assert.Error(t, err)
				assert.ErrorIs(t, assert.AnError, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func Test_Build_Defaults(t *testing.T) {
	manifest := `
build:
  api: {}`

	var data interface{}
	err := Unmarshal([]byte(manifest), &data)
	assert.NoError(t, err)

	schema := build{}.JSONSchema()
	schemaData, err := json.Marshal(schema)
	assert.NoError(t, err)

	var schemaMap map[string]interface{}
	err = json.Unmarshal(schemaData, &schemaMap)
	assert.NoError(t, err)

	// Verify default values
	props := schemaMap["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})["properties"].(map[string]interface{})

	contextProp := props["context"].(map[string]interface{})
	assert.Equal(t, ".", contextProp["default"])

	dockerfileProp := props["dockerfile"].(map[string]interface{})
	assert.Equal(t, "Dockerfile", dockerfileProp["default"])
}

func Test_Build_Required(t *testing.T) {
	schema := build{}.JSONSchema()
	schemaData, err := json.Marshal(schema)
	assert.NoError(t, err)

	var schemaMap map[string]interface{}
	err = json.Unmarshal(schemaData, &schemaMap)
	assert.NoError(t, err)

	// Verify no required fields at root level
	_, hasRequired := schemaMap["required"]
	assert.False(t, hasRequired)

	// Verify no required fields for each build entry
	props := schemaMap["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})
	_, hasRequired = props["required"]
	assert.False(t, hasRequired)
}

func Test_Build_Types(t *testing.T) {
	schema := build{}.JSONSchema()
	schemaData, err := json.Marshal(schema)
	assert.NoError(t, err)

	var schemaMap map[string]interface{}
	err = json.Unmarshal(schemaData, &schemaMap)
	assert.NoError(t, err)

	// Verify root type is object
	assert.Equal(t, []interface{}{"object"}, schemaMap["type"])

	// Get properties
	props := schemaMap["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})["properties"].(map[string]interface{})

	// Verify property types
	tests := map[string][]string{
		"args":       {"object"},
		"context":    {"string"},
		"depends_on": {"array"},
		"dockerfile": {"string"},
		"image":      {"string"},
		"secrets":    {"object"},
		"target":     {"string"},
	}

	for prop, expectedType := range tests {
		propMap := props[prop].(map[string]interface{})
		assert.Equal(t, expectedType, propMap["type"].(map[string]interface{})["types"])
	}
}

func Test_Build_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema, err := NewJsonSchema().ToJSON()
	assert.NoError(t, err)

	var jsonSchema map[string]interface{}
	err = json.Unmarshal(oktetoJsonSchema, &jsonSchema)
	assert.NoError(t, err)

	buildProps := jsonSchema["properties"].(map[string]interface{})["build"].(map[string]interface{})
	buildPatternProps := buildProps["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})
	buildProperties := buildPatternProps["properties"].(map[string]interface{})

	manifestKeys := model.GetStructKeys(model.Manifest{})

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
	for _, key := range manifestKeys["build.Info"] {
		if !ignoredProps[key] {
			filteredManifestKeys = append(filteredManifestKeys, key)
		}
	}
	assert.ElementsMatch(t, filteredManifestKeys, buildPropKeys, "Build schema properties should match Manifest struct fields")
}
