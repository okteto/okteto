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

func Test_Forward(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
forward: []`,
		},
		{
			name: "valid shorthand notation",
			manifest: `
forward:
  - 8080:80
  - 5432:postgres:5432`,
		},
		{
			name: "valid object notation with name",
			manifest: `
forward:
  - localPort: 8080
    remotePort: 80
    name: web`,
		},
		{
			name: "valid object notation with labels",
			manifest: `
forward:
  - localPort: 5432
    remotePort: 5432
    labels:
      app: database`,
		},
		{
			name: "invalid - both name and labels",
			manifest: `
forward:
  - localPort: 8080
    remotePort: 80
    name: web
    labels:
      app: web`,
			expectErr: true,
		},
		{
			name: "invalid - missing required fields",
			manifest: `
forward:
  - localPort: 8080`,
			expectErr: true,
		},
		{
			name: "invalid shorthand format",
			manifest: `
forward:
  - "8080:invalid:format:5432"`,
			expectErr: true,
		},
		{
			name: "invalid - additional properties",
			manifest: `
forward:
  - localPort: 8080
    remotePort: 80
    name: web
    invalid: true`,
			expectErr: true,
		},
		{
			name: "invalid shorthand notation missing second port",
			manifest: `
forward:
  - 8080:80
  - 5432:postgres`,
			expectErr: true,
		},
		{
			name: "invalid shorthand notation missing first port",
			manifest: `
forward:
  - 8080:80
  - postgres:5432`,
			expectErr: true,
		},
		{
			name: "invalid missing port",
			manifest: `
forward:
  - 8080`,
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

func Test_Forward_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema, err := NewJsonSchema().ToJSON()
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(oktetoJsonSchema, &s)
	assert.NoError(t, err)

	forwardSchema := s["properties"].(map[string]interface{})["forward"].(map[string]interface{})
	forwardItems := forwardSchema["items"].(map[string]interface{})
	forwardOneOf := forwardItems["oneOf"].([]interface{})

	var objectSchema map[string]interface{}
	for _, schema := range forwardOneOf {
		schemaMap := schema.(map[string]interface{})
		if schemaType, ok := schemaMap["type"].(string); ok && schemaType == "object" {
			objectSchema = schemaMap
			break
		}
	}
	assert.NotNil(t, objectSchema, "Object schema not found in oneOf")
	objectProperties := objectSchema["properties"].(map[string]interface{})

	forwardPropKeys := make([]string, 0, len(objectProperties))
	for k := range objectProperties {
		forwardPropKeys = append(forwardPropKeys, k)
	}

	manifestKeys := model.GetStructKeys(model.Manifest{})
	assert.ElementsMatch(t, manifestKeys["forward.Forward"], forwardPropKeys, "JSON Schema Forward section should match Manifest Forward section")
}
