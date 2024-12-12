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

func Test_Destroy(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
destroy: {}`,
		},
		{
			name: "array",
			manifest: `
destroy:
  - kubectl delete -f k8s.yaml
  - some other command`,
		},
		{
			name: "array within commands",
			manifest: `
destroy:
  commands:
    - kubectl delete -f k8s.yaml
    - some other command`,
		},
		{
			name: "array of objects within commands",
			manifest: `
destroy:
  commands:
    - name: Deploy app
      command: kubectl delete -f manifests
    - name: Other
      command: some other command`,
		},
		{
			name: "invalid commands type",
			manifest: `
destroy:
  commands: "invalid"`,
			expectErr: true,
		},
		{
			name: "additional properties",
			manifest: `
destroy:
  invalid: value`,
			expectErr: true,
		},
		{
			name: "array and remote",
			manifest: `
destroy:
  - kubectl delete -f k8s.yaml
  - some other command
  remote: true`,
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

func Test_Destroy_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema := NewJsonSchema()

	b, err := json.Marshal(oktetoJsonSchema)
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(b, &s)
	assert.NoError(t, err)

	destroySchema := s["properties"].(map[string]interface{})["destroy"].(map[string]interface{})
	destroyOneOf := destroySchema["oneOf"].([]interface{})
	var destroyProperties map[string]interface{}
	for _, schema := range destroyOneOf {
		schemaMap := schema.(map[string]interface{})
		if schemaType, ok := schemaMap["type"].(string); ok && schemaType == "object" {
			destroyProperties = schemaMap["properties"].(map[string]interface{})
			break
		}
	}

	destroyPropKeys := make([]string, 0, len(destroyProperties))
	for k := range destroyProperties {
		destroyPropKeys = append(destroyPropKeys, k)
	}

	manifestKeys := model.GetStructKeys(model.Manifest{})
	assert.ElementsMatch(t, manifestKeys["model.DestroyInfo"], destroyPropKeys, "JSON Schema Destroy section should match Manifest Destroy section")
}
