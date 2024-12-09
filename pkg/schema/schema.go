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
	Deploy       deploy       `json:"deploy" jsonschema:"title=deploy,description=The deployment configuration for your development environment."`
	Icon         icon         `json:"icon" jsonschema:"title=icon,description=Sets the icon that will be shown in the Okteto UI."`
	Dependencies dependencies `json:"dependencies"`
	Dev          dev          `json:"dev" jsonschema:"title=dev,description=A list of development containers to define the behavior of okteto up and synchronize your code in your development environment."`
	Forward      forward      `json:"forward" jsonschema:"title=forward,description=When declaring a global forward Okteto will automatically handle port collision when two or more okteto up sessions are running simultaneously. If the okteto up session detects that the port is already in use and said port is defined as global forward okteto up will ignore the port collision and continue the up sequence. If the port is later available okteto up session will automatically connect to it without interrupting the session."`
	External     external     `json:"external" jsonschema:"title=external,description=A list of external resources that are part of your development environment. Use this section for resources that are deployed outside of the okteto cluster like Cloud resources or dashboards."`
	Build        build        `json:"build"`
	Test         test         `json:"test" jsonschema:"title=test,description=The test configuration for your development environment. This feature is only supported in clusters that have Okteto installed."`
	Destroy      destroy      `json:"destroy" jsonschema:"title=destroy,description=A list of commands to destroy external resources created by your development environment."`
	Context      string       `json:"context" jsonschema:"title=context,description=The build context. Relative paths are relative to the location of the Okteto Manifest (default: .),example=api"`
	Name         string       `json:"name" jsonschema:"title=name,description=The name of your development environment. It defaults to the name of your git repository."`
	Image        string       `json:"image" jsonschema:"title=image,description=The name of the image to build and push. In clusters that have Okteto installed, this is optional (if not specified, the Okteto Registry is used)."`
	Namespace    string       `json:"namespace" jsonschema:"title=namespace,description=The namespace where the development environment is deployed. By default, it takes the current okteto context namespace. You can use an environment variable to replace the namespace field, or any part of it: namespace: $DEV_NAMESPACE"`
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
