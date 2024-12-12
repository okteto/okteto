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

func Test_Deploy(t *testing.T) {
	tests := []struct {
		name      string
		manifest  string
		expectErr bool
	}{
		{
			name: "empty",
			manifest: `
deploy: {}`,
		},
		{
			name: "array",
			manifest: `
deploy:
  - kubectl apply -f k8s.yaml
  - some other command`,
		},
		{
			name: "array within commands",
			manifest: `
deploy:
  commands:
    - kubectl apply -f k8s.yaml
    - some other command`,
		},
		{
			name: "array of objects within commands",
			manifest: `
deploy:
  commands:
    - name: Deploy app
      command: kubectl apply -f manifests
    - name: Other
      command: some other command`,
		},
		{
			name: "deploy with compose",
			manifest: `
deploy:
  compose: docker-compose.yml`,
		},
		{
			name: "deploy with extended compose notation",
			manifest: `
deploy:
  compose:
    - file: docker-compose.yml
      services:
        - frontend
    - file: docker-compose.dev.yml
      services:
        - api
  endpoints:
    - path: /
      service: frontend
      port: 80
    - path: /api
      service: api
      port: 8080`,
		},
		{
			name: "compose and commands combined",
			manifest: `
deploy:
  commands:
    - helm upgrade --install movies chart --set api.image=${OKTETO_BUILD_API_IMAGE} --set frontend.image=${OKTETO_BUILD_FRONTEND_IMAGE}
  compose: docker-compose.yml`,
		},
		{
			name: "deploy with divert",
			manifest: `
deploy:
  commands:
    - helm upgrade --install movies chart --set api.image=${OKTETO_BUILD_API_IMAGE} --set frontend.image=${OKTETO_BUILD_FRONTEND_IMAGE}
  divert:
    driver: istio
    virtualServices:
      - name: vs1
        namespace: staging
        routes:
          - route1
          - route2
    hosts:
      - virtualService: frontend
        namespace: staging`,
		},
		{
			name: "full deploy",
			manifest: `
deploy:
  image: okteto/deploy:latest
  remote: true
  commands:
    - name: Deploy database
      command: helm install postgresql
    - name: Deploy app
      command: kubectl apply -f k8s
  compose: docker-compose.yml
  divert:
    driver: istio
    virtualServices:
      - name: frontend
        namespace: test
    hosts:
      - virtualService: frontend
        namespace: test
  endpoints:
    - path: /
      service: frontend
      port: 80`,
		},
		{
			name: "invalid commands type",
			manifest: `
deploy:
  commands: "invalid"`,
			expectErr: true,
		},
		{
			name: "invalid divert driver",
			manifest: `
deploy:
  divert:
    driver: invalid`,
			expectErr: true,
		},
		{
			name: "invalid endpoints type",
			manifest: `
deploy:
  endpoints: "invalid"`,
			expectErr: true,
		},
		{
			name: "additional properties",
			manifest: `
deploy:
  invalid: value`,
			expectErr: true,
		},
		{
			name: "array and remote",
			manifest: `
deploy:
  - kubectl apply -f k8s.yaml
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

func Test_Deploy_JsonSchema_IsAlignedWithManifest(t *testing.T) {
	oktetoJsonSchema := NewJsonSchema()

	b, err := json.Marshal(oktetoJsonSchema)
	assert.NoError(t, err)

	var s map[string]interface{}
	err = json.Unmarshal(b, &s)
	assert.NoError(t, err)

	deploySchema := s["properties"].(map[string]interface{})["deploy"].(map[string]interface{})
	deployOneOf := deploySchema["oneOf"].([]interface{})
	var deployProperties map[string]interface{}
	for _, schema := range deployOneOf {
		schemaMap := schema.(map[string]interface{})
		if schemaType, ok := schemaMap["type"].(string); ok && schemaType == "object" {
			deployProperties = schemaMap["properties"].(map[string]interface{})
			break
		}
	}

	deployPropKeys := make([]string, 0, len(deployProperties))
	for k := range deployProperties {
		deployPropKeys = append(deployPropKeys, k)
	}

	manifestKeys := model.GetStructKeys(model.Manifest{})
	assert.ElementsMatch(t, manifestKeys["model.DeployInfo"], deployPropKeys, "JSON Schema Deploy section should match Manifest Deploy section")
}
