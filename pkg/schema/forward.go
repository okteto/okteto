//  Copyright 2024 The Okteto Authors
// Copyright 2023|2024 The Okteto Authors
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

import "github.com/invopop/jsonschema"

type forward struct{}

func (forward) JSONSchema() *jsonschema.Schema {
	shorthandPattern := &jsonschema.Schema{
		Type:    "string",
		Pattern: "^[0-9]+:([a-zA-Z0-9]+:)?[0-9]+$",
	}

	// Define the properties for the detailed object notation
	objectProps := jsonschema.NewProperties()
	objectProps.Set("localPort", &jsonschema.Schema{
		Type: "integer",
	})
	objectProps.Set("remotePort", &jsonschema.Schema{
		Type: "integer",
	})
	objectProps.Set("name", &jsonschema.Schema{
		Type: "string",
	})
	objectProps.Set("labels", &jsonschema.Schema{
		Type: "object",
		AdditionalProperties: &jsonschema.Schema{
			Type: "string",
		},
	})

	detailedObjectWithOptionalName := &jsonschema.Schema{
		Type:                 "object",
		Properties:           objectProps,
		Required:             []string{"localPort", "remotePort"},
		AdditionalProperties: jsonschema.FalseSchema,
	}

	itemsSchema := &jsonschema.Schema{
		AnyOf: []*jsonschema.Schema{
			shorthandPattern,
			detailedObjectWithOptionalName,
		},
	}

	return &jsonschema.Schema{
		Type:  "array",
		Items: itemsSchema,
	}
}
