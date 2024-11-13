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

type build struct{}

func (build) JSONSchema() *jsonschema.Schema {
	buildProps := jsonschema.NewProperties()
	buildProps.Set("args", &jsonschema.Schema{
		Title:       "args",
		Description: "Add build arguments, which are environment variables accessible only during the build process. Build arguments with a value containing a $ sign are resolved to the environment variable value on the machine Okteto is running on.",
		Type:        &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	buildProps.Set("context", &jsonschema.Schema{
		Title:       "context",
		Description: "The build context. Relative paths are relative to the location of the Okteto Manifest (default: .)",
		Type:        &jsonschema.Type{Types: []string{"string"}},
	})
	buildProps.Set("depends_on", &jsonschema.Schema{
		Title:       "depends_on",
		Description: "The name of the service that the current service depends on. The service will be built after the service it depends on is built. If the service it depends on is not defined in the Okteto Manifest, the build will fail.",
		Type:        &jsonschema.Type{Types: []string{"string"}},
	})
	buildProps.Set("dockerfile", &jsonschema.Schema{
		Title:       "dockerfile",
		Description: "The path to the Dockerfile. Relative paths are relative to the location of the Okteto Manifest (default: Dockerfile)",
		Type:        &jsonschema.Type{Types: []string{"string"}},
	})
	buildProps.Set("image", &jsonschema.Schema{
		Title:       "image",
		Description: "The name of the image to build and push. In clusters that have Okteto installed, this is optional (if not specified, the Okteto Registry is used).",
		Type:        &jsonschema.Type{Types: []string{"string"}},
	})
	buildProps.Set("secrets", &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	buildProps.Set("target", &jsonschema.Schema{
		Title:       "target",
		Description: "The target build stage in the Dockerfile. If not specified, the default target is used.",
		Type:        &jsonschema.Type{Types: []string{"string"}},
	})

	return &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Title:                "Name of your image",
				Description:          "The build configuration for your image",
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Properties:           buildProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
