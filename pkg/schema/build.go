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

import "github.com/invopop/jsonschema"

type build struct{}

func (build) JSONSchema() *jsonschema.Schema {
	buildProps := jsonschema.NewProperties()
	buildProps.Set("context", &jsonschema.Schema{
		Type:        "string",
		Title:       "context",
		Description: "The build context. Relative paths are relative to the location of the Okteto Manifest (default: .)",
	})
	buildProps.Set("dockerfile", &jsonschema.Schema{
		Type:        "string",
		Title:       "dockerfile",
		Description: "The path to the Dockerfile. Relative paths are relative to the location of the Okteto Manifest (default: Dockerfile)",
	})
	buildProps.Set("target", &jsonschema.Schema{
		Type:        "string",
		Title:       "target",
		Description: "The target build stage in the Dockerfile. If not specified, the default target is used.",
	})
	buildProps.Set("depends_on", &jsonschema.Schema{
		Type:        "string",
		Title:       "depends_on",
		Description: "The name of the service that the current service depends on. The service will be built after the service it depends on is built. If the service it depends on is not defined in the Okteto Manifest, the build will fail.",
	})
	buildProps.Set("secrets", &jsonschema.Schema{
		Type: "object",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: "string",
			},
		},
	})
	buildProps.Set("args", &jsonschema.Schema{
		Type: "object",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: "string",
			},
		},
	})
	return &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 "object",
				Properties:           buildProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
