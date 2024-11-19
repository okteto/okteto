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

type forward struct{}

func (forward) JSONSchema() *jsonschema.Schema {
	shorthandPattern := &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "forward",
		Description: "Port forward in the format localPort:service:remotePort or localPort:remotePort",
		Pattern:     "^\\d+:[^:]+:\\d+$|^\\d+:\\d+$", // example: 5432:postgres:5432
	}

	objectProps := jsonschema.NewProperties()
	objectProps.Set("localPort", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"integer"}},
		Title:       "localPort",
		Description: "Local port to forward from",
	})
	objectProps.Set("remotePort", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"integer"}},
		Title:       "remotePort",
		Description: "Remote port to forward to",
	})
	objectProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "name",
		Description: "Name of the service to forward to",
	})
	objectProps.Set("labels", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "labels",
		Description: "Labels to select the service to forward to",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})

	detailedObjectSchema := &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Description: "Detailed port forward configuration",
		Properties:  objectProps,
		Required:    []string{"localPort", "remotePort"},
		// name and labels are mutually exclusive
		OneOf: []*jsonschema.Schema{
			{
				Required: []string{"name"},
				Not: &jsonschema.Schema{
					Required: []string{"labels"},
				},
			},
			{
				Required: []string{"labels"},
				Not: &jsonschema.Schema{
					Required: []string{"name"},
				},
			},
			{
				Not: &jsonschema.Schema{
					AnyOf: []*jsonschema.Schema{
						{Required: []string{"name"}},
						{Required: []string{"labels"}},
					},
				},
			},
		},
		AdditionalProperties: jsonschema.FalseSchema,
	}

	return &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "forward",
		Description: "Global port forwards that handle port collisions automatically between multiple okteto up sessions",
		Items: &jsonschema.Schema{
			OneOf: []*jsonschema.Schema{
				shorthandPattern,
				detailedObjectSchema,
			},
		},
	}
}
