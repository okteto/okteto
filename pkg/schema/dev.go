//  Copyright 2023-2024 The Okteto Authors
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

type dev struct{}

func (dev) JSONSchema() *jsonschema.Schema {
	devProps := jsonschema.NewProperties()
	devProps.Set("command", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}}, // changed after
		Title:       "command",
		Description: "The command of your development container. If empty, it defaults to sh. The command can also be a list",
		OneOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"string"}},
				Default: "sh",
			},
			{
				Type:  &jsonschema.Type{Types: []string{"object"}},
				Items: &jsonschema.Schema{Type: &jsonschema.Type{Types: []string{"string"}}},
			},
		},
	})
	devProps.Set("mode", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "mode",
		Description: "Container mode (sync, hybrid)",
		Enum:        []any{"sync", "hybrid"},
		Default:     "sync",
	})
	devProps.Set("forward", &jsonschema.Schema{
		Title:       "forward",
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Description: "",
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"object"}},
				Items: &jsonschema.Schema{
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^[0-9]+:[0-9]+$", // "^[0-9]+:[a-zA-Z0-9]+:[0-9]+$"
				},
			},
		},
	})
	devProps.Set("sync", &jsonschema.Schema{
		Title:       "sync",
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Description: "",
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"object"}},
				Items: &jsonschema.Schema{
					Type:    &jsonschema.Type{Types: []string{"string"}},
					Pattern: "^.*:.*$",
				},
			},
		},
	})
	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           devProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
