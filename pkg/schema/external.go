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

import "github.com/kubeark/jsonschema"

type external struct{}

func (external) JSONSchema() *jsonschema.Schema {
	endpointProps := jsonschema.NewProperties()
	endpointProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "name",
		Description: "The name of the endpoint",
		Required:    []string{"name"},
	})
	endpointProps.Set("url", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "url",
		Description: "The url of the endpoint. Can be set dynamically during deployment using $OKTETO_EXTERNAL_{EXTERNAL_NAME}_ENDPOINTS_{ENDPOINT_NAME}_URL",
		Format:      "uri",
	})

	externalProps := jsonschema.NewProperties()
	externalProps.Set("notes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "notes",
		Description: "Path to documentation or notes about the external resource",
	})
	externalProps.Set("icon", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Enum:        []any{"default", "container", "dashboard", "database", "function", "graph", "storage", "launchdarkly", "mongodb", "gcp", "aws", "okteto"},
		Title:       "icon",
		Description: withManifestRefDocLink("Icon to represent the external resource", "icon-string-optional"),
		Default:     "default",
	})
	externalProps.Set("endpoints", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "endpoints",
		Description: withManifestRefDocLink("List of endpoints to access the external resource", "endpoints-object-required"),
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           endpointProps,
			Required:             []string{"name"},
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})

	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           externalProps,
				Required:             []string{"endpoints"},
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
