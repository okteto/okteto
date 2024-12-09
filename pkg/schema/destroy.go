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

type destroy struct{}

func (destroy) JSONSchema() *jsonschema.Schema {
	namedCommandProps := jsonschema.NewProperties()
	namedCommandProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Name of the command",
	})
	namedCommandProps.Set("command", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Command to execute",
	})

	destroyProps := jsonschema.NewProperties()
	destroyProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Image to use for running destroy commands",
	})
	destroyProps.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "The working directory for running the destroy commands. If left empty, it defaults to the directory containing the Okteto Manifest",
	})
	destroyProps.Set("remote", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Description: "Whether to run commands remotely",
	})
	destroyProps.Set("commands", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of commands to execute for destroying resources",
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				{
					Type: &jsonschema.Type{Types: []string{"string"}},
				},
				{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           namedCommandProps,
					Required:             []string{"command"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})

	return &jsonschema.Schema{
		Title:       "destroy",
		Description: "A list of commands to destroy external resources created by your development environment.\nhttps://www.okteto.com/docs/reference/okteto-manifest/#destroy-string-optional",
		OneOf: []*jsonschema.Schema{
			{
				Type: &jsonschema.Type{Types: []string{"array"}},
				Items: &jsonschema.Schema{
					OneOf: []*jsonschema.Schema{
						{
							Type: &jsonschema.Type{Types: []string{"string"}},
						},
						{
							Type:                 &jsonschema.Type{Types: []string{"object"}},
							Properties:           namedCommandProps,
							Required:             []string{"command"},
							AdditionalProperties: jsonschema.FalseSchema,
						},
					},
				},
			},
			{
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           destroyProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
