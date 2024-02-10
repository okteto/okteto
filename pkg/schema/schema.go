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

import (
	"encoding/json"

	"github.com/invopop/jsonschema"
	"github.com/okteto/okteto/pkg/model"
)

type manifest struct {
	Build     build            `json:"build" jsonschema:"title=build,description=A list of images to build as part of your development environment."`
	Icon      icon             `json:"icon" jsonschema:"title=icon,description=Sets the icon that will be shown in the Okteto UI."`
	Dev       dev              `json:"dev" jsonschema:"title=dev,description=A list of development containers to define the behavior of okteto up and synchronize your code in your development environment."`
	Forward   forward          `json:"forward" jsonschema:"title=forward,description=When declaring a global forward Okteto will automatically handle port collision when two or more okteto up sessions are running simultaneously. If the okteto up session detects that the port is already in use and said port is defined as global forward okteto up will ignore the port collision and continue the up sequence. If the port is later available okteto up session will automatically connect to it without interrupting the session."`
	External  external         `json:"external" jsonschema:"title=external,description=A list of external resources that are part of your development environment. Use this section for resources that are deployed outside of the okteto cluster like Cloud resources or dashboards."`
	Destroy   interface{}      `json:"destroy" jsonschema:"title=destroy,oneof_type=object;array,description=Allows destroying resources created by your development environment. Can be either a list of commands or an object (destroy.image, destroy.commands) which in this case will execute remotely."`
	Context   string           `json:"context" jsonschema:"title=context,description=The build context. Relative paths are relative to the location of the Okteto Manifest (default: .),example=api"`
	Namespace string           `json:"namespace" jsonschema:"title=namespace,description=The namespace where the development environment is deployed. By default, it takes the current okteto context namespace. You can use an environment variable to replace the namespace field, or any part of it: namespace: $DEV_NAMESPACE"`
	Image     string           `json:"image" jsonschema:"title=image,description=The name of the image to build and push. In clusters that have Okteto installed, this is optional (if not specified, the Okteto Registry is used)."`
	Name      string           `json:"name" jsonschema:"title=name,description=The name of your development environment. It defaults to the name of your git repository."`
	Deploy    model.DeployInfo `json:"deploy" jsonschema:"title=deploy,description=The deployment configuration for your development environment. This feature is only supported in clusters that have Okteto installed. https://www.okteto.com/docs/reference/okteto-manifest/#deploy-string-optional"`
}

type OktetoJsonSchema struct {
	s *jsonschema.Schema
}

func NewJsonSchema() *OktetoJsonSchema {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	r.Anonymous = true
	r.AllowAdditionalProperties = false
	r.RequiredFromJSONSchemaTags = false

	s := r.Reflect(&manifest{})
	s.ID = "https://okteto.com/schemas/okteto-manifest.json"
	s.Title = "Okteto Manifest"
	s.Required = []string{}

	return &OktetoJsonSchema{
		s: s,
	}
}

// ToJSON fixes the issues with the generated schema and returns the JSON bytes
func (o *OktetoJsonSchema) ToJSON() ([]byte, error) {
	schemaBytes, err := json.MarshalIndent(o.s, "", "  ")
	if err != nil {
		return nil, err
	}

	// to obj again
	var data map[string]interface{}
	err = json.Unmarshal(schemaBytes, &data)
	if err != nil {
		return nil, err
	}

	// TODO: remove when MultiTypes are supported (https://github.com/invopop/jsonschema/issues/134)
	data["properties"].(map[string]interface{})["deploy"].(map[string]interface{})["type"] = []string{"array", "object"}
	data["properties"].(map[string]interface{})["dev"].(map[string]interface{})["patternProperties"].(map[string]interface{})[".*"].(map[string]interface{})["properties"].(map[string]interface{})["command"].(map[string]interface{})["type"] = []string{"array", "string"}

	// data to bytes
	out, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return nil, err
	}

	return out, nil
}

type icon struct{}

func (icon) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		AnyOf: []*jsonschema.Schema{
			{
				Type:    "string",
				Enum:    []any{"default", "container", "dashboard", "database", "function", "graph", "storage", "launchdarkly", "mongodb", "gcp", "aws", "okteto"},
				Default: "default",
			},
			{
				Type:    "string",
				Pattern: "[-a-zA-Z0-9@:%._+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_+.~#?&/=]*)",
			},
		},
	}
}

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

type dev struct{}

func (dev) JSONSchema() *jsonschema.Schema {
	devProps := jsonschema.NewProperties()
	devProps.Set("command", &jsonschema.Schema{
		Type:        "string", // changed after
		Title:       "command",
		Description: "The command of your development container. If empty, it defaults to sh. The command can also be a list",
		OneOf: []*jsonschema.Schema{
			{
				Type:    "string",
				Default: "sh",
			},
			{
				Type:  "array",
				Items: &jsonschema.Schema{Type: "string"},
			},
		},
	})
	devProps.Set("mode", &jsonschema.Schema{
		Type:        "string",
		Title:       "mode",
		Description: "Container mode (sync, hybrid)",
		Enum:        []any{"sync", "hybrid"},
		Default:     "sync",
	})
	devProps.Set("forward", &jsonschema.Schema{
		Title:       "forward",
		Type:        "array",
		Description: "",
		OneOf: []*jsonschema.Schema{
			{
				Type: "array",
				Items: &jsonschema.Schema{
					Type:    "string",
					Pattern: "^[0-9]+:[0-9]+$", // "^[0-9]+:[a-zA-Z0-9]+:[0-9]+$"
				},
			},
		},
	})
	devProps.Set("sync", &jsonschema.Schema{
		Title:       "sync",
		Type:        "array",
		Description: "",
		OneOf: []*jsonschema.Schema{
			{
				Type: "array",
				Items: &jsonschema.Schema{
					Type:    "string",
					Pattern: "^.*:.*$",
				},
			},
		},
	})
	return &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 "object",
				Properties:           devProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}

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

type external struct{}

func (external) JSONSchema() *jsonschema.Schema {
	endpointProps := jsonschema.NewProperties()
	endpointProps.Set("name", &jsonschema.Schema{
		Type: "string",
	})
	endpointProps.Set("url", &jsonschema.Schema{
		Type:   "string",
		Format: "https?:\\/\\/(www\\.)?[-a-zA-Z0-9@:%._\\+~#=]{1,256}\\.[a-zA-Z0-9()]{1,6}\\b([-a-zA-Z0-9()@:%_\\+.~#?&//=]*)",
	})

	externalProps := jsonschema.NewProperties()
	externalProps.Set("notes", &jsonschema.Schema{
		Type: "string",
	})
	externalProps.Set("icon", &jsonschema.Schema{
		Type: "string",
	})
	externalProps.Set("endpoints", &jsonschema.Schema{
		Type: "array",
		Items: &jsonschema.Schema{
			Type:       "object",
			Properties: endpointProps,
			Required:   []string{"url"},
		},
	})

	return &jsonschema.Schema{
		Type:                 "object",
		AdditionalProperties: jsonschema.FalseSchema,
		PatternProperties: map[string]*jsonschema.Schema{
			".*": {
				Type:                 "object",
				AdditionalProperties: jsonschema.FalseSchema,
				Properties:           externalProps,
			},
		},
	}
}
