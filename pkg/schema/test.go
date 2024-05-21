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

type test struct{}

func (test) JSONSchema() *jsonschema.Schema {
	props := jsonschema.NewProperties()
	props.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "context",
		Description: "The build context. Relative paths are relative to the location of the Okteto Manifest (default: .)",
	})
	props.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: "The image to use for the test container",
	})

	commandProps := jsonschema.NewProperties()
	commandProps.Set("name", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})
	commandProps.Set("command", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
	})

	props.Set("commands", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array", "string"}},
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
			},
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           commandProps,
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})
	props.Set("depends_on", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	props.Set("caches", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"array"}},
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           props,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
