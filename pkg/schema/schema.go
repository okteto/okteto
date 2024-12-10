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

	"github.com/kubeark/jsonschema"
)

type manifest struct {
	Deploy       deploy       `json:"deploy" jsonschema:"title=deploy,description=A list of commands to deploy your development environment. It's usually a combination of helm\\, kubectl\\, and okteto commands.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#deploy-string-optional"`
	Icon         icon         `json:"icon" jsonschema:"title=icon,description=The icon associated to your development environment in the Okteto UI.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#icon-string-optional-1"`
	Dependencies dependencies `json:"dependencies" jsonschema:"title=dependencies,description=A list of repositories you want to deploy as part of your development environment.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#dependencies-string-optional"`
	Dev          dev          `json:"dev" jsonschema:"title=dev,description=A list of development containers to define the behavior of okteto up and synchronize your code in your development environment.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#dev-object-optional"`
	Forward      forward      `json:"forward" jsonschema:"title=forward,description=Global port forwards to handle port collisions automatically between multiple okteto up sessions.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#forward-string-optional-1"`
	External     external     `json:"external" jsonschema:"title=external,description=A list of external resources that are part of your development environment. Use this section for resources that are deployed outside of the Okteto cluster, like Cloud resources or dashboards.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#external-object-optional"`
	Build        build        `json:"build" jsonschema:"title=build,description=A list of images to build as part of your development environment.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#build-object-optional"`
	Test         test         `json:"test" jsonschema:"title=test,description=A dictionary of Test Containers to run tests using Remote Execution.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#test-object-optional"`
	Destroy      destroy      `json:"destroy" jsonschema:"title=destroy,description=A list of commands to destroy external resources created by your development environment.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#destroy-string-optional"`
	Name         string       `json:"name" jsonschema:"title=name,description=The name of your development environment. It defaults to the name of your git repository.\nDocumentation: https://www.okteto.com/docs/reference/okteto-manifest/#name-string-optional"`
}

type OktetoJsonSchema struct {
	s *jsonschema.Schema
}

func NewJsonSchema() *OktetoJsonSchema {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	r.Anonymous = true
	r.AllowAdditionalProperties = false
	r.RequiredFromJSONSchemaTags = false

	s := r.Reflect(&manifest{})
	s.ID = "https://raw.githubusercontent.com/okteto/okteto/master/schema.json"
	s.Title = "Okteto Manifest Schema"
	s.Description = "A JSON schema providing inline suggestions and validation for creating Okteto Manifests in supported code editors. Okteto Manifests define Development Environments and workflows for Kubernetes applications, simplifying cloud-native development."
	s.Required = []string{}

	return &OktetoJsonSchema{
		s: s,
	}
}

// ToJSON fixes the issues with the generated schema and returns the JSON bytes
func (o *OktetoJsonSchema) ToJSON() ([]byte, error) {
	schemaBytes, err := json.MarshalIndent(o.s, "", "  ")
	if err != nil {
		return nil, err
	}

	// to obj again
	var data map[string]interface{}
	err = json.Unmarshal(schemaBytes, &data)
	if err != nil {
		return nil, err
	}

	// data to bytes
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return out, nil
}
