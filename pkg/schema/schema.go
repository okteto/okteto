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
	oktetoLog "github.com/okteto/okteto/pkg/log"
	"github.com/okteto/okteto/pkg/model"
	"os"
)

type manifest struct {
	Build     build  `json:"build" jsonschema:"title=build,description=A list of images to build as part of your development environment."`
	Context   string `json:"context" jsonschema:"title=context,description=The build context. Relative paths are relative to the location of the Okteto Manifest (default: .),example=api"`
	Namespace string `json:"namespace" jsonschema:"title=namespace,description=The namespace where the development environment is deployed. By default, it takes the current okteto context namespace. You can use an environment variable to replace the namespace field, or any part of it: namespace: $DEV_NAMESPACE"`
	Image     string `json:"image" jsonschema:"title=image,description=The name of the image to build and push. In clusters that have Okteto installed, this is optional (if not specified, the Okteto Registry is used)."`
	Icon      icon   `json:"icon" jsonschema:"title=icon,description=Sets the icon that will be shown in the Okteto UI. The supported values for icons are listed below."`
	// TODO: Dev breaks due to recursion of Dev.Services being an array of []*Dev
	Dev    dev              `json:"dev" jsonschema:"title=dev,description=A list of development containers to define the behavior of okteto up and synchronize your code in your development environment."`
	Deploy model.DeployInfo `json:"deploy" jsonschema:"title=deploy,description=The deployment configuration for your development environment. This feature is only supported in clusters that have Okteto installed. https://www.okteto.com/docs/reference/okteto-manifest/#deploy-string-optional"`
	// TODO: the library doesn't allow oneof_ref and say what type they are! See: https://github.com/invopop/jsonschema/issues/68
	Destroy interface{} `json:"destroy" jsonschema:"title=destroy,oneof_type=object;array,description=Allows destroying resources created by your development environment. Can be either a list of commands or an object (destroy.image, destroy.commands) which in this case will execute remotely."`
	//Dependencies map[string]deps.Dependency `json:"dependencies" jsonschema:"title=dependencies,description=Repositories you want to deploy as part of your development environment. This feature is only supported in clusters that have Okteto installed."`
	// TODO: make sure all are covered: https://www.okteto.com/docs/reference/manifest/#example
}

func GenerateJsonSchema() *jsonschema.Schema {
	r := new(jsonschema.Reflector)
	r.DoNotReference = true
	r.Anonymous = true
	r.AllowAdditionalProperties = false
	r.RequiredFromJSONSchemaTags = false

	schema := r.Reflect(&manifest{})
	schema.ID = "https://okteto.com/schemas/okteto-manifest.json"
	schema.Title = "Okteto Manifest"
	schema.Required = []string{}

	return schema
}

func FixAndMarshal(schema *jsonschema.Schema) ([]byte, error) {
	schemaBytes, err := json.MarshalIndent(schema, "", "  ")
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

func SaveSchema(schemaBytes []byte, outputFilePath string) error {
	err := os.WriteFile(outputFilePath, schemaBytes, 0644)
	if err != nil {
		return err
	}
	oktetoLog.Success("okteto json schema has been generated and stored in %s", schemaBytes)

	return nil
}

type icon struct{}

func (icon) JSONSchema() *jsonschema.Schema {
	return &jsonschema.Schema{
		Type: "string",
		OneOf: []*jsonschema.Schema{
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
