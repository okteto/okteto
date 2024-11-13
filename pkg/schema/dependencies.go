//  Copyright 2024 The Okteto Authors
//  Licensed under the Apache License, Version 2.0 (the "License");
//  you may not use this file except in compliance with the License.
//  You may obtain a copy of the License at
//
//  http://www.apache.org/licenses/LICENSE-2.0
//
//  Unless required by applicable law or agreed to in writing, software
//  distributed under the License is distributed on an "AS IS" BASIS,
//  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//  See the License for the specific language governing permissions and
//  limitations under the License.

package schema

import "github.com/kubeark/jsonschema"

type dependencies struct{}

func (dependencies) JSONSchema() *jsonschema.Schema {
	extendedProps := jsonschema.NewProperties()
	extendedProps.Set("repository", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "repository",
		Description: "The Git repository URL of the dependency",
	})
	extendedProps.Set("manifest", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "manifest",
		Description: "The path to the Okteto manifest file in the repository",
	})
	extendedProps.Set("branch", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "branch",
		Description: "The Git branch to use",
	})
	extendedProps.Set("variables", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "variables",
		Description: "Environment variables to pass to the dependency",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	extendedProps.Set("wait", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Title:       "wait",
		Description: "Whether to wait for the dependency to be ready",
	})
	extendedProps.Set("timeout", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "timeout",
		Description: "Maximum time to wait for the dependency to be ready",
		Pattern:     "^[0-9]+(h|m|s)$",
	})

	return &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
			},
			{
				Type: &jsonschema.Type{Types: []string{"object"}},
				PatternProperties: map[string]*jsonschema.Schema{
					".*": {
						Type:                 &jsonschema.Type{Types: []string{"object"}},
						Properties:           extendedProps,
						Required:             []string{"repository"},
						AdditionalProperties: jsonschema.FalseSchema,
					},
				},
			},
		},
	}
}
