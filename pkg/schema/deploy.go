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

type deploy struct{}

func (deploy) JSONSchema() *jsonschema.Schema {
	namedCommandProps := jsonschema.NewProperties()
	namedCommandProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Name of the command",
	})
	namedCommandProps.Set("command", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Command to execute",
	})

	composeFileProps := jsonschema.NewProperties()
	composeFileProps.Set("file", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Path to the compose file",
	})
	composeFileProps.Set("services", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of services to deploy",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	endpointProps := jsonschema.NewProperties()
	endpointProps.Set("path", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Path for the endpoint",
	})
	endpointProps.Set("service", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Service name",
	})
	endpointProps.Set("port", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"integer"}},
		Description: "Port number",
	})

	virtualServiceProps := jsonschema.NewProperties()
	virtualServiceProps.Set("name", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Name of the virtual service",
	})
	virtualServiceProps.Set("namespace", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Namespace of the virtual service",
	})
	virtualServiceProps.Set("routes", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of routes to divert. If empty, all routes are diverted",
		Items: &jsonschema.Schema{
			Type: &jsonschema.Type{Types: []string{"string"}},
		},
	})

	hostProps := jsonschema.NewProperties()
	hostProps.Set("virtualService", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Name of the virtual service",
	})
	hostProps.Set("namespace", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Namespace of the virtual service",
	})

	divertProps := jsonschema.NewProperties()
	divertProps.Set("driver", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Enum:        []interface{}{"istio"},
		Description: "The backend for divert. Currently, only 'istio' is supported",
	})
	divertProps.Set("namespace", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Namespace of the divert",
	})
	divertProps.Set("virtualServices", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of virtual services to divert",
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           virtualServiceProps,
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})
	divertProps.Set("hosts", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of hosts to divert in the developer namespace",
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           hostProps,
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})

	deployProps := jsonschema.NewProperties()
	deployProps.Set("image", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "Image to use for deployment",
	})
	deployProps.Set("context", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"string"}},
		Description: "The working directory for running the deploy commands. If left empty, it defaults to the directory containing the Okteto Manifest.",
	})
	deployProps.Set("remote", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"boolean"}},
		Description: "Whether to run commands remotely",
	})
	deployProps.Set("commands", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of commands to execute",
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
	deployProps.Set("compose", &jsonschema.Schema{
		OneOf: []*jsonschema.Schema{
			{
				Type:        &jsonschema.Type{Types: []string{"string"}},
				Description: "Path to the compose file",
			},
			{
				Type:        &jsonschema.Type{Types: []string{"array"}},
				Description: "List of compose configurations",
				Items: &jsonschema.Schema{
					Type:                 &jsonschema.Type{Types: []string{"object"}},
					Properties:           composeFileProps,
					Required:             []string{"file"},
					AdditionalProperties: jsonschema.FalseSchema,
				},
			},
		},
	})
	deployProps.Set("endpoints", &jsonschema.Schema{
		Type:        &jsonschema.Type{Types: []string{"array"}},
		Description: "List of endpoints",
		Items: &jsonschema.Schema{
			Type:                 &jsonschema.Type{Types: []string{"object"}},
			Properties:           endpointProps,
			Required:             []string{"path", "service", "port"},
			AdditionalProperties: jsonschema.FalseSchema,
		},
	})
	deployProps.Set("divert", &jsonschema.Schema{
		Type:                 &jsonschema.Type{Types: []string{"object"}},
		Properties:           divertProps,
		AdditionalProperties: jsonschema.FalseSchema,
		Description:          "Configuration for diverting traffic between namespaces",
	})

	return &jsonschema.Schema{
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
				Properties:           deployProps,
				AdditionalProperties: jsonschema.FalseSchema,
			},
		},
	}
}
