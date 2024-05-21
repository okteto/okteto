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

type icon struct{}

func (icon) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"string"}},
		AnyOf: []*jsonschema.Schema{
			{
				Type:    &jsonschema.Type{Types: []string{"string"}},
				Enum:    []any{"default", "container", "dashboard", "database", "function", "graph", "storage", "launchdarkly", "mongodb", "gcp", "aws", "okteto"},
				Default: "default",
			},
			{
				Type:    &jsonschema.Type{Types: []string{"string"}},
				Pattern: "[-a-zA-Z0-9@:%._+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_+.~#?&/=]*)",
			},
		},
	}
}
