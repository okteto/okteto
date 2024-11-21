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

func Test_External(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
external: {}`,
		},
		{
			name: "external with all fields",
			manifest: `
external:
  db:
    notes: docs/database.md
    icon: database
    endpoints:
    - name: db
      url: https://localhost:3306
  functions:
    notes: docs/lambdas.md
    icon: function
    endpoints:
    - name: data-aggregator
      url: https://fake-id.lambda-url.us-east-1.on.aws.aggregator
    - name: data-processor
      url: https://fake-id.lambda-url.us-east-1.on.aws.processor`,
		},
		{
			name: "invalid - additional properties",
			manifest: `
external:
  database:
    endpoints:
      - name: admin
        url: "https://database.com"
        invalid: true`,
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

func Test_External_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema, err := NewJsonSchema().ToJSON()
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(oktetoJsonSchema, &s)
	assert.NoError(t, err)

	externalSchema := s["properties"].(map[string]interface{})["external"].(map[string]interface{})
	externalPatternProperties := externalSchema["patternProperties"].(map[string]interface{})
	externalProperties := externalPatternProperties[".*"].(map[string]interface{})["properties"].(map[string]interface{})

	externalPropKeys := make([]string, 0, len(externalProperties))
	for k := range externalProperties {
		externalPropKeys = append(externalPropKeys, k)
	}

	manifestKeys := model.GetStructKeys(model.Manifest{})
	assert.ElementsMatch(t, manifestKeys["externalresource.ExternalResource"], externalPropKeys, "JSON Schema External section should match Manifest External section")
}
