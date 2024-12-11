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

type build struct{}

func (build) JSONSchema() *jsonschema.Schema {
	buildProps := jsonschema.NewProperties()
	buildProps.Set("args", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "args",
		Description: "Build arguments, which are environment variables accessible only during the build process. Build arguments with a value containing a $ sign are resolved to the environment variable value on the machine Okteto is running on",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	buildProps.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "context",
		Description: "The build context. Relative paths are relative to the location of the Okteto Manifest",
		Default:     ".",
	})
	buildProps.Set("depends_on", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Title:       "depends_on",
		Description: "List of images that need to be built first",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})
	buildProps.Set("dockerfile", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "dockerfile",
		Description: "The path to the Dockerfile. It's a relative path to the build context",
		Default:     "Dockerfile",
	})
	buildProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "image",
		Description: "The name of the image to build and push. In clusters that have Okteto installed, this is optional (if not specified, the Okteto Registry is used)",
	})
	buildProps.Set("secrets", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"object"}},
		Title:       "secrets",
		Description: "List of secrets exposed to the build. The value of each secret refers to a file. Okteto will resolve references containing a $ sign in this file to environment variables on the machine Okteto is running on",
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type: &jsonschema.Type{Types: []string{"string"}},
			},
		},
	})
	buildProps.Set("target", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Title:       "target",
		Description: "Build the specified stage as defined inside the Dockerfile",
	})

	return &jsonschema.Schema{
		Type: &jsonschema.Type{Types: []string{"object"}},
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 &jsonschema.Type{Types: []string{"object"}},
				Description:          withManifestRefDocLink("A list of images to build as part of your development environment.\n\n", "build-object-optional"),
				Properties:           buildProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
		AdditionalProperties: jsonschema.FalseSchema,
	}
}
